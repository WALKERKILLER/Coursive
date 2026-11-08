part of 'study_canvas.dart';

// Grounded answers: numbered citations, the view a citation leads to and the
// way back. Contract: docs/specs/0027-grounded-canvas-answers.md.

final _evidenceMarker = RegExp(r'\[\[e:([A-Za-z0-9_-]{1,12})\]\]');

/// Replaces host-issued markers with numbered links in the order of
/// [evidence]; a marker without kept evidence is dropped.
String studyEvidenceMarkdown(String answer, List evidence) {
  final order = {
    for (var i = 0; i < evidence.length; i++)
      '${(evidence[i] as Map)['id']}': i + 1,
  };
  return answer.replaceAllMapped(_evidenceMarker, (m) {
    final n = order[m[1]];
    return n == null ? '' : '[$n](coursive-evidence:${m[1]})';
  });
}

/// The answer as a reader keeps it: no markers.
String studyPlainAnswer(String answer) =>
    answer.replaceAll(RegExp(r'\[\[(?:e|next):[^\]\n]*\]\]'), '');

final _legacyCitation = RegExp(r'\[[^\]\n]+\]\(coursive://citation/(e\d+)\)');

/// Course answers from the first course-scope builds linked citations as
/// `[n](coursive://citation/eN)` beside a `citations` table. They load as
/// schema 2 evidence, so one renderer and one way back serve every answer.
void studyUpgradeCitations(Map turn) {
  final result = turn['result'];
  if (result is! Map ||
      result['schema_version'] == 2 ||
      result['citations'] is! List) {
    return;
  }
  final evidence = [
    for (final c in (result['citations'] as List).whereType<Map>())
      if (c['source'] is Map && '${c['id']}'.startsWith('e'))
        _legacyEvidence(c),
  ].take(24).toList();
  final ids = {for (final e in evidence) e['id']};
  turn['answer'] = '${turn['answer'] ?? ''}'.replaceAllMapped(
    _legacyCitation,
    (m) => ids.contains(m[1]) ? '[[e:${m[1]}]]' : '',
  );
  turn['result'] = {
    'sources': result['sources'] ?? const [],
    'schema_version': 2,
    'evidence': evidence,
    'suggestions': const [],
  };
}

Map<String, dynamic> _legacyEvidence(Map c) {
  final s = Map<String, dynamic>.from(c['source'] as Map)
    ..remove('text')
    ..remove('snippet');
  final from = (s['time_start_ms'] as num? ?? 0).toInt();
  final to = (s['time_end_ms'] as num? ?? 0).toInt();
  final page = (s['page'] as num? ?? 0).toInt();
  final label = '${c['label'] ?? ''}';
  return {
    'id': '${c['id']}',
    'source_id': 'course-${s['id']}',
    'kind': to > from ? 'timestamped_transcript' : 'exact_text',
    'status': 'verified',
    'label': label.substring(0, math.min(120, label.length)),
    'anchor': {
      'id': '${s['id']}',
      'workspace_id': '',
      'object_id': '',
      'kind': to > from
          ? 'video'
          : page > 0
          ? 'pdf'
          : 'text',
      'source_lecture_id': '${s['lecture_id'] ?? ''}',
      'original_path': '${s['path'] ?? ''}',
      if (page > 0) 'page': page,
      if (to > from) ...{'time_start_ms': from, 'time_end_ms': to},
      'fingerprint': '${s['fingerprint'] ?? ''}',
    },
    'course_source': s,
  };
}

/// A pinned answer's identity: its conversation turn, hashed (FNV-1a) when
/// the key would exceed the 100 characters a card generation may hold.
String studyPinGeneration(String conversation, String turn) {
  final key = 'canvas-turn:$conversation:$turn';
  if (key.length <= 100) return key;
  var h = 0xcbf29ce484222325;
  for (final b in utf8.encode(key)) {
    h = (h ^ b) * 0x100000001b3;
  }
  return 'canvas-turn:#${h.toUnsigned(64).toRadixString(16)}';
}

/// Schema 2 citations of a saved turn, or null for older turns.
List<Map>? studyTurnEvidence(Map turn) {
  final result = turn['result'];
  if (result is! Map || result['schema_version'] != 2) return null;
  return (result['evidence'] as List? ?? const []).whereType<Map>().toList();
}

/// Where a citation points on the canvas, in world coordinates.
class _EvidenceView {
  const _EvidenceView(this.card, this.rects, {this.page = 0}) : problem = null;
  const _EvidenceView.invalid(this.problem)
    : card = null,
      rects = const [],
      page = 0;
  final StudyCard? card;

  /// Text lines, a region or ink bounds; empty means the whole card.
  final List<Rect> rects;

  /// A PDF page the card is not showing.
  final int page;

  /// Why the citation cannot be followed; null when it can.
  final String? problem;

  Rect? get bounds =>
      rects.isEmpty ? card?.rect : rects.reduce((a, b) => a.expandToInclude(b));
}

/// A view left by following a citation.
class _ViewMark {
  _ViewMark({
    required this.camera,
    required this.zoom,
    required this.stageOpen,
    required this.selection,
    this.reviewPeek = false,
    this.focus,
    this.thread,
    this.scroll,
    this.region,
  });
  final Offset camera;
  final double zoom;
  final bool stageOpen, reviewPeek;
  final Set<String> selection;

  /// The conversation on stage and how far its messages were scrolled.
  final String? thread;
  final double? scroll;

  /// A lassoed region, kept so the learner can ask about it again.
  final ({
    Rect region,
    Rect? bounds,
    List<Offset> polygon,
    StudySelection detail,
  })?
  region;

  /// What had keyboard focus, such as the outline item being edited.
  final FocusNode? focus;

  /// The PDF page a card showed before navigation turned it, and the page
  /// navigation turned it to.
  String? card;
  int shownPage = 0;
  ({int page, String path, String text, List<Map> boxes, double aspect})? page;

  /// The video navigation paused, its earlier time and the time it went to.
  String? video;
  int videoBefore = 0, videoAt = 0;
}

extension _StudyEvidence on StudyCanvasState {
  StudyCard? _evidenceCard(StudyAnchor a) => document!.cards
      .where(
        (c) =>
            a.objectId == 'lecture' ? c.type == 'lecture' : c.id == a.objectId,
      )
      .firstOrNull;

  /// Validates [evidence] against the canvas as it is now.
  _EvidenceView _evidenceView(Map evidence) {
    final d = document!;
    final a = StudyAnchor.fromJson(evidence['anchor'] as Map? ?? const {});
    if ((evidence['status'] ?? 'verified') != 'verified') {
      return _EvidenceView.invalid(l10n.studyEvidenceStale);
    }
    // A course passage is checked against its file when it is followed.
    if (evidence['course_source'] is Map) return const _EvidenceView(null, []);
    if (a.workspaceId.isNotEmpty && a.workspaceId != widget.ctl.lectureId) {
      return _EvidenceView.invalid(l10n.studyEvidenceOtherLecture);
    }
    if (a.kind == 'ink') {
      final ink = d.strokes
          .where((s) => a.strokeIds.contains(s.id) && d.visible(s.layer))
          .toList();
      if (ink.isEmpty || ink.length != a.strokeIds.length) {
        return _EvidenceView.invalid(l10n.studyEvidenceStale);
      }
      return _EvidenceView(null, [
        ink.map((s) => s.bounds).reduce((x, y) => x.expandToInclude(y)),
      ]);
    }
    final card = _evidenceCard(a);
    if (card == null || !d.visible(card.layer)) {
      return _EvidenceView.invalid(l10n.studyEvidenceMissing);
    }
    final upright = card.quarterTurns % 4 == 0;
    if (card.type == 'pdfdoc' && a.page > 0 && card.page != a.page) {
      return _EvidenceView(card, const [], page: a.page);
    }
    if (evidence['kind'] == 'exact_text' && a.end > a.start) {
      final text = card.type == 'pdfdoc' ? card.text : card.contentText;
      if (a.end > text.length ||
          (a.quote.isNotEmpty && text.substring(a.start, a.end) != a.quote)) {
        return _EvidenceView.invalid(l10n.studyEvidenceStale);
      }
      if (card.type == 'pdfdoc' && upright) {
        final lines = _textLines(card, a.start, a.end);
        if (lines.isNotEmpty) return _EvidenceView(card, lines);
      }
    }
    if (a.region != null && upright && card.type != 'lecture') {
      final b = _sourceBounds(card), r = a.region!;
      return _EvidenceView(card, [
        Rect.fromLTWH(
          b.left + r.left * b.width,
          b.top + r.top * b.height,
          r.width * b.width,
          r.height * b.height,
        ),
      ]);
    }
    return _EvidenceView(card, const []);
  }

  /// The cited range's text lines on a page, from its real word boxes.
  List<Rect> _textLines(StudyCard card, int start, int end) {
    final b = _sourceBounds(card);
    double n(Object? v) => v is num && v.isFinite ? v.toDouble() : 0;
    final lines = <Rect>[];
    for (final box in card.textBoxes) {
      final s = n(box['start']), e = n(box['end']);
      if (e <= start || s >= end) continue;
      final r = Rect.fromLTWH(
        b.left + n(box['x']) * b.width,
        b.top + n(box['y']) * b.height,
        n(box['w']) * b.width,
        n(box['h']) * b.height,
      );
      final last = lines.lastOrNull;
      // Words on one baseline read as one highlighted line.
      if (last != null &&
          (r.center.dy - last.center.dy).abs() < last.height * .5 &&
          r.left >= last.left - 1) {
        lines.last = last.expandToInclude(r);
      } else {
        lines.add(r);
      }
    }
    return lines;
  }

  /// A short locator: page, time range, region or ink.
  String _evidenceLocator(Map evidence) {
    final a = StudyAnchor.fromJson(evidence['anchor'] as Map? ?? const {});
    return switch ('${evidence['kind']}') {
      'timestamped_transcript' =>
        '${fmtClock(a.timeStartMs)}–${fmtClock(a.timeEndMs)}',
      'visual_observation' => l10n.studyEvidenceSeen,
      _ when a.kind == 'ink' => l10n.studyEvidenceInk,
      _ when a.page > 0 => l10n.studySourcePage(a.page),
      _ => '${evidence['label'] ?? ''}',
    };
  }

  /// Hover and keyboard focus show the cited material; nothing moves.
  void _previewEvidence(Map? evidence) {
    if (identical(_evidenceFocus.value, evidence)) return;
    _evidenceLanding.reset();
    _evidenceFocus.value = evidence;
    if (evidence != null) _evidenceSweep.forward(from: 0);
  }

  /// Validates, remembers the current view and carries it to [evidence].
  Future<void> _followEvidence(Map evidence) async {
    final d = document;
    if (d == null) return;
    if (evidence['course_source'] is Map) {
      return _followCourseEvidence(evidence);
    }
    var view = _evidenceView(evidence);
    if (view.problem != null) {
      _toast(view.problem!);
      return;
    }
    final a = StudyAnchor.fromJson(evidence['anchor'] as Map? ?? const {});
    final card = view.card;
    Map? page;
    if (view.page > 0 && card != null) {
      try {
        page = await widget.api.studyPDFPage(
          widget.ctl.lectureId,
          card.source,
          view.page,
        ) as Map;
      } catch (_) {
        if (mounted) _toast(l10n.studyEvidenceMissing);
        return;
      }
      if (!mounted || document != d || !d.cards.contains(card)) return;
      final text = '${page['text']}';
      if (a.end > text.length ||
          (a.quote.isNotEmpty && text.substring(a.start, a.end) != a.quote)) {
        _toast(l10n.studyEvidenceStale);
        return;
      }
    }
    final mark = _ViewMark(
      camera: d.camera,
      zoom: d.zoom,
      stageOpen: _stageOpen,
      selection: {..._selection},
      reviewPeek: _reviewPeek,
      thread: _activeConversation,
      scroll: switch (_messageScroll[_activeConversation]) {
        final c? when c.hasClients => c.offset,
        _ => null,
      },
      region: switch (_regionSelection) {
        final r? when _selectionReady => (
          region: r,
          bounds: _selectionBounds,
          polygon: List.of(_selectionPolygon),
          detail: StudySelection()
            ..objectId = _detail.objectId
            ..start = _detail.start
            ..end = _detail.end
            ..region = _detail.region
            ..strokeIds.addAll(_detail.strokeIds),
        ),
        _ => null,
      },
      // A click on a citation in the review leaves no node focused; the
      // item it was clicked in is where the learner returns.
      focus: switch (FocusManager.instance.primaryFocus) {
        final FocusScopeNode _ when _review != null => _reviewFocus,
        final node => node,
      },
    );
    if (page != null && card != null) {
      mark
        ..card = card.id
        ..shownPage = view.page
        ..page = (
          page: card.page,
          path: card.path,
          text: card.text,
          boxes: card.textBoxes,
          aspect: card.pageAspect,
        );
      // Reading state, like the camera: no undo entry and no edit.
      card
        ..pageAspect = (page['width'] as num) / (page['height'] as num)
        ..page = view.page
        ..path = '${page['file']}'
        ..text = '${page['text']}'
        ..textBoxes = (page['boxes'] as List? ?? []).cast<Map>();
      view = _evidenceView(evidence);
    }
    if (a.kind == 'video' && card != null) {
      // A cited moment is shown, not played over the learner.
      mark
        ..video = card.id
        ..videoAt = a.timeStartMs;
      if (card.type == 'lecture') {
        mark.videoBefore = widget.ctl.position.value.inMilliseconds;
        widget.ctl.seekTo(a.timeStartMs);
        unawaited(widget.ctl.player.pause());
      } else {
        final local = _localVideoKeys[card.id]?.currentState;
        mark.videoBefore =
            local?.player.state.position.inMilliseconds ??
            _localVideoTimes[card.id] ??
            0;
        _localVideoTimes[card.id] = a.timeStartMs;
        // A player still opening seeks once it is ready.
        if (local != null) {
          unawaited(local.seekPaused(a.timeStartMs).catchError((_) {}));
        }
      }
    }
    _bookmarks.add(mark);
    if (_bookmarks.length > 10) _bookmarks.removeAt(0);
    // Keys go back to the canvas so Esc and Alt+← return from here.
    FocusManager.instance.primaryFocus?.unfocus(
      disposition: UnfocusDisposition.previouslyFocusedChild,
    );
    _update(() {
      // A narrow window gives the whole view to the source until the return.
      if (_viewport.width < 720) {
        _stageOpen = false;
        if (_review != null) _reviewPeek = true;
      }
      _selection.clear();
      _regionSelection = null;
      _selectionReady = false;
    });
    _scene.value++;
    _flyToEvidence(view);
    _evidenceFocus.value = evidence;
    _evidenceSweep.forward(from: 0);
    _evidenceLanding.forward(from: 0);
  }

  /// The part of the window that shows the canvas, without the sidebar,
  /// the top controls and the pen dock.
  Rect _openArea() {
    var area = Rect.fromLTRB(
      Space.lg,
      _topBarsBottom + Space.xxxl,
      _viewport.width - Space.lg,
      _viewport.height - Space.huge,
    );
    // The open review takes its side, or the bottom of a narrow window.
    if (_reviewRect case final r?) {
      area = _reviewNarrow
          ? Rect.fromLTRB(
              area.left,
              area.top,
              area.right,
              math.min(area.bottom, r.top - Space.sm),
            )
          : r.center.dx < _viewport.width / 2
          ? Rect.fromLTRB(
              math.max(area.left, r.right + Space.lg),
              area.top,
              area.right,
              area.bottom,
            )
          : Rect.fromLTRB(
              area.left,
              area.top,
              math.min(area.right, r.left - Space.lg),
              area.bottom,
            );
    }
    if (_stageOpen && _viewport.width >= 720) {
      final w =
          math.min(420.0, math.max(240.0, _viewport.width - 16)) + Space.lg;
      area = _aiPlacement.edge == StudyDockEdge.left
          ? Rect.fromLTRB(area.left + w, area.top, area.right, area.bottom)
          : Rect.fromLTRB(area.left, area.top, area.right - w, area.bottom);
    }
    return area;
  }

  void _flyToEvidence(_EvidenceView view) {
    var target = view.bounds;
    if (target == null) return;
    final card = view.card;
    if (card != null && view.rects.isNotEmpty && card.type == 'pdfdoc') {
      // Read a passage across the page's width, with the lines around it.
      final page = _sourceBounds(card);
      target = Rect.fromLTRB(
        page.left,
        math.max(page.top, target.top - page.height * .12),
        page.right,
        math.min(page.bottom, target.bottom + page.height * .12),
      );
    }
    final area = _openArea();
    final zoom = math
        .min(
          area.width / math.max(1, target.width),
          area.height / math.max(1, target.height),
        )
        .clamp(.05, 2.0)
        .toDouble();
    _flyTo(area.center - target.center * zoom, zoom);
  }

  /// Restores the newest view left by a citation; false when there is none.
  /// Alt+← / ⌘[ follow PDF readers' back navigation, e.g. Zotero's
  /// (https://www.zotero.org/support/kb/keyboard_shortcuts).
  bool _goBack() {
    final d = document;
    if (d == null || _bookmarks.isEmpty) return false;
    final mark = _bookmarks.removeLast();
    final card = d.cards.where((c) => c.id == mark.card).firstOrNull;
    final page = mark.page;
    // Only a page the learner has not turned since is turned back.
    if (card != null && page != null && card.page == mark.shownPage) {
      card
        ..page = page.page
        ..path = page.path
        ..text = page.text
        ..textBoxes = page.boxes
        ..pageAspect = page.aspect;
    }
    final video = d.cards.where((c) => c.id == mark.video).firstOrNull;
    if (video != null) {
      if (video.type == 'lecture') {
        final player = widget.ctl.player;
        if (!player.state.playing &&
            (widget.ctl.position.value.inMilliseconds - mark.videoAt).abs() <=
                1500) {
          widget.ctl.seekTo(mark.videoBefore);
        }
      } else {
        final local = _localVideoKeys[video.id]?.currentState;
        final at =
            local?.player.state.position.inMilliseconds ??
            _localVideoTimes[video.id] ??
            mark.videoAt;
        if (local?.player.state.playing != true &&
            (at - mark.videoAt).abs() <= 1500) {
          _localVideoTimes[video.id] = mark.videoBefore;
          if (local != null) {
            unawaited(
              local.player.seek(Duration(milliseconds: mark.videoBefore)),
            );
          }
        }
      }
    }
    _evidenceLanding.reset();
    _evidenceFocus.value = null;
    _update(() {
      _stageOpen = mark.stageOpen;
      _reviewPeek = mark.reviewPeek;
      _selection
        ..clear()
        ..addAll(mark.selection.where((id) => d.cards.any((c) => c.id == id)));
      if (d.cards.any((c) => c.id == mark.thread)) {
        _activeConversation = mark.thread;
      }
      final r = mark.region;
      if (r != null) {
        final detail = r.detail;
        _detail
          ..objectId = d.cards.any((c) => c.id == detail.objectId)
              ? detail.objectId
              : null
          ..start = detail.start
          ..end = detail.end
          ..region = detail.region;
        _detail.strokeIds
          ..clear()
          ..addAll(
            detail.strokeIds.where((id) => d.strokes.any((s) => s.id == id)),
          );
        _regionSelection = r.region;
        _selectionBounds = r.bounds ?? r.region;
        _selectionPolygon = r.polygon;
        _selectionReady = true;
      }
    });
    // The messages scroll back to the answer the citation was in.
    if (mark.scroll case final offset?) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        final c = _messageScroll[mark.thread];
        if (c != null && c.hasClients) {
          c.jumpTo(offset.clamp(0, c.position.maxScrollExtent));
        }
      });
    }
    _scene.value++;
    _flyTo(mark.camera, mark.zoom);
    // Back where the learner was typing or reviewing, focus included.
    final focus = mark.focus;
    if (focus != null && focus.context?.mounted == true) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (focus.context?.mounted == true) focus.requestFocus();
      });
    }
    return true;
  }

  /// 「返回原处」, shown while a citation's view can be left.
  Widget _returnPill() => Positioned(
    top: _topBarsBottom + Space.sm,
    left: 0,
    right: 0,
    child: Center(
      child: AnimatedSwitcher(
        duration: Motion.medium1,
        switchInCurve: Motion.emphasizedDecelerate,
        switchOutCurve: Motion.emphasizedAccelerate,
        transitionBuilder: (child, a) => FadeTransition(
          opacity: a,
          child: SlideTransition(
            position: Tween(
              begin: const Offset(0, -.4),
              end: Offset.zero,
            ).animate(a),
            child: child,
          ),
        ),
        child: _bookmarks.isEmpty
            ? const SizedBox.shrink()
            : _ReturnPill(
                key: _returnKey,
                count: _bookmarks.length,
                onPressed: _goBack,
              ),
      ),
    ),
  );
}

class _ReturnPill extends StatelessWidget {
  const _ReturnPill({super.key, required this.count, required this.onPressed});
  final int count;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final mac = Theme.of(context).platform == TargetPlatform.macOS;
    return GlassSurface(
      radius: Corners.full,
      depth: GlassDepth.float,
      child: Tooltip(
        message: mac ? l10n.studyReturnKeysMac : l10n.studyReturnKeys,
        child: InkWell(
          key: const ValueKey('study-return'),
          customBorder: const StadiumBorder(),
          onTap: onPressed,
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: 44),
            child: Padding(
              padding: const EdgeInsetsDirectional.fromSTEB(
                Space.md,
                Space.xs,
                Space.lg,
                Space.xs,
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.undo_rounded, size: 18, color: cs.primary),
                  const SizedBox(width: Space.sm),
                  Text(l10n.studyReturn, style: tt.labelLarge),
                  if (count > 1) ...[
                    const SizedBox(width: Space.sm),
                    Text(
                      '$count',
                      style: tt.labelMedium?.copyWith(
                        color: cs.onSurfaceVariant,
                        fontFeatures: const [ui.FontFeature.tabularFigures()],
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// A numbered citation inside an answer. Pointer hover and keyboard focus
/// preview the passage and light it on the canvas; a click follows it. On
/// touch, a tap opens the preview with an explicit 「定位」.
class _CitationMark extends StatefulWidget {
  const _CitationMark({
    required this.owner,
    required this.evidence,
    required this.number,
  });
  final StudyCanvasState owner;
  final Map evidence;
  final int number;

  @override
  State<_CitationMark> createState() => _CitationMarkState();
}

class _CitationMarkState extends State<_CitationMark> {
  final _link = LayerLink();
  final _portal = OverlayPortalController();
  bool _hover = false, _focus = false, _pinned = false;
  PointerDeviceKind? _kind;

  StudyCanvasState get owner => widget.owner;

  void _sync() {
    final open = _hover || _focus || _pinned;
    open ? _portal.show() : _portal.hide();
    if (open) {
      owner._previewEvidence(widget.evidence);
    } else if (owner._evidenceFocus.value == widget.evidence &&
        !owner._evidenceLanding.isAnimating) {
      owner._previewEvidence(null);
    }
    setState(() {});
  }

  /// Centred over the mark, but slid sideways to stay 12 dp inside the
  /// screen: on a phone a mark near the edge has no room for half a preview.
  double _previewShift(BuildContext overlay) {
    final box = context.findRenderObject() as RenderBox?;
    if (box == null || !box.hasSize) return 0;
    final width = MediaQuery.sizeOf(overlay).width;
    final x = box.localToGlobal(box.size.topCenter(Offset.zero)).dx;
    const half = _CitationPreview.maxWidth / 2, edge = 12.0;
    if (width < 2 * (half + edge)) return width / 2 - x;
    if (x - half < edge) return edge - (x - half);
    if (x + half > width - edge) return width - edge - (x + half);
    return 0;
  }

  void _follow() {
    _pinned = _hover = _focus = false;
    _portal.hide();
    unawaited(owner._followEvidence(widget.evidence));
    setState(() {});
  }

  @override
  void dispose() {
    if (owner._evidenceFocus.value == widget.evidence &&
        !owner._evidenceLanding.isAnimating) {
      owner._evidenceFocus.value = null;
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final base = DefaultTextStyle.of(context).style;
    final size = base.fontSize ?? 14;
    final problem = owner.document == null
        ? null
        : owner._evidenceView(widget.evidence).problem;
    final active = _hover || _focus || _pinned;
    final mark = AnimatedContainer(
      duration: Motion.short3,
      constraints: BoxConstraints(minWidth: size * 1.05),
      padding: const EdgeInsets.symmetric(horizontal: 3.5, vertical: .5),
      decoration: BoxDecoration(
        color: problem != null
            ? cs.onSurface.withValues(alpha: .06)
            : active
            ? cs.primary
            : cs.primary.withValues(alpha: .1),
        borderRadius: BorderRadius.circular(Corners.full),
      ),
      child: Text(
        '${widget.number}',
        textAlign: TextAlign.center,
        style: base.copyWith(
          fontSize: size * .68,
          height: 1.25,
          fontWeight: FontWeight.w700,
          fontFeatures: const [ui.FontFeature.tabularFigures()],
          color: problem != null
              ? cs.outline
              : active
              ? cs.onPrimary
              : cs.primary,
          decoration: problem != null ? TextDecoration.lineThrough : null,
        ),
      ),
    );
    return OverlayPortal(
      controller: _portal,
      overlayChildBuilder: (overlay) => CompositedTransformFollower(
        link: _link,
        targetAnchor: Alignment.topCenter,
        followerAnchor: Alignment.bottomCenter,
        offset: Offset(_previewShift(overlay), -Space.xs),
        child: Align(
          alignment: Alignment.bottomCenter,
          child: TapRegion(
            groupId: this,
            onTapOutside: (_) {
              if (!_pinned) return;
              _pinned = false;
              _sync();
            },
            child: IgnorePointer(
              ignoring: !_pinned,
              child: _CitationPreview(
                owner: owner,
                evidence: widget.evidence,
                number: widget.number,
                problem: problem,
                onLocate: _pinned && problem == null ? _follow : null,
              ),
            ),
          ),
        ),
      ),
      child: CompositedTransformTarget(
        link: _link,
        child: TapRegion(
          groupId: this,
          child: MouseRegion(
            cursor: SystemMouseCursors.click,
            onEnter: (_) {
              _hover = true;
              _sync();
            },
            onExit: (_) {
              _hover = false;
              _sync();
            },
            child: Listener(
              onPointerDown: (e) => _kind = e.kind,
              child: FocusableActionDetector(
                onShowFocusHighlight: (v) {
                  _focus = v;
                  _sync();
                },
                actions: {
                  ActivateIntent: CallbackAction<ActivateIntent>(
                    onInvoke: (_) {
                      _follow();
                      return null;
                    },
                  ),
                },
                child: Semantics(
                  button: true,
                  label: l10n.studyEvidenceSemantics(
                    widget.number,
                    owner._evidenceLocator(widget.evidence),
                  ),
                  child: GestureDetector(
                    behavior: HitTestBehavior.opaque,
                    onTap: () {
                      if (_kind == PointerDeviceKind.touch) {
                        _pinned = !_pinned;
                        _sync();
                      } else {
                        _follow();
                      }
                    },
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 1.5),
                      child: Transform.translate(
                        offset: Offset(0, -size * .32),
                        child: mark,
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// What a citation points at: where, the quoted words, and whether it still holds.
class _CitationPreview extends StatelessWidget {
  const _CitationPreview({
    required this.owner,
    required this.evidence,
    required this.number,
    required this.problem,
    this.onLocate,
  });
  final StudyCanvasState owner;
  final Map evidence;
  final int number;
  final String? problem;
  final VoidCallback? onLocate;

  static const double maxWidth = 300;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final quote = '${(evidence['anchor'] as Map?)?['quote'] ?? ''}'.trim();
    final label = '${evidence['label'] ?? ''}';
    final locator = owner._evidenceLocator(evidence);
    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: maxWidth),
      child: GlassSurface(
        depth: GlassDepth.float,
        radius: Corners.md,
        padding: const EdgeInsets.fromLTRB(
          Space.md,
          Space.sm,
          Space.md,
          Space.sm,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              [
                if (label.isNotEmpty && label != locator) label,
                locator,
              ].join(' · '),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: tt.labelSmall?.copyWith(color: cs.onSurfaceVariant),
            ),
            if (quote.isNotEmpty) ...[
              const SizedBox(height: Space.xs),
              DecoratedBox(
                decoration: BoxDecoration(
                  border: BorderDirectional(
                    start: BorderSide(
                      color: cs.primary.withValues(alpha: .5),
                      width: 2,
                    ),
                  ),
                ),
                child: Padding(
                  padding: const EdgeInsetsDirectional.only(start: Space.sm),
                  child: Text(
                    quote,
                    maxLines: 4,
                    overflow: TextOverflow.ellipsis,
                    style: tt.bodySmall?.copyWith(height: 1.5),
                  ),
                ),
              ),
            ] else if (evidence['kind'] == 'visual_observation') ...[
              const SizedBox(height: Space.xs),
              Text(l10n.studyEvidenceSeenBody, style: tt.bodySmall),
            ],
            if (problem != null) ...[
              const SizedBox(height: Space.xs),
              Text(problem!, style: tt.labelSmall?.copyWith(color: cs.error)),
            ],
            if (onLocate != null)
              Align(
                alignment: AlignmentDirectional.centerEnd,
                child: TextButton.icon(
                  onPressed: onLocate,
                  icon: const Icon(Icons.my_location_rounded, size: 16),
                  label: Text(l10n.studyEvidenceLocate),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

/// How much of an answer rests on material: passages, transcript lines and
/// inspected images, and how many of those no longer hold.
class _Provenance extends StatelessWidget {
  const _Provenance({
    required this.owner,
    required this.evidence,
    required this.suggestions,
    required this.onFollowup,
    this.onOutline,
    this.trailing,
  });
  final StudyCanvasState owner;
  final List<Map> evidence;
  final List suggestions;
  final ValueChanged<String> onFollowup;

  /// Starts 「整理为复习提纲」 after the learner confirms its sources.
  final VoidCallback? onOutline;

  /// Sits at the end of the coverage line (the pin button).
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    int count(String kind) => evidence.where((e) => e['kind'] == kind).length;
    final stale = owner.document == null
        ? 0
        : evidence.where((e) => owner._evidenceView(e).problem != null).length;
    final parts = [
      if (count('exact_text') > 0)
        l10n.studyCoveragePassages(count('exact_text')),
      if (count('timestamped_transcript') > 0)
        l10n.studyCoverageTranscript(count('timestamped_transcript')),
      if (count('visual_observation') > 0)
        l10n.studyCoverageVisual(count('visual_observation')),
    ];
    final byId = {for (final e in evidence) '${e['id']}': e};
    final next = [
      for (final s in suggestions.whereType<Map>())
        if (s['kind'] == 'open_source' && byId.containsKey(s['evidence_id']))
          ActionChip(
            avatar: const Icon(Icons.north_east_rounded, size: 16),
            label: Text(
              l10n.studySuggestOpen(
                owner._evidenceLocator(byId[s['evidence_id']]!),
              ),
            ),
            visualDensity: VisualDensity.compact,
            onPressed: () =>
                unawaited(owner._followEvidence(byId[s['evidence_id']]!)),
          )
        else if (s['kind'] == 'ask_followup' && '${s['text'] ?? ''}'.isNotEmpty)
          ActionChip(
            avatar: const Icon(
              Icons.subdirectory_arrow_right_rounded,
              size: 16,
            ),
            label: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 260),
              child: Text(
                '${s['text']}',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            tooltip: l10n.studySuggestFollowupHint,
            visualDensity: VisualDensity.compact,
            onPressed: () => onFollowup('${s['text']}'),
          )
        else if (s['kind'] == 'propose_outline' && onOutline != null)
          ActionChip(
            key: const ValueKey('study-suggest-outline'),
            avatar: const Icon(Icons.format_list_bulleted_rounded, size: 16),
            label: Text(l10n.outlineAsk),
            tooltip: l10n.outlineAskHint,
            visualDensity: VisualDensity.compact,
            onPressed: onOutline,
          ),
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                parts.isEmpty
                    ? l10n.studyCoverageNone
                    : [
                        l10n.studyCoverageLead(parts.join(' · ')),
                        if (stale > 0) l10n.studyCoverageStale(stale),
                      ].join(' · '),
                key: const ValueKey('study-coverage'),
                style: tt.labelSmall?.copyWith(
                  color: stale > 0 ? cs.error : cs.onSurfaceVariant,
                ),
              ),
            ),
            ?trailing,
          ],
        ),
        if (next.isNotEmpty) ...[
          const SizedBox(height: Space.sm),
          Wrap(spacing: Space.xs, runSpacing: Space.xs, children: next),
        ],
      ],
    );
  }
}

/// Lights the cited material like a highlighter pass; material outside the
/// view gets a small marker on the nearest edge instead.
class _EvidencePainter extends CustomPainter {
  _EvidencePainter(this.s)
    : super(
        repaint: Listenable.merge([
          s._camera,
          s._scene,
          s._evidenceFocus,
          s._evidenceSweep,
          s._evidenceLanding,
        ]),
      );
  final StudyCanvasState s;

  @override
  void paint(Canvas canvas, Size size) {
    final evidence = s._evidenceFocus.value, d = s.document;
    if (evidence == null || d == null) return;
    final view = s._evidenceView(evidence);
    if (view.problem != null || view.bounds == null) return;
    final landing = s._evidenceLanding;
    // A followed citation stays lit while the eye settles, then fades.
    final fade = landing.value == 0
        ? 1.0
        : 1 -
              Curves.easeIn.transform(
                ((landing.value - .65) / .35).clamp(0, 1),
              );
    if (fade <= 0) return;
    final sweep = Motion.emphasizedDecelerate.transform(s._evidenceSweep.value);
    final accent = s.chrome.accent;
    Rect screen(Rect r) => Rect.fromPoints(
      r.topLeft * d.zoom + d.camera,
      r.bottomRight * d.zoom + d.camera,
    );
    final bounds = screen(view.bounds!);
    final visible = (Offset.zero & size).deflate(Space.sm);
    if (!visible.overlaps(bounds)) {
      _edge(
        canvas,
        visible,
        bounds,
        s._evidenceLocator(evidence),
        accent,
        fade,
      );
      return;
    }
    if (view.rects.isEmpty) {
      // A whole card, or a page the card is not showing yet.
      final card = RRect.fromRectAndRadius(
        bounds.inflate(4),
        const Radius.circular(Corners.md),
      );
      canvas.drawRRect(
        card,
        Paint()
          ..color = accent.withValues(alpha: .18 * fade)
          ..maskFilter = const MaskFilter.blur(BlurStyle.outer, 10),
      );
      canvas.drawRRect(
        card,
        Paint()
          ..color = accent.withValues(alpha: .8 * fade)
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2,
      );
      if (view.page > 0) {
        _label(
          canvas,
          bounds.topRight + const Offset(-Space.sm, Space.sm),
          s._evidenceLocator(evidence),
          accent,
          fade,
          alignEnd: true,
        );
      }
      return;
    }
    final region = evidence['kind'] != 'exact_text';
    for (final r in view.rects.map(screen)) {
      final lit = Rect.fromLTWH(
        r.left - 2,
        r.top - 1,
        (r.width + 4) * sweep,
        r.height + 2,
      );
      canvas.drawRRect(
        RRect.fromRectAndRadius(lit, const Radius.circular(3)),
        Paint()..color = accent.withValues(alpha: (region ? .1 : .24) * fade),
      );
      if (region) {
        canvas.drawRRect(
          RRect.fromRectAndRadius(
            r.inflate(2),
            const Radius.circular(Corners.xs),
          ),
          Paint()
            ..color = accent.withValues(alpha: .85 * fade)
            ..style = PaintingStyle.stroke
            ..strokeWidth = 1.5,
        );
      }
    }
  }

  void _edge(
    Canvas canvas,
    Rect visible,
    Rect target,
    String text,
    Color accent,
    double fade,
  ) {
    final c = visible.center, t = target.center;
    final v = t - c;
    // Where the ray from the centre to the target leaves the visible area.
    final kx = v.dx == 0
        ? double.infinity
        : (visible.width / 2 - 28) / v.dx.abs();
    final ky = v.dy == 0
        ? double.infinity
        : (visible.height / 2 - 20) / v.dy.abs();
    final at = c + v * math.min(kx, ky);
    final angle = math.atan2(v.dy, v.dx);
    canvas.drawCircle(
      at,
      14,
      Paint()..color = accent.withValues(alpha: .92 * fade),
    );
    canvas.save();
    canvas.translate(at.dx, at.dy);
    canvas.rotate(angle);
    canvas.drawPath(
      Path()
        ..moveTo(5, 0)
        ..lineTo(-3, -4.5)
        ..lineTo(-3, 4.5)
        ..close(),
      Paint()..color = s.chrome.paper.withValues(alpha: fade),
    );
    canvas.restore();
    final end = v.dx > 0;
    _label(
      canvas,
      at + Offset(end ? -20 : 20, 0),
      text,
      accent,
      fade,
      alignEnd: end,
      middle: true,
    );
  }

  void _label(
    Canvas canvas,
    Offset at,
    String text,
    Color accent,
    double fade, {
    bool alignEnd = false,
    bool middle = false,
  }) {
    if (text.isEmpty) return;
    final painter = TextPainter(
      text: TextSpan(
        text: text,
        style: TextStyle(
          fontSize: 11.5,
          fontWeight: FontWeight.w600,
          color: s.chrome.paper.withValues(alpha: fade),
        ),
      ),
      textDirection: TextDirection.ltr,
      maxLines: 1,
      ellipsis: '…',
    )..layout(maxWidth: 180);
    final box = Rect.fromLTWH(
      alignEnd ? at.dx - painter.width - 16 : at.dx,
      middle ? at.dy - (painter.height + 6) / 2 : at.dy,
      painter.width + 16,
      painter.height + 6,
    );
    canvas.drawRRect(
      RRect.fromRectAndRadius(box, const Radius.circular(Corners.full)),
      Paint()..color = accent.withValues(alpha: .92 * fade),
    );
    painter.paint(canvas, box.topLeft + const Offset(8, 3));
    painter.dispose();
  }

  @override
  bool shouldRepaint(_EvidencePainter old) => old.s != s;
}
