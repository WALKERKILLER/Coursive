part of 'study_canvas.dart';

class _StudyReadingPosition {
  _StudyReadingPosition(StudyCanvasState owner, StudyCard? card)
    : camera = owner.document!.camera,
      zoom = owner.document!.zoom,
      thread = owner._activeConversation,
      selection = Set.of(owner._selection),
      detail = Map.of(owner._detail.json()),
      region = owner._regionSelection,
      detailRegion = owner._detail.region,
      bounds = owner._selectionBounds,
      highlight = owner._sourceHighlight,
      polygon = List.of(owner._selectionPolygon),
      ready = owner._selectionReady,
      card = card?.copy(),
      time = card?.type == 'lecture'
          ? owner.widget.ctl.position.value.inMilliseconds
          : owner._localVideoTimes[card?.id],
      scroll =
          owner._messageScroll[owner._activeConversation]?.hasClients == true
          ? owner._messageScroll[owner._activeConversation]!.offset
          : 0;
  final Offset camera;
  final double zoom, scroll;
  final String? thread;
  final Set<String> selection;
  final Map detail;
  final Rect? region, detailRegion, bounds, highlight;
  final List<Offset> polygon;
  final bool ready;
  final StudyCard? card;
  final int? time;
}

extension _StudySourceNavigation on StudyCanvasState {
  Rect get _sourceViewport {
    final width = _viewport.width;
    final sidebar = _stageOpen
        ? math.min(420.0, math.max(240.0, width - 16)) + 24
        : 0.0;
    final left = _aiPlacement.edge == StudyDockEdge.left ? sidebar : 0.0;
    return Rect.fromLTWH(
      left + 24,
      _topBarsBottom + 24,
      math.max(120, width - sidebar - 48),
      math.max(120, _viewport.height - _topBarsBottom - 104),
    );
  }

  void _rememberReadingPosition(_StudyReadingPosition position) {
    _returnPositions.add(position);
    if (_returnPositions.length > 20) _returnPositions.removeAt(0);
    _update(() {});
  }

  Future<void> _focusSource(StudyAnchor anchor) async {
    final card = document?.cards
        .where((c) => c.id == anchor.objectId)
        .firstOrNull;
    final position = _StudyReadingPosition(this, card);
    if (await _navigateAnchor(anchor)) _rememberReadingPosition(position);
  }

  Future<bool> _navigateAnchor(StudyAnchor a) async {
    if (a.workspaceId.isNotEmpty && a.workspaceId != widget.ctl.lectureId) {
      _toast(l10n.studyCitationUnavailable);
      return false;
    }
    final d = document!;
    final card = d.cards.where((c) => c.id == a.objectId).firstOrNull;
    Rect? target = card?.rect;
    if (a.kind == 'ink') {
      final ink = d.strokes.where((s) => a.strokeIds.contains(s.id)).toList();
      if (ink.length != a.strokeIds.length || ink.isEmpty) {
        _toast(l10n.studyCitationUnavailable);
        return false;
      }
      target = ink.map((s) => s.bounds).reduce((r, s) => r.expandToInclude(s));
    }
    if (target == null || card != null && !d.visible(card.layer)) {
      _toast(l10n.studyCitationUnavailable);
      return false;
    }
    Map? page;
    try {
      if (card?.type == 'pdfdoc' && a.page > 0 && card!.page != a.page)
        page = await widget.api.studyPDFPage(
          widget.ctl.lectureId,
          card.source,
          a.page,
        ) as Map;
      if (!mounted || document != d) return false;
      final text = page == null ? card?.contentText : '${page['text']}';
      if (a.quote.isNotEmpty &&
          a.kind != 'ink' &&
          (text == null ||
              a.start < 0 ||
              a.start + a.quote.length > text.length ||
              text.substring(a.start, a.start + a.quote.length) != a.quote))
        throw FormatException(l10n.studyCitationUnavailable);
      if (card != null && a.kind == 'video') {
        if (card.type == 'lecture') {
          widget.ctl.seekTo(a.timeStartMs);
          await widget.ctl.player.pause();
        } else {
          final local = _localVideoKeys[card.id]?.currentState;
          if (local != null) {
            await local.seekPaused(a.timeStartMs);
          }
          _localVideoTimes[card.id] = a.timeStartMs;
        }
      }
      if (page != null && card != null) {
        card
          ..page = a.page
          ..path = '${page['file']}'
          ..text = '${page['text']}'
          ..pageAspect = (page['width'] as num) / (page['height'] as num)
          ..textBoxes = (page['boxes'] as List? ?? []).cast<Map>();
      }
      if (card?.collapsed == true) {
        card!.collapsed = false;
        card.rect = Rect.fromLTWH(
          card.rect.left,
          card.rect.top,
          card.rect.width,
          math.max(240, card.expandedH),
        );
      }
      target = card?.rect ?? target;
      if (card != null && a.region != null) {
        final r = a.region!, parent = _sourceBounds(card);
        target = Rect.fromLTWH(
          parent.left + r.left * parent.width,
          parent.top + r.top * parent.height,
          r.width * parent.width,
          r.height * parent.height,
        );
      }
      final viewport = _sourceViewport;
      final zoom = math
          .min(
            viewport.width / math.max(100, target.width),
            viewport.height / math.max(100, target.height),
          )
          .clamp(.05, 2)
          .toDouble();
      _flyTo(viewport.center - target.center * zoom, zoom);
      _selection
        ..clear()
        ..addAll(card == null ? <String>[] : [card.id]);
      _sourceHighlight = target;
      _regionSelection = null;
      _selectionReady = false;
      if (page != null) d.changed();
      _cameraChanged();
      if (a.end > a.start && card != null && a.end <= card.contentText.length)
        _toast(
          l10n.studySourceExcerpt(
            card.contentText.substring(a.start, math.min(a.end, a.start + 160)),
          ),
        );
      return true;
    } catch (e) {
      if (mounted) _toast(l10n.studyCitationUnavailable);
      return false;
    }
  }

  Future<void> _focusCitation(StudyCitation citation) {
    final key = '${citation.source['id']}:${citation.source['offset']}';
    return _sourceNavigations.putIfAbsent(
      key,
      () =>
          _resolveCitation(citation)
              .whenComplete(() => _sourceNavigations.remove(key)),
    );
  }

  Future<void> _resolveCitation(StudyCitation citation) async {
    final d = document!;
    try {
      final source = Map<String, dynamic>.from(
        await widget.api.resolveStudySource(
          widget.ctl.lectureId,
          citation.source,
        ) as Map,
      );
      if (!mounted || document != d) return;
      final path = '${source['video_path'] ?? source['path']}';
      final lecture = '${source['lecture_id']}';
      final exactPDF =
          source['kind'] == 'pdf' && (source['page'] as num? ?? 0) > 0;
      final video = source['video_path'] != null;
      final excerpt = !video && !exactPDF;
      final identity = excerpt
          ? 'excerpt:${source['id']}:${source['offset']}:${source['fingerprint']}'
          : '';
      var card = d.cards
          .where(
            (c) =>
                c.originalPath == path &&
                c.sourceLectureId == lecture &&
                (excerpt
                    ? c.documentId == identity
                    : c.type == (video ? 'video' : 'pdfdoc')),
          )
          .firstOrNull;
      final position = _StudyReadingPosition(this, card);
      final fresh = card == null;
      if (card != null && !d.visible(card.layer))
        throw FormatException(l10n.studyCitationUnavailable);
      if (card == null) {
        if (!d.editable(d.activeLayer) || d.cards.length >= 1998)
          throw FormatException(l10n.studyLocked);
        if (excerpt) {
          card = StudyCard(
            id: studyId(),
            type: 'text',
            layer: d.activeLayer,
            rect: _freeSpot(const Size(480, 360)),
            title: citation.label,
            text: '${source['text']}',
            originalPath: path,
            sourceLectureId: lecture,
            documentId: identity,
          );
        } else {
          final imported = await widget.api.importStudyMaterial(
            widget.ctl.lectureId,
            path,
          );
          if (!mounted || document != d) return;
          card = StudyCard.fromJson((imported['objects'] as List).first as Map);
          card.layer = d.activeLayer;
          card.rect = _freeSpot(card.rect.size);
        }
      }
      // Stage any fallible page read before changing the user's current view.
      final page = (source['page'] as num? ?? 0).toInt();
      if (exactPDF && card.page != page) {
        final value = await widget.api.studyPDFPage(
          widget.ctl.lectureId,
          card.source,
          page,
        );
        if (!mounted || document != d) return;
        card
          ..page = page
          ..path = '${value['file']}'
          ..text = '${value['text']}'
          ..pageAspect = (value['width'] as num) / (value['height'] as num)
          ..textBoxes = (value['boxes'] as List? ?? []).cast<Map>();
      }
      if (video)
        _localVideoTimes[card.id] = (source['time_start_ms'] as num? ?? 0)
            .toInt();
      if (fresh) {
        d.checkpoint();
        d.cards.add(card);
        d.changed();
        if (!await flush() || !mounted || document != d) return;
        _update(() {});
        await WidgetsBinding.instance.endOfFrame;
      }
      final anchor = StudyAnchor(
        id: citation.id,
        objectId: card.id,
        kind: video
            ? 'video'
            : excerpt
            ? 'text'
            : 'pdf',
        workspaceId: widget.ctl.lectureId,
        sourceLectureId: lecture,
        originalPath: path,
        page: page,
        timeStartMs: (source['time_start_ms'] as num? ?? 0).toInt(),
        timeEndMs: (source['time_end_ms'] as num? ?? 0).toInt(),
        quote: excerpt
            ? ('${source['text']}').substring(
                0,
                math.min(2000, '${source['text']}'.length),
              )
            : '',
      );
      if (excerpt && card.sources.isEmpty) card.sources.add(anchor);
      if (await _navigateAnchor(anchor)) _rememberReadingPosition(position);
    } catch (e) {
      if (mounted) _toast(l10n.studyCitationUnavailable);
    }
  }

  Future<void> _returnToQuestion() async {
    if (_returnPositions.isEmpty) return;
    final position = _returnPositions.last, d = document!;
    final old = position.card;
    final card = old == null
        ? null
        : d.cards.where((c) => c.id == old.id).firstOrNull;
    if (card != null && old != null) {
      // Only view fields are restored. Current ink, text edits and layout survive.
      if (card.type == 'pdfdoc') {
        card
          ..page = old.page
          ..path = old.path
          ..text = old.text
          ..textBoxes = old.textBoxes
          ..pageAspect = old.pageAspect;
      }
      if (card.collapsed != old.collapsed) {
        card.collapsed = old.collapsed;
        card.rect = Rect.fromLTWH(
          card.rect.left,
          card.rect.top,
          card.rect.width,
          old.rect.height,
        );
      }
      if (position.time != null) {
        if (card.type == 'lecture') {
          widget.ctl.seekTo(position.time!);
          await widget.ctl.player.pause();
        } else if (card.type == 'video') {
          _localVideoTimes[card.id] = position.time!;
          final local = _localVideoKeys[card.id]?.currentState;
          if (local != null) {
            await local.player.seek(Duration(milliseconds: position.time!));
            await local.player.pause();
          }
        }
      }
    }
    if (!mounted || document != d) return;
    _returnPositions.removeLast();
    _selection
      ..clear()
      ..addAll(
        position.selection.where((id) => d.cards.any((c) => c.id == id)),
      );
    _detail
      ..objectId = (position.detail['object_id'] as String?)
      ..start = (position.detail['start'] as int? ?? 0)
      ..end = (position.detail['end'] as int? ?? 0)
      ..region = position.detailRegion;
    _detail.strokeIds
      ..clear()
      ..addAll((position.detail['stroke_ids'] as List).cast<String>());
    _regionSelection = position.region;
    _selectionBounds = position.bounds;
    _sourceHighlight = position.highlight;
    _selectionPolygon = position.polygon;
    _selectionReady = position.ready;
    if (d.cards.any((c) => c.id == position.thread))
      _activeConversation = position.thread;
    _flyTo(position.camera, position.zoom);
    _update(() {});
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final scroll = _messageScroll[position.thread];
      if (scroll != null && scroll.hasClients)
        scroll.jumpTo(
          position.scroll.clamp(0, scroll.position.maxScrollExtent),
        );
    });
  }
}
