part of 'study_canvas.dart';

extension _StudySnapshotCapture on StudyCanvasState {
  Future<StudySnapshot> _prepareSelectionSnapshot() async {
    final d = document!, region = _regionSelection;
    if (region == null) throw FormatException(l10n.studySelectionEmpty);
    final polygon = List<Offset>.of(_selectionPolygon);
    final zoom = d.zoom, camera = d.camera, edit = d.edit;
    final query = _selectedQuery();
    final cards = d.cards
        .where(
          (c) =>
              c.type != 'conversation' &&
              d.visible(c.layer) &&
              c.rect.overlaps(region),
        )
        .toList();
    query['objects'] = cards.map((c) => c.id).toList();
    query['object_pages'] = {
      for (final c in cards)
        if (c.type == 'pdfdoc') c.id: c.page,
    };
    query['include_video'] = cards.any((c) => c.type == 'lecture');
    query['object_video_ms'] = {
      for (final c in cards)
        if (c.type == 'video') c.id: _localVideoTimes[c.id] ?? 0,
    };
    void check() {
      if (!mounted ||
          document != d ||
          d.edit != edit ||
          d.zoom != zoom ||
          d.camera != camera ||
          _regionSelection != region) {
        throw FormatException(l10n.studySelectionChanged);
      }
    }

    final sources = await _selectionSources(query);
    check();
    try {
      // RepaintBoundary cannot reliably include native video textures. Replace
      // just the displayed video area during capture, below the ink layer.
      for (final c in cards.where(
        (c) => !c.collapsed && (c.type == 'video' || c.type == 'lecture'),
      )) {
        final ms = c.type == 'lecture'
            ? query['video_ms'] as int
            : (query['object_video_ms'] as Map)[c.id] as int;
        final task = await widget.api.prepareStudyMedia(widget.ctl.lectureId, {
          'object_id': c.id,
          'from_ms': ms,
          'to_ms': ms + 1,
          'frame': true,
          'transcribe': false,
        });
        final deadline = DateTime.now().add(const Duration(seconds: 150));
        try {
          while (true) {
            check();
            final current = await widget.api.studyMediaTask(
              widget.ctl.lectureId,
              '${task['id']}',
            );
            if (current['state'] == 'done') {
              final file = '${current['result']?['image_path'] ?? ''}';
              if (file.isEmpty) {
                throw FormatException(l10n.studySnapshotVideoFailed);
              }
              final bytes = await widget.api.studyImageBytes(file);
              check();
              final provider = MemoryImage(bytes);
              await precacheImage(provider, context);
              _snapshotFrames[c.id] = bytes;
              break;
            }
            if (current['state'] != 'running' ||
                DateTime.now().isAfter(deadline)) {
              throw FormatException(l10n.studySnapshotVideoFailed);
            }
            await Future<void>.delayed(const Duration(milliseconds: 250));
          }
        } finally {
          await widget.api.cancelStudyMedia(
            widget.ctl.lectureId,
            '${task['id']}',
          );
        }
      }
      check();
      _update(() => _snapshotting = true);
      final bytes = await _regionImage(region, polygon: polygon);
      check();
      if (bytes.length > 3 * 1024 * 1024) {
        throw FormatException(l10n.studySnapshotTooLarge);
      }
      return StudySnapshot(bytes, {
        'bounds': [region.left, region.top, region.width, region.height],
        'polygon': polygon.map((p) => [p.dx, p.dy]).toList(),
        'sources': sources.map((a) => a.json()).toList(),
      });
    } finally {
      _snapshotFrames.clear();
      if (mounted) _update(() => _snapshotting = false);
    }
  }

  Widget _snapshotVideo(StudyCard card, Widget child) => Stack(
    fit: StackFit.expand,
    children: [
      child,
      if (_snapshotting && _snapshotFrames[card.id] != null)
        Positioned.fill(
          child: ColoredBox(
            color: Colors.black,
            child: Image.memory(_snapshotFrames[card.id]!, fit: BoxFit.contain),
          ),
        ),
    ],
  );

  Widget _snapshotDragHandle() {
    final drop = StudySelectionDrop();
    return Draggable<StudySelectionDrop>(
      data: drop,
      maxSimultaneousDrags: _selectionBusy || _snapshotPreparing ? 0 : 1,
      onDragStarted: () {
        _update(() => _snapshotPreparing = true);
        drop.snapshot = _prepareSelectionSnapshot();
        // Catch immediately even when the user drops outside the composer.
        drop.snapshot!.then(
          (_) {
            if (mounted) {
              _update(() {
                _stageOpen = true;
                _snapshotPreparing = false;
              });
            }
          },
          onError: (Object e) {
            if (mounted) {
              _update(() => _snapshotPreparing = false);
              _toast('$e');
            }
          },
        );
      },
      feedback: GlassSurface(
        radius: Corners.full,
        depth: GlassDepth.float,
        padding: const EdgeInsets.symmetric(
          horizontal: Space.md,
          vertical: Space.sm,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.add_photo_alternate_outlined, size: 18),
            const SizedBox(width: Space.sm),
            Text(l10n.studySnapshotDrag),
          ],
        ),
      ),
      // A tap attaches it too, for those who do not drag.
      child: IconButton(
        key: const ValueKey('study-snapshot-attach'),
        tooltip: l10n.studySnapshotDrag,
        onPressed: _selectionBusy || _snapshotPreparing
            ? null
            : () => _selectionJob(() async {
                _update(() => _snapshotPreparing = true);
                try {
                  await _acceptSnapshot(
                    StudySelectionDrop()
                      ..snapshot = _prepareSelectionSnapshot(),
                  );
                  if (mounted) _update(() => _stageOpen = true);
                } finally {
                  if (mounted) _update(() => _snapshotPreparing = false);
                }
              }),
        icon: _snapshotPreparing
            ? const ReadingOrb()
            : const Icon(Icons.add_photo_alternate_outlined, size: 20),
      ),
    );
  }

  Future<void> _acceptSnapshot(StudySelectionDrop drop) async {
    try {
      final pending = drop.snapshot;
      if (pending == null) return;
      final snapshot = await pending;
      if (!mounted) return;
      var state = _conversationKeys[_activeConversation]?.currentState;
      if (state == null) {
        await _startConversation(bind: false, snapshot: snapshot);
      } else {
        await state.attachSnapshot(snapshot);
      }
    } catch (e) {
      if (mounted) _toast('$e');
    }
  }
}
