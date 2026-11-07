part of 'study_canvas.dart';

extension _StudyInteraction on StudyCanvasState {
  void _updateEraserCursor(Offset local) {
    _eraserCursor = local;
    _cursorRadius = _eraserSize;
    _cursor.value++;
  }

  void _eraseAt(Offset a, Offset b) {
    final d = document!;
    if (_eraserMode == 'stroke') {
      d.eraseWhole(a, b, _eraserSize / d.zoom);
    } else {
      d.erase(a, b, _eraserSize / d.zoom);
    }
  }

  void _beginErase(PointerEvent e, Offset p) {
    document!.checkpoint();
    _startErase(e.localPosition, p);
  }

  void _startErase(Offset local, Offset p) {
    _erasing = true;
    _selectionReady = false;
    _regionSelection = null;
    _detail.clear();
    _selection.clear();
    _last = p;
    _eraserLasso.clear();
    if (_eraserMode == 'lasso') {
      _eraserLasso.add(p);
    } else {
      _eraseAt(p, p);
    }
    _updateEraserCursor(local);
  }

  // Erase is a press-and-hold at touchdown. Once the pen has travelled,
  // endpoint holds belong to shape recognition, even with heavy pressure.
  void _watchPressure(PointerEvent e) {
    if (_pressureStart != null &&
        (e.localPosition - _pressureStart!).distance > 6) {
      _pressureStart = null;
    }
    if (_pressureStart == null ||
        !_pressureEraser ||
        _shape != null ||
        e.kind != PointerDeviceKind.stylus ||
        e.pressureMax <= e.pressureMin ||
        _pressure(e) < _pressureThreshold) {
      _pressureTimer?.cancel();
      _pressureTimer = null;
      _pressureOrigin = null;
      return;
    }
    if (_pressureOrigin != null &&
        (e.localPosition - _pressureOrigin!).distance <= 6)
      return;
    _pressureTimer?.cancel();
    _pressureOrigin = e.localPosition;
    _pressureKeep = math.max(0, _points.length - 1);
    final pointer = e.pointer;
    _pressureTimer = Timer(const Duration(milliseconds: 380), () {
      _pressureTimer = null;
      if (!mounted || _drawingPointer != pointer || _pressureOrigin == null)
        return;
      final local = _pointers[pointer];
      if (local == null) return;
      final d = document!;
      _cancelCardHold();
      _resetShape();
      d.checkpoint();
      // Keep writing before the hold; remove only the stationary trigger tail.
      if (_pressureKeep > 1) {
        d.strokes.addAll(
          StudyDocument.chunks(
            StudyStroke(
              id: studyId(),
              layer: d.activeLayer,
              points: _points.take(_pressureKeep).toList(),
              color: _gestureColor,
              width: _gestureWidth,
              brush: _gestureBrush,
              tool: _tool == StudyTool.highlighter ? 'highlighter' : 'pen',
              anchor: _anchor,
              page:
                  d.cards
                      .where((c) => c.id == _anchor && c.type == 'pdfdoc')
                      .firstOrNull
                      ?.page ??
                  0,
            ),
          ),
        );
        d.changed();
      }
      _points = [];
      _prefix.clear();
      _prefixEnd = 0;
      _drawingPointer = null;
      _temporaryEraser = true;
      _startErase(local, _world(local));
      _scene.value++;
      _live.value++;
      unawaited(HapticFeedback.selectionClick());
    });
  }

  void _finishSelection() {
    final d = document!, r = _marquee!;
    if (r.width * d.zoom < 5 || r.height * d.zoom < 5) return;
    final polygon = _lassoRectangle
        ? [r.topLeft, r.topRight, r.bottomRight, r.bottomLeft]
        : List<Offset>.of(_lasso);
    _detail.clear();
    if (_selectionKinds.contains(StudySelectionKind.ink))
      _detail.strokeIds.addAll(d.strokesInPolygon(polygon));
    _selection.clear();
    // A loop on a document selects ink/visual context. It only moves the entire
    // document if the loop encloses the card, avoiding accidental PDF movement.
    final loop = Path()..addPolygon(polygon, true);
    for (final c in d.cards.where(
      (c) =>
          c.type != 'conversation' &&
          d.visible(c.layer) &&
          _selectionKinds.contains(studyCardSelectionKind(c)),
    )) {
      if (!r.contains(c.rect.topLeft) || !r.contains(c.rect.bottomRight))
        continue;
      // A free lasso's bounding rectangle is not its enclosed area.
      final outside = Path.combine(
        PathOperation.difference,
        Path()..addRect(c.rect),
        loop,
      );
      if (outside.getBounds().isEmpty) _selection.add(c.id);
    }
    _regionSelection = r;
    _selectionBounds = _boundsOfSelection() ?? r;
    final anchored = d.cards
        .where(
          (c) =>
              c.type != 'conversation' &&
              _selectionKinds.contains(studyCardSelectionKind(c)) &&
              d.visible(c.layer) &&
              _sourceBounds(c).contains(r.center) &&
              _sourceBounds(c).overlaps(r),
        )
        .lastOrNull;
    if (anchored != null && _selection.isEmpty) {
      _detail.objectId = anchored.id;
      _detail.region = _normalizeRegion(
        r.intersect(_sourceBounds(anchored)),
        _sourceBounds(anchored),
      );
    } else {
      _detail.region = r;
    }
    _quickAsk = false;
    _quickError = null;
    _quickImages = false;
    _quickVisuals = [];
    _selectionReady = true;
    _cursor.value++;
    if (_captureLasso) unawaited(_captureRegionCard());
  }

  Rect? _boundsOfSelection() {
    final d = document!;
    final bounds = [
      for (final s in d.strokes)
        if (_detail.strokeIds.contains(s.id)) s.bounds,
      for (final c in d.cards)
        if (_selection.contains(c.id)) c.rect,
    ];
    return bounds.isEmpty
        ? null
        : bounds.reduce((a, b) => a.expandToInclude(b));
  }

  void _transformSelection(Rect from, Rect to, {int? color, double? width}) {
    final d = document!;
    // Anchored ink is transformed by its selected parent exactly once.
    final ink = d.strokes
        .where(
          (s) =>
              _detail.strokeIds.contains(s.id) &&
              !_selection.contains(s.anchor),
        )
        .map((s) => s.id)
        .toSet();
    if (_selection.isNotEmpty && color == null && width == null)
      d.transformCards(_selection, from, to);
    d.transformInk(
      color != null || width != null ? _detail.strokeIds : ink,
      from,
      to,
      color: color,
      width: width,
    );
    _selectionBounds = _boundsOfSelection() ?? to;
    _regionSelection = _selectionBounds;
    _detail.region = _selectionBounds;
    _detail.objectId = '';
    _cursor.value++;
    _update(() {});
  }

  void _copySelection() {
    final d = document!;
    _copiedCards = d.cards
        .where((c) => _selection.contains(c.id) && c.type != 'lecture')
        .map((c) => c.copy())
        .toList();
    final parents = _copiedCards.map((c) => c.id).toSet();
    _copiedInk = d.strokes
        .where(
          (s) => _detail.strokeIds.contains(s.id) || parents.contains(s.anchor),
        )
        .toList();
    _copiedBounds = _selectionBounds;
    _update(() {});
  }

  void _pasteSelection() {
    final d = document!;
    if (_copiedBounds == null || (_copiedInk.isEmpty && _copiedCards.isEmpty))
      return;
    if (d.cards.length + _copiedCards.length > 2000 ||
        d.strokes.length + _copiedInk.length > 29000 ||
        d.strokes.fold<int>(0, (n, s) => n + s.points.length) +
                _copiedInk.fold<int>(0, (n, s) => n + s.points.length) >
            490000) {
      _toast('画布内容已接近容量上限，请先整理后再粘贴');
      return;
    }
    if (!d.editable(d.activeLayer)) {
      _toast(l10n.studyLayerUnavailable);
      return;
    }
    d.checkpoint();
    final oldBounds = _copiedBounds!;
    final delta = _visibleWorld.contains(oldBounds.center)
        ? Offset(28 / d.zoom, 28 / d.zoom)
        : _visibleWorld.center - oldBounds.center;
    final cardIds = {for (final c in _copiedCards) c.id: studyId()};
    final gestureIds = {for (final s in _copiedInk) s.gesture: studyId()};
    _detail.clear();
    _selection.clear();
    for (final c in _copiedCards) {
      final copy = StudyCard.fromJson({
        ...c.json(),
        'id': cardIds[c.id],
        'pdf_column': cardIds[c.pdfColumn] ?? '',
        'layer': d.activeLayer,
      });
      copy.rect = copy.rect.shift(delta);
      d.cards.add(copy);
      _selection.add(copy.id);
    }
    for (final s in _copiedInk) {
      final copy = StudyStroke(
        id: studyId(),
        layer: d.activeLayer,
        gesture: gestureIds[s.gesture],
        anchor: cardIds[s.anchor] ?? s.anchor,
        page: s.page,
        points: [for (final p in s.points) InkPoint(p.p + delta, p.pressure)],
        color: s.color,
        width: s.width,
        tool: s.tool,
        brush: s.brush,
      );
      d.strokes.add(copy);
      _detail.strokeIds.add(copy.id);
    }
    _tool = StudyTool.region;
    _selectionBounds = _boundsOfSelection();
    _regionSelection = _selectionBounds;
    _detail.region = _selectionBounds;
    _quickAsk = false;
    _selectionReady = true;
    d.changed();
  }

  void _deleteSelection() {
    final d = document!;
    final cards = d.cards
        .where(
          (c) =>
              _selection.contains(c.id) &&
              c.type != 'lecture' &&
              d.canTransformCard(c),
        )
        .map((c) => c.id)
        .toSet();
    final ink = d.strokes
        .where(
          (s) =>
              d.editable(s.layer) &&
              (_detail.strokeIds.contains(s.id) || cards.contains(s.anchor)),
        )
        .map((s) => s.id)
        .toSet();
    if (cards.isEmpty && ink.isEmpty) return;
    d.checkpoint();
    d.cards.removeWhere((c) => cards.contains(c.id));
    d.strokes.removeWhere((s) => ink.contains(s.id));
    d.connections.removeWhere(
      (c) => cards.contains(c.from) || cards.contains(c.to),
    );
    _selection.clear();
    _detail.clear();
    _selectionReady = false;
    _selectionBounds = null;
    _regionSelection = null;
    d.changed();
  }

  void _clearSelection() => _update(() {
    _selectionReady = false;
    _selectionBounds = null;
    _regionSelection = null;
    _quickAsk = false;
    _selection.clear();
    _detail.clear();
    _cursor.value++;
  });

  /// What the selection binds, one entry per source, as the composer names it.
  List<_Bound> _boundSources() {
    final d = document!;
    return [
      for (final c in d.cards)
        if (c.type != 'conversation' &&
            d.visible(c.layer) &&
            (_selection.contains(c.id) || _detail.objectId == c.id))
          _Bound(
            id: c.id,
            label: c.title.isNotEmpty
                ? c.title
                : c.type == 'lecture'
                ? l10n.studyBoundLecture
                : l10n.studyBoundCard,
            detail: [
              if (c.type == 'pdfdoc' && c.page > 0)
                l10n.studySourcePage(c.page),
              if (_detail.objectId == c.id && _detail.region != null)
                l10n.studyBoundRegion,
            ].join(' · '),
            icon: switch (c.type) {
              'lecture' || 'video' => Icons.smart_display_outlined,
              'image' => Icons.image_outlined,
              'pdf' || 'pdfdoc' => Icons.description_outlined,
              _ => Icons.notes_rounded,
            },
            readable: _readable(
              c,
              _detail.objectId == c.id ? _detail.region : null,
            ),
          ),
      if (_detail.strokeIds.isNotEmpty)
        _Bound(
          id: 'selected-ink',
          label: l10n.studyBoundInk(_detail.strokeIds.length),
          icon: Icons.gesture_rounded,
          readable: false,
        ),
    ];
  }

  /// Whether the daemon will find words to read: a page region reads the
  /// words at least half inside it, as `studyRegionSpan` does. Null when
  /// only a run can tell (a transcript).
  bool? _readable(StudyCard c, Rect? region) {
    if (c.type == 'lecture' || c.type == 'video') return null;
    if (region == null) {
      return const {'image', 'pdf'}.contains(c.type)
          ? false
          : c.contentText.trim().isNotEmpty;
    }
    if (c.type != 'pdfdoc' || c.quarterTurns % 4 != 0) return false;
    double n(Object? v) => v is num && v.isFinite ? v.toDouble() : 0;
    return c.textBoxes.any((b) {
      final box = Rect.fromLTWH(n(b['x']), n(b['y']), n(b['w']), n(b['h']));
      if (box.isEmpty || n(b['end']) <= n(b['start'])) return false;
      final hit = box.intersect(region);
      return hit.width > 0 &&
          hit.height > 0 &&
          hit.width * hit.height >= box.width * box.height / 2;
    });
  }

  void _unbind(String id) => _update(() {
    if (id == 'selected-ink') {
      _detail.strokeIds.clear();
    } else if (_detail.objectId == id) {
      _detail
        ..objectId = null
        ..start = 0
        ..end = 0
        ..region = _regionSelection;
    }
    _selection.remove(id);
    // A crop belongs to the binding it was taken from.
    _quickVisuals = [];
    if (_quickImages) unawaited(_previewQuickVision());
  });

  void _bindCard(String id) => _update(() {
    _selection.add(id);
    _quickVisuals = [];
    if (_quickImages) unawaited(_previewQuickVision());
  });

  Future<void> _setQuickVision(bool on) async {
    _update(() {
      _quickImages = on;
      _quickVisuals = [];
      _quickError = null;
    });
    if (on) await _previewQuickVision();
  }

  /// Takes the crops 「看图」 will send, so the learner sees them first.
  Future<void> _previewQuickVision() async {
    if (_quickCapturing) return;
    _update(() => _quickCapturing = true);
    try {
      final images = await _captureSelection();
      if (mounted && _quickImages) _update(() => _quickVisuals = images);
    } catch (e) {
      if (mounted && _quickImages) {
        _update(() => _quickError = l10n.studyQuickSendFailed('$e'));
      }
    } finally {
      if (mounted) _update(() => _quickCapturing = false);
    }
  }

  Widget _selectionToolbar() {
    if (_tool != StudyTool.region ||
        !_selectionReady ||
        _selectionBounds == null ||
        _selectionDragging)
      return const SizedBox.shrink();
    return AnimatedBuilder(
      animation: _camera,
      builder: (context, _) {
        final d = document!, r = _selectionBounds!;
        final anchor = Rect.fromPoints(
          r.topLeft * d.zoom + d.camera,
          r.bottomRight * d.zoom + d.camera,
        );
        return Positioned.fill(
          child: CustomSingleChildLayout(
            delegate: _FloatingLayout(anchor, _topBarsBottom),
            child: _Appear(
              key: _selectionMenuKey,
              child: _quickAsk
                  ? _quickComposer(context)
                  : _selectionBar(context),
            ),
          ),
        );
      },
    );
  }

  /// Three verbs for a selection, with editing in a menu.
  Widget _selectionBar(BuildContext context) {
    final d = document!, r = _selectionBounds!;
    final ink = _detail.strokeIds.isNotEmpty;
    return GlassSurface(
      key: const ValueKey('study-selection-bar'),
      radius: Corners.full,
      depth: GlassDepth.float,
      padding: const EdgeInsets.all(Space.xxs),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          _BarAction(
            label: l10n.studyAskAI,
            icon: Icons.auto_awesome_outlined,
            tooltip: l10n.studyAskHint,
            primary: true,
            onPressed: () => unawaited(_ask()),
          ),
          _BarAction(
            label: l10n.studyExcerpt,
            icon: Icons.crop_rounded,
            tooltip: l10n.studyExcerptHint,
            onPressed: _selectionBusy
                ? null
                : () => unawaited(_captureRegionCard()),
          ),
          _BarAction(
            label: _linkStart == null ? l10n.studyRelate : l10n.studyRelateHere,
            icon: _linkStart == null
                ? Icons.link_rounded
                : Icons.add_link_rounded,
            tooltip: _linkStart == null
                ? l10n.studyRelateHint
                : l10n.studyRelateHereHint,
            onPressed: _selectionBusy
                ? null
                : () => _selectionJob(_linkSelection),
          ),
          if (ink) ...[
            PopupMenuButton<int>(
              tooltip: '选区改色',
              icon: const Icon(Icons.palette_outlined, size: 20),
              itemBuilder: (_) => [
                for (final c in _penPalette.take(20))
                  PopupMenuItem(
                    value: c,
                    child: Row(
                      children: [
                        Icon(Icons.circle, color: Color(c)),
                        const SizedBox(width: 12),
                        Text(
                          '#${(c & 0xffffff).toRadixString(16).padLeft(6, '0')}',
                        ),
                      ],
                    ),
                  ),
              ],
              onSelected: (c) {
                d.checkpoint();
                _transformSelection(r, r, color: c);
              },
            ),
            PopupMenuButton<double>(
              tooltip: '选区粗细',
              icon: const Icon(Icons.line_weight, size: 20),
              itemBuilder: (_) => [
                for (final w in [1.0, 2.0, 3.0, 5.0, 8.0, 12.0])
                  PopupMenuItem(value: w, child: Text('${w.toInt()}')),
              ],
              onSelected: (w) {
                d.checkpoint();
                _transformSelection(r, r, width: w);
              },
            ),
          ],
          _selectionActions(),
        ],
      ),
    );
  }

  /// 「问 AI」 in place: what is bound, the question, and whether pixels go too.
  Widget _quickComposer(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final bound = _boundSources();
    final unreadable =
        bound.isNotEmpty && bound.every((b) => b.readable == false);
    final d = document!;
    final more =
        d.cards
            .where(
              (c) =>
                  c.type != 'conversation' &&
                  d.visible(c.layer) &&
                  !_selection.contains(c.id) &&
                  _detail.objectId != c.id,
            )
            .toList()
          // Nearby material first.
          ..sort(
            (a, b) => (a.rect.center - _selectionBounds!.center).distance
                .compareTo((b.rect.center - _selectionBounds!.center).distance),
          );
    return SizedBox(
      key: const ValueKey('study-quick-composer'),
      width: math.min(380.0, _viewport.width - 24),
      child: GlassSurface(
        radius: Corners.lg,
        depth: GlassDepth.float,
        padding: const EdgeInsets.fromLTRB(
          Space.md,
          Space.sm,
          Space.sm,
          Space.sm,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Wrap(
              spacing: Space.xs,
              runSpacing: Space.xs,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                for (final b in bound)
                  _BoundChip(
                    bound: b,
                    onRemove: _quickSending ? null : () => _unbind(b.id),
                  ),
                if (more.isNotEmpty)
                  MenuAnchor(
                    menuChildren: [
                      for (final c in more.take(12))
                        MenuItemButton(
                          onPressed: () => _bindCard(c.id),
                          child: ConstrainedBox(
                            constraints: const BoxConstraints(maxWidth: 240),
                            child: Text(
                              c.title.isNotEmpty
                                  ? c.title
                                  : l10n.studyBoundCard,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ),
                    ],
                    builder: (context, menu, _) => IconButton(
                      tooltip: l10n.studyBindMore,
                      visualDensity: VisualDensity.compact,
                      onPressed: _quickSending
                          ? null
                          : () => menu.isOpen ? menu.close() : menu.open(),
                      icon: const Icon(Icons.add_rounded, size: 18),
                    ),
                  ),
              ],
            ),
            Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Expanded(
                  child: Focus(
                    canRequestFocus: false,
                    onKeyEvent: (_, e) {
                      if (e is KeyDownEvent &&
                          e.logicalKey == LogicalKeyboardKey.escape) {
                        // The draft stays for the next 「问 AI」.
                        _update(() => _quickAsk = false);
                        return KeyEventResult.handled;
                      }
                      return KeyEventResult.ignored;
                    },
                    child: TextField(
                      key: const ValueKey('study-quick-question'),
                      controller: _quickInput,
                      autofocus: true,
                      maxLength: 4000,
                      style: tt.bodyLarge,
                      decoration: InputDecoration(
                        hintText: l10n.studyQuickHint,
                        counterText: '',
                        border: InputBorder.none,
                        enabledBorder: InputBorder.none,
                        focusedBorder: InputBorder.none,
                        filled: false,
                        isDense: true,
                        contentPadding: const EdgeInsets.symmetric(
                          vertical: Space.md,
                        ),
                      ),
                      onSubmitted: (_) => _sendQuickQuestion(),
                    ),
                  ),
                ),
                IconButton.filled(
                  tooltip: '发送问题',
                  onPressed: _quickSending ? null : _sendQuickQuestion,
                  icon: _quickSending
                      ? SizedBox.square(
                          dimension: 18,
                          child: Icon(
                            Icons.more_horiz_rounded,
                            color: cs.onSurfaceVariant,
                          ),
                        )
                      : const Icon(Icons.arrow_upward_rounded),
                ),
              ],
            ),
            Row(
              children: [
                FilterChip(
                  avatar: Icon(
                    _quickImages
                        ? Icons.visibility_outlined
                        : Icons.visibility_off_outlined,
                    size: 16,
                  ),
                  label: Text(l10n.studyAllowImages),
                  tooltip: l10n.studyAllowImagesHint,
                  selected: _quickImages,
                  showCheckmark: false,
                  visualDensity: VisualDensity.compact,
                  onSelected: _quickSending
                      ? null
                      : (v) => unawaited(_setQuickVision(v)),
                ),
                const SizedBox(width: Space.sm),
                if (_quickImages && _quickCapturing)
                  Icon(Icons.crop_rounded, size: 18, color: cs.outline)
                else
                  for (final v in _quickVisuals)
                    Padding(
                      padding: const EdgeInsetsDirectional.only(end: Space.xs),
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(Corners.xs),
                        child: DecoratedBox(
                          position: DecorationPosition.foreground,
                          decoration: BoxDecoration(
                            border: Border.all(color: cs.outlineVariant),
                            borderRadius: BorderRadius.circular(Corners.xs),
                          ),
                          child: Image.memory(
                            base64Decode('${v['data']}'),
                            key: ValueKey('study-quick-crop-${v['source_id']}'),
                            height: 36,
                            fit: BoxFit.contain,
                            gaplessPlayback: true,
                          ),
                        ),
                      ),
                    ),
                const Spacer(),
                Text(
                  l10n.studyQuickKeys,
                  style: tt.labelSmall?.copyWith(color: cs.outline),
                ),
              ],
            ),
            if (unreadable && !_quickImages)
              Padding(
                padding: const EdgeInsets.only(top: Space.xs),
                child: Text(
                  l10n.studyQuickNeedsVision,
                  key: const ValueKey('study-quick-needs-vision'),
                  style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
                ),
              ),
            if (_quickError != null)
              Padding(
                padding: const EdgeInsets.only(top: Space.xs),
                child: Text(
                  _quickError!,
                  key: const ValueKey('study-quick-error'),
                  style: tt.bodySmall?.copyWith(color: cs.error),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// A source the quick composer will send.
class _Bound {
  const _Bound({
    required this.id,
    required this.label,
    required this.icon,
    this.detail = '',
    this.readable,
  });
  final String id, label, detail;
  final IconData icon;

  /// Readable text is present; false needs 「看图」; null is unknown.
  final bool? readable;
}

class _BoundChip extends StatelessWidget {
  const _BoundChip({required this.bound, this.onRemove});
  final _Bound bound;
  final VoidCallback? onRemove;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final readable = bound.readable;
    return Container(
      key: ValueKey('study-bound-${bound.id}'),
      height: 30,
      padding: const EdgeInsetsDirectional.only(start: Space.sm),
      decoration: BoxDecoration(
        color: cs.onSurface.withValues(alpha: .06),
        borderRadius: BorderRadius.circular(Corners.sm),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(bound.icon, size: 14, color: cs.onSurfaceVariant),
          const SizedBox(width: Space.xs),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 150),
            child: Text(
              bound.label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: tt.labelMedium,
            ),
          ),
          if (bound.detail.isNotEmpty) ...[
            const SizedBox(width: Space.xs),
            Text(
              bound.detail,
              style: tt.labelSmall?.copyWith(color: cs.onSurfaceVariant),
            ),
          ],
          if (readable != null) ...[
            const SizedBox(width: Space.xs),
            Tooltip(
              message: readable
                  ? l10n.studyBoundReadable
                  : l10n.studyBoundNeedsVision,
              child: Icon(
                readable
                    ? Icons.short_text_rounded
                    : Icons.image_search_rounded,
                size: 14,
                color: readable ? cs.primary : cs.tertiary,
              ),
            ),
          ],
          SizedBox.square(
            dimension: 30,
            child: IconButton(
              tooltip: l10n.studyBoundRemove,
              padding: EdgeInsets.zero,
              iconSize: 14,
              onPressed: onRemove,
              icon: const Icon(Icons.close_rounded),
            ),
          ),
        ],
      ),
    );
  }
}

/// A selection-bar verb; the primary one is filled.
class _BarAction extends StatelessWidget {
  const _BarAction({
    required this.label,
    required this.icon,
    required this.tooltip,
    required this.onPressed,
    this.primary = false,
  });
  final String label, tooltip;
  final IconData icon;
  final VoidCallback? onPressed;
  final bool primary;

  @override
  Widget build(BuildContext context) {
    const size = Size(44, 44);
    final shape = const StadiumBorder();
    return Tooltip(
      message: tooltip,
      child: primary
          ? FilledButton.tonalIcon(
              onPressed: onPressed,
              style: FilledButton.styleFrom(minimumSize: size, shape: shape),
              icon: Icon(icon, size: 18),
              label: Text(label),
            )
          : TextButton.icon(
              onPressed: onPressed,
              style: TextButton.styleFrom(minimumSize: size, shape: shape),
              icon: Icon(icon, size: 18),
              label: Text(label),
            ),
    );
  }
}

/// Places a floating control 8 dp off the anchor's top-right corner, below it
/// when the top is covered, and 12 dp inside the window.
/// After tldraw's contextual toolbar (https://tldraw.dev/examples/contextual-toolbar)
/// and the M3 floating toolbar guidance (https://m3.material.io/components/toolbars/guidelines).
class _FloatingLayout extends SingleChildLayoutDelegate {
  _FloatingLayout(this.anchor, this.topInset);
  final Rect anchor;
  final double topInset;
  static const gap = 8.0, edge = 12.0;

  @override
  BoxConstraints getConstraintsForChild(BoxConstraints c) =>
      BoxConstraints.loose(
        Size(
          math.max(0, c.maxWidth - 2 * edge),
          math.max(0, c.maxHeight - 2 * edge),
        ),
      );

  @override
  Offset getPositionForChild(Size size, Size child) {
    final x = (anchor.right - child.width).clamp(
      edge,
      math.max(edge, size.width - child.width - edge),
    );
    final top = topInset + edge, bottom = size.height - edge;
    var y = anchor.top - gap - child.height;
    if (y < top) y = anchor.bottom + gap;
    // Neither side has room: stay inside the window over the selection.
    if (y + child.height > bottom) {
      y = (anchor.top - gap - child.height).clamp(
        top,
        math.max(top, bottom - child.height),
      );
    }
    return Offset(x.toDouble(), y);
  }

  @override
  bool shouldRelayout(_FloatingLayout old) =>
      old.anchor != anchor || old.topInset != topInset;
}

/// A short scale-and-fade as a floating control appears or changes mode.
class _Appear extends StatelessWidget {
  const _Appear({super.key, required this.child});
  final Widget child;

  @override
  Widget build(BuildContext context) => TweenAnimationBuilder<double>(
    key: ValueKey(child.key),
    tween: Tween(begin: 0, end: 1),
    duration: MediaQuery.disableAnimationsOf(context)
        ? Duration.zero
        : Motion.short4,
    curve: Motion.emphasizedDecelerate,
    builder: (_, t, child) => Opacity(
      opacity: t,
      child: Transform.scale(
        scale: .96 + .04 * t,
        alignment: Alignment.bottomRight,
        child: child,
      ),
    ),
    child: child,
  );
}

class _InteractionFeedback extends CustomPainter {
  _InteractionFeedback(this.state, Listenable repaint)
    : super(repaint: repaint);
  final StudyCanvasState state;
  @override
  void paint(Canvas canvas, Size size) {
    final s = state, d = s.document!;
    Offset screen(Offset p) => p * d.zoom + d.camera;
    final accent = state.chrome.accent;
    final line = Paint()
      ..color = accent
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5;
    void dash(Path path) {
      for (final metric in path.computeMetrics()) {
        for (var start = 0.0; start < metric.length; start += 10) {
          canvas.drawPath(
            metric.extractPath(start, math.min(start + 6, metric.length)),
            line,
          );
        }
      }
    }

    final loop = s._erasing ? s._eraserLasso : s._lasso;
    if (loop.length > 1 && !(s._lassoRectangle && !s._erasing)) {
      final path = Path()..addPolygon(loop.map(screen).toList(), true);
      canvas.drawPath(path, Paint()..color = state.chrome.accentWash);
      dash(path);
    } else {
      final r =
          s._marquee ??
          (s._selectionReady ? s._selectionBounds : null) ??
          s._sourceHighlight;
      if (r != null) {
        final box = Rect.fromPoints(screen(r.topLeft), screen(r.bottomRight));
        dash(Path()..addRect(box));
        if (s._selectionReady) {
          canvas.drawCircle(
            box.bottomRight,
            7,
            Paint()..color = state.chrome.paper,
          );
          canvas.drawCircle(box.bottomRight, 7, line);
        }
      }
    }
    final cursor = s._eraserCursor;
    if (cursor != null &&
        (s._tool == StudyTool.eraser || s._temporaryEraser) &&
        s._eraserMode != 'lasso') {
      canvas.drawCircle(
        cursor,
        s._cursorRadius,
        Paint()..color = state.chrome.paper.withValues(alpha: .24),
      );
      canvas.drawCircle(
        cursor,
        s._cursorRadius,
        Paint()
          ..color = state.chrome.paper
          ..style = PaintingStyle.stroke
          ..strokeWidth = 3,
      );
      canvas.drawCircle(
        cursor,
        s._cursorRadius,
        Paint()
          ..color = state.chrome.inkSoft
          ..style = PaintingStyle.stroke
          ..strokeWidth = 1.3,
      );
      if (s._temporaryEraser)
        canvas.drawCircle(cursor, 2, Paint()..color = accent);
    }
  }

  @override
  bool shouldRepaint(_InteractionFeedback old) => old.state != state;
}

/// A ring that fills under a finger resting on a card, so the hold that
/// lifts the card reads as deliberate rather than as a stall. It stays
/// hidden through the first moments, when most contacts become pans.
class _HoldCue extends StatelessWidget {
  const _HoldCue({required this.at});
  final ValueListenable<Offset?> at;

  static const double _size = 44;

  @override
  Widget build(BuildContext context) => ValueListenableBuilder<Offset?>(
    valueListenable: at,
    builder: (context, p, _) => p == null
        ? const SizedBox.shrink()
        : Positioned(
            left: p.dx - _size / 2,
            top: p.dy - _size / 2,
            width: _size,
            height: _size,
            child: IgnorePointer(
              // A new hold starts a new fill.
              child: _HoldRing(key: ValueKey(p)),
            ),
          ),
  );
}

class _HoldRing extends StatefulWidget {
  const _HoldRing({super.key});
  @override
  State<_HoldRing> createState() => _HoldRingState();
}

class _HoldRingState extends State<_HoldRing>
    with SingleTickerProviderStateMixin {
  // Silent for the first quarter of the hold.
  static const _quiet = .25;
  late final _fill = AnimationController(
    vsync: this,
    duration: canvasCardHoldDelay,
  )..forward();

  @override
  void dispose() {
    _fill.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (MediaQuery.disableAnimationsOf(context)) return const SizedBox();
    final cs = Theme.of(context).colorScheme;
    return AnimatedBuilder(
      animation: _fill,
      builder: (context, _) {
        final t = ((_fill.value - _quiet) / (1 - _quiet)).clamp(0.0, 1.0);
        return Opacity(
          opacity: Motion.emphasizedDecelerate.transform(
            (t * 4).clamp(0.0, 1.0),
          ),
          child: CustomPaint(
            painter: _RingPainter(
              Motion.emphasized.transform(t),
              cs.primary,
              cs.surface,
            ),
          ),
        );
      },
    );
  }
}

class _RingPainter extends CustomPainter {
  _RingPainter(this.t, this.ink, this.halo);
  final double t;
  final Color ink, halo;

  @override
  void paint(Canvas canvas, Size size) {
    final r = size.shortestSide / 2 - 3;
    final c = size.center(Offset.zero);
    // A soft disc keeps the ring legible over any material.
    canvas.drawCircle(c, r + 2, Paint()..color = halo.withValues(alpha: .55));
    canvas.drawCircle(
      c,
      r,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3
        ..color = ink.withValues(alpha: .18),
    );
    canvas.drawArc(
      Rect.fromCircle(center: c, radius: r),
      -math.pi / 2,
      2 * math.pi * t,
      false,
      Paint()
        ..style = PaintingStyle.stroke
        ..strokeWidth = 3
        ..strokeCap = StrokeCap.round
        ..color = ink,
    );
  }

  @override
  bool shouldRepaint(_RingPainter o) =>
      o.t != t || o.ink != ink || o.halo != halo;
}
