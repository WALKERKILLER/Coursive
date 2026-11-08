part of 'study_canvas.dart';

/// The outline under review: its conversation node and turn.
typedef _ReviewTarget = ({String card, String turn});

extension _StudyOutline on StudyCanvasState {
  _ConversationBodyState? get _reviewConversation =>
      _review == null ? null : _conversationKeys[_review!.card]?.currentState;

  Map? get _reviewTurn {
    final c = _reviewConversation;
    if (c == null || c.data == null) return null;
    return c.turns
        .whereType<Map>()
        .where(
          (t) => '${t['id']}' == _review!.turn && outlineProposal(t) != null,
        )
        .firstOrNull;
  }

  /// Opens the review beside the canvas. A window too narrow for both
  /// panels gives the review the side the conversation had.
  void _openReview(String card, String turn) {
    _liveThreads.add(card);
    _update(() {
      _review = (card: card, turn: turn);
      _reviewPeek = false;
      if (_viewport.width < 1180) _stageOpen = false;
    });
  }

  void _closeReview() => _update(() => _review = null);

  bool get _reviewNarrow => _viewport.width < 720;

  /// Where the open review sits, in canvas pixels; null when closed.
  Rect? get _reviewRect {
    if (_review == null || _reviewTurn == null) return null;
    final top = math.max(76.0, _topBarsBottom + Space.sm);
    if (_reviewNarrow) {
      final h = _reviewPeek ? 76.0 : _viewport.height * .62;
      return Rect.fromLTWH(
        Space.sm,
        _viewport.height - h - Space.sm,
        _viewport.width - 2 * Space.sm,
        h,
      );
    }
    final w = math.min(400.0, _viewport.width * .42);
    final left = _aiPlacement.edge == StudyDockEdge.left
        ? _viewport.width - w - Space.sm
        : Space.sm;
    return Rect.fromLTWH(left, top, w, _viewport.height - top - Space.huge);
  }

  /// Citations whose material no longer validates on the canvas.
  Set<String> _staleEvidence(List<Map> evidence) => {
    if (document != null)
      for (final e in evidence)
        if (_evidenceView(e).problem != null) '${e['id']}',
  };

  OutlineExport _outlineExport(Map turn) {
    final evidence = studyTurnEvidence(turn) ?? const [];
    return outlineExport(
      outlineProposal(turn)!,
      evidence,
      stale: _staleEvidence(evidence),
      lectureId: widget.ctl.lectureId,
    );
  }

  /// The selected ink a mark points at, still on a visible layer.
  List<StudyStroke> _markInk(Map item) {
    final ids = {
      for (final id
          in (item['anchor'] as Map?)?['stroke_ids'] as List? ?? const [])
        '$id',
    };
    final d = document;
    if (ids.isEmpty || d == null) return const [];
    return d.strokes
        .where((s) => ids.contains(s.id) && d.visible(s.layer))
        .toList();
  }

  Widget _reviewPanel() {
    final rect = _reviewRect;
    final conversation = _reviewConversation, turn = _reviewTurn;
    return Positioned.fromRect(
      rect: rect ?? Rect.zero,
      child: rect == null || conversation == null || turn == null
          ? const SizedBox.shrink()
          : _OutlineReview(
              key: _reviewKey,
              owner: this,
              conversation: conversation,
              turn: turn,
              narrow: _reviewNarrow,
            ),
    );
  }
}

/// The review of an outline proposal: the learner accepts, rewrites,
/// reorders, splits, merges, flags or deletes each item and gives each
/// handwritten mark its meaning. Every change stays in the conversation's
/// draft; only 「保存为课程笔记」 writes a note.
class _OutlineReview extends StatefulWidget {
  const _OutlineReview({
    super.key,
    required this.owner,
    required this.conversation,
    required this.turn,
    required this.narrow,
  });
  final StudyCanvasState owner;
  final _ConversationBodyState conversation;
  final Map turn;
  final bool narrow;

  @override
  State<_OutlineReview> createState() => _OutlineReviewState();
}

class _OutlineReviewState extends State<_OutlineReview> {
  final _undo = <String>[];
  final _editors = <String, TextEditingController>{};
  final _rowKeys = <String, GlobalKey>{};
  final _rowFocus = <String, FocusNode>{};
  final _scroll = ScrollController();
  String? _editing;
  bool _saving = false, _conflict = false;
  String? _error, _notice;
  Timer? _persist;

  StudyCanvasState get owner => widget.owner;
  Map get proposal => outlineProposal(widget.turn)!;
  List<Map> get evidence => studyTurnEvidence(widget.turn) ?? const [];
  List get sections => proposal['sections'] as List;

  @override
  void dispose() {
    _persist?.cancel();
    if (_persist != null) unawaited(widget.conversation.save());
    for (final c in _editors.values) {
      c.dispose();
    }
    for (final f in _rowFocus.values) {
      f.dispose();
    }
    _scroll.dispose();
    super.dispose();
  }

  /// Records an undo step, applies [change] and keeps the draft.
  void _change(VoidCallback change, {bool snapshot = true}) {
    if (snapshot) {
      _undo.add(jsonEncode(proposal));
      if (_undo.length > 60) _undo.removeAt(0);
    }
    setState(change);
    _notice = null;
    _persist?.cancel();
    _persist = Timer(const Duration(milliseconds: 500), () {
      _persist = null;
      unawaited(widget.conversation.save());
    });
  }

  void _undoLast() {
    if (_undo.isEmpty) return;
    final previous = jsonDecode(_undo.removeLast()) as Map;
    _editing = null;
    _change(
      () => (widget.turn['result'] as Map)['outline_proposal'] = previous,
      snapshot: false,
    );
  }

  Iterable<(Map, Map?)> get _rows sync* {
    for (final s in sections.whereType<Map>()) {
      yield (s, null);
      for (final i in (s['items'] as List).whereType<Map>()) {
        yield (s, i);
      }
    }
  }

  String _rowId((Map, Map?) r) => '${(r.$2 ?? r.$1)['id']}';

  /// A flat list where a heading starts a section; moving rows regroups them.
  /// [to] is the row's index after the move.
  void _reorder(int from, int to) {
    final rows = _rows.toList();
    final moved = rows.removeAt(from);
    rows.insert(to, moved);
    if (rows.first.$2 != null) return;
    final next = <Map>[];
    for (final (s, i) in rows) {
      if (i == null) {
        next.add({...s, 'items': []});
      } else {
        (next.last['items'] as List).add(i);
      }
    }
    if (next.any((s) => (s['items'] as List).length > 20)) {
      setState(() => _notice = l10n.outlineTooManyItems);
      return;
    }
    _change(() => proposal['sections'] = next);
  }

  void _move(Map item, int by) {
    final rows = _rows.toList();
    final at = rows.indexWhere((r) => identical(r.$2, item));
    final to = at + by;
    // Passing a heading moves the item into the neighbouring section.
    if (to < 1 || to >= rows.length) return;
    _reorder(at, to);
    _rowFocus['${item['id']}']?.requestFocus();
  }

  (Map, int)? _locate(Map item) {
    for (final s in sections.whereType<Map>()) {
      final i = (s['items'] as List).indexWhere((x) => identical(x, item));
      if (i >= 0) return (s, i);
    }
    return null;
  }

  String _freshId() {
    final used = {for (final (s, i) in _rows) '${(i ?? s)['id']}'};
    var n = used.length + 1;
    while (used.contains('i$n')) {
      n++;
    }
    return 'i$n';
  }

  void _review(Map item, String review) =>
      _change(() => item['review'] = review);

  void _toggleConfirm(Map item) {
    if (item['kind'] == 'unresolved_mark') return;
    _review(item, item['review'] == 'confirmed' ? 'pending' : 'confirmed');
  }

  void _toggleCheck(Map item) {
    if (item['kind'] == 'unresolved_mark') return;
    _review(item, item['review'] == 'to_check' ? 'pending' : 'to_check');
  }

  void _delete(Map item) {
    final at = _locate(item);
    if (at == null) return;
    _change(() => (at.$1['items'] as List).removeAt(at.$2));
  }

  /// Splits at the caret while editing, otherwise after the first sentence.
  void _split(Map item) {
    final at = _locate(item);
    if (at == null || (at.$1['items'] as List).length >= 20) return;
    final text = '${item['text']}';
    final caret = _editing == item['id']
        ? _editors['${item['id']}']?.selection.baseOffset ?? -1
        : -1;
    var cut = caret > 0 && caret < text.length ? caret : -1;
    if (cut < 0) {
      final m = RegExp(r'[。！？；.!?;]\s*').firstMatch(text);
      cut = m == null || m.end >= text.length ? text.length ~/ 2 : m.end;
    }
    final head = text.substring(0, cut).trim(),
        tail = text.substring(cut).trim();
    if (head.isEmpty || tail.isEmpty) return;
    _editing = null;
    _change(() {
      item['text'] = head;
      (at.$1['items'] as List).insert(at.$2 + 1, {
        ...item,
        'id': _freshId(),
        'text': tail,
        'evidence_ids': [...outlineEvidenceIds(item)],
      });
    });
  }

  void _mergeNext(Map item) {
    final at = _locate(item);
    final items = at?.$1['items'] as List?;
    if (at == null || items == null || at.$2 + 1 >= items.length) return;
    final next = items[at.$2 + 1] as Map;
    if (next['kind'] == 'unresolved_mark' ||
        item['kind'] == 'unresolved_mark') {
      return;
    }
    _change(() {
      item['text'] = '${item['text']}\n${next['text']}';
      item['evidence_ids'] = {
        ...outlineEvidenceIds(item),
        ...outlineEvidenceIds(next),
      }.take(24).toList();
      items.removeAt(at.$2 + 1);
    });
  }

  void _addItem(Map section) {
    final items = section['items'] as List;
    if (items.length >= 20) return;
    final item = {
      'id': _freshId(),
      'kind': 'summary',
      'text': '',
      'evidence_ids': [],
      'review': 'confirmed',
    };
    _change(() => items.add(item));
    _edit(item);
  }

  void _deleteSection(Map section) {
    final i = sections.indexWhere((s) => identical(s, section));
    if (i < 0 || sections.length == 1) return;
    _change(() {
      // Items stay: they join the section above.
      final items = section['items'] as List;
      sections.removeAt(i);
      final target = sections[i == 0 ? 0 : i - 1] as Map;
      final room = 20 - (target['items'] as List).length;
      (target['items'] as List).addAll(items.take(room));
    });
  }

  /// Only the learner gives a handwritten mark its meaning.
  void _choose(Map item, String? choice, [String text = '']) => _change(() {
    if (choice == null) {
      item
        ..remove('choice')
        ..remove('choice_text')
        ..['review'] = 'needs_interpretation';
    } else {
      item
        ..['choice'] = choice
        ..['review'] = 'confirmed';
      if (choice == 'custom') {
        item['choice_text'] = text;
      } else {
        item.remove('choice_text');
      }
    }
  });

  void _edit(Map item) {
    final id = '${item['id']}';
    _editors[id] ??= TextEditingController(text: '${item['text']}');
    _editors[id]!.text = '${item['text']}';
    _undo.add(jsonEncode(proposal));
    setState(() => _editing = id);
  }

  void _commit(Map item) {
    final text = _editors['${item['id']}']?.text.trim() ?? '';
    setState(() => _editing = null);
    if (text.isEmpty) {
      _delete(item);
    } else if (text != item['text']) {
      _change(
        () =>
            item['text'] = text.length > 1300 ? text.substring(0, 1300) : text,
        snapshot: false,
      );
    } else {
      _undo.removeLast();
    }
    _rowFocus['${item['id']}']?.requestFocus();
  }

  void _cancelEdit(Map item) {
    setState(() => _editing = null);
    if (_undo.isNotEmpty) _undo.removeLast();
    _rowFocus['${item['id']}']?.requestFocus();
  }

  /// Carries the list to the next item that still needs attention.
  void _jump(bool Function(Map) needs) {
    final rows = _rows.where((r) => r.$2 != null && needs(r.$2!)).toList();
    if (rows.isEmpty) return;
    final focused = rows.indexWhere(
      (r) => _rowFocus[_rowId(r)]?.hasFocus == true,
    );
    final target = rows[(focused + 1) % rows.length];
    final ctx = _rowKeys[_rowId(target)]?.currentContext;
    if (ctx != null) {
      unawaited(
        Scrollable.ensureVisible(
          ctx,
          duration: Motion.medium2,
          curve: Motion.emphasized,
          alignment: .3,
        ),
      );
    }
    _rowFocus[_rowId(target)]?.requestFocus();
  }

  KeyEventResult _rowKey(Map item, KeyEvent e) {
    if (e is! KeyDownEvent || _editing != null) return KeyEventResult.ignored;
    final alt = HardwareKeyboard.instance.isAltPressed;
    final k = e.logicalKey;
    if (alt && k == LogicalKeyboardKey.arrowUp) {
      _move(item, -1);
    } else if (alt && k == LogicalKeyboardKey.arrowDown) {
      _move(item, 1);
    } else if (k == LogicalKeyboardKey.keyA) {
      _toggleConfirm(item);
    } else if (k == LogicalKeyboardKey.keyF) {
      _toggleCheck(item);
    } else if (k == LogicalKeyboardKey.enter || k == LogicalKeyboardKey.keyE) {
      _edit(item);
    } else if (k == LogicalKeyboardKey.delete ||
        k == LogicalKeyboardKey.backspace) {
      _delete(item);
    } else if (k == LogicalKeyboardKey.arrowDown ||
        k == LogicalKeyboardKey.arrowUp) {
      final ids = [
        for (final r in _rows)
          if (r.$2 != null) _rowId(r),
      ];
      final at =
          ids.indexOf('${item['id']}') +
          (k == LogicalKeyboardKey.arrowDown ? 1 : -1);
      if (at >= 0 && at < ids.length) {
        final ctx = _rowKeys[ids[at]]?.currentContext;
        if (ctx != null) {
          unawaited(
            Scrollable.ensureVisible(
              ctx,
              duration: Motion.short4,
              alignmentPolicy: ScrollPositionAlignmentPolicy.keepVisibleAtEnd,
            ),
          );
        }
        _rowFocus[ids[at]]?.requestFocus();
      }
    } else {
      return KeyEventResult.ignored;
    }
    return KeyEventResult.handled;
  }

  String get _markdown => outlineMarkdown(owner._outlineExport(widget.turn));

  /// Saves the reviewed outline as this lecture's note under its stable ID.
  /// A retry after a lost response finds its own note; a note changed
  /// elsewhere is never overwritten.
  Future<void> _save({bool copy = false}) async {
    if (_saving) return;
    if (_editing != null) {
      final item = _rows
          .map((r) => r.$2)
          .firstWhere(
            (i) => i != null && '${i['id']}' == _editing,
            orElse: () => null,
          );
      if (item != null) _commit(item);
    }
    final p = proposal, md = _markdown;
    final api = owner.widget.api;
    setState(() {
      _saving = true;
      _error = null;
      _conflict = false;
    });
    try {
      final id = copy
          ? '${outlineNoteId(p)}-${DateTime.now().millisecondsSinceEpoch.toRadixString(36)}'
          : '${p['note_id'] ?? outlineNoteId(p)}';
      String? base;
      if (!copy) {
        Map? current;
        try {
          current =
              await api.get('/api/notes/${Uri.encodeComponent(id)}') as Map;
        } on ApiError catch (e) {
          if (e.status != 404) rethrow;
        }
        if (current != null) {
          final body = '${current['body'] ?? ''}';
          if (body != md &&
              (p['state'] != 'accepted' ||
                  outlineNoteHash(body) != p['note_hash'])) {
            setState(() {
              _conflict = true;
              _error = l10n.outlineSaveConflict;
            });
            return;
          }
          base = body;
        }
      }
      if (base != md) {
        await api.put('/api/notes', {
          'id': id,
          'base_body': ?base,
          'course_id': owner.widget.ctl.lecture?.courseId ?? '',
          'lecture_id': owner.widget.ctl.lectureId,
          'title': '${p['title']}',
          'body': md,
          'source': 'user',
        });
      }
      _change(() {
        p
          ..['state'] = 'accepted'
          ..['note_id'] = id
          ..['note_hash'] = outlineNoteHash(md);
      }, snapshot: false);
      _persist?.cancel();
      _persist = null;
      if (!await widget.conversation.save()) {
        if (mounted) setState(() => _error = l10n.outlineStateNotRecorded);
        return;
      }
      if (mounted) setState(() => _notice = l10n.outlineSaved);
    } on ApiError catch (e) {
      if (mounted) {
        setState(() {
          _conflict = e.status == 409;
          _error = e.status == 409
              ? l10n.outlineSaveConflict
              : l10n.outlineSaveFailed('$e');
        });
      }
    } catch (e) {
      if (mounted) setState(() => _error = l10n.outlineSaveFailed('$e'));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  bool get _dirty {
    final p = proposal;
    return p['state'] == 'accepted' &&
        outlineNoteHash(_markdown) != p['note_hash'];
  }

  Future<void> _exportMarkdown() async {
    final title = '${proposal['title']}'.trim();
    final path = await pickSavePath(
      context,
      suggestedName: '${title.isEmpty ? l10n.outlineUntitled : title}.md',
      type: const XTypeGroup(label: 'Markdown', extensions: ['md']),
    );
    if (path == null || !mounted) return;
    try {
      await File(path).writeAsString(_markdown, flush: true);
      if (mounted) setState(() => _notice = l10n.outlineExportSaved(path));
    } catch (e) {
      if (mounted) setState(() => _error = l10n.outlineExportFailed('$e'));
    }
  }

  void _pin() {
    final title = '${proposal['title']}'.trim();
    owner._pinAnswer(
      widget.turn,
      widget.conversation.widget.card.conversationId,
      text: _markdown,
      title: title,
    );
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final stale = owner._staleEvidence(evidence);
    final stats = outlineStats(proposal, stale);
    final numbers = <String, int>{};
    for (final (_, i) in _rows) {
      for (final id in i == null ? const <String>[] : outlineEvidenceIds(i)) {
        numbers.putIfAbsent(id, () => numbers.length + 1);
      }
    }
    final byId = {for (final e in evidence) '${e['id']}': e};
    final rows = _rows.toList();
    final peek = widget.narrow && owner._reviewPeek;
    final header = _header(context, stats, stale);
    return Focus(
      onKeyEvent: (_, e) {
        if (e is! KeyDownEvent) return KeyEventResult.ignored;
        final meta =
            HardwareKeyboard.instance.isControlPressed ||
            HardwareKeyboard.instance.isMetaPressed;
        if (meta &&
            e.logicalKey == LogicalKeyboardKey.keyZ &&
            _editing == null) {
          _undoLast();
          return KeyEventResult.handled;
        }
        if (e.logicalKey == LogicalKeyboardKey.escape && _editing == null) {
          owner._closeReview();
          return KeyEventResult.handled;
        }
        return KeyEventResult.ignored;
      },
      child: _SidebarMotion(
        open: true,
        fromStart:
            !widget.narrow && owner._aiPlacement.edge != StudyDockEdge.left,
        child: GlassSurface(
          depth: GlassDepth.panel,
          radius: Corners.xl,
          padding: const EdgeInsets.fromLTRB(
            Space.lg,
            Space.md,
            Space.sm,
            Space.md,
          ),
          child: peek
              ? header
              : Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    header,
                    const SizedBox(height: Space.sm),
                    Expanded(
                      child: rows.isEmpty
                          ? Center(
                              child: Text(
                                l10n.outlineEmpty,
                                style: tt.bodyMedium?.copyWith(
                                  color: cs.onSurfaceVariant,
                                ),
                              ),
                            )
                          : ReorderableListView(
                              scrollController: _scroll,
                              buildDefaultDragHandles: false,
                              padding: const EdgeInsets.only(
                                right: Space.sm,
                                bottom: Space.md,
                              ),
                              footer: _appendix(context, byId, numbers),
                              onReorderItem: _reorder,
                              children: [
                                for (final (n, r) in rows.indexed)
                                  KeyedSubtree(
                                    key: ValueKey('outline-row-${_rowId(r)}'),
                                    child: r.$2 == null
                                        ? _sectionRow(context, r.$1, n)
                                        : _itemRow(
                                            context,
                                            r.$2!,
                                            n,
                                            byId,
                                            numbers,
                                            stale,
                                          ),
                                  ),
                              ],
                            ),
                    ),
                    _footer(context),
                  ],
                ),
        ),
      ),
    );
  }

  Widget _header(BuildContext context, OutlineStats stats, Set<String> stale) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final title = _editors['title'] ??= TextEditingController(
      text: '${proposal['title']}',
    );
    Widget jump(String label, Color color, bool Function(Map) needs) =>
        ActionChip(
          label: Text(label),
          labelStyle: tt.labelSmall?.copyWith(
            color: color,
            fontWeight: FontWeight.w600,
          ),
          side: BorderSide(color: color.withValues(alpha: .35)),
          visualDensity: VisualDensity.compact,
          materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
          onPressed: () => _jump(needs),
        );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Expanded(
              child: TextField(
                key: const ValueKey('outline-title'),
                controller: title,
                style: tt.titleMedium?.copyWith(fontWeight: FontWeight.w600),
                decoration: const InputDecoration.collapsed(hintText: ''),
                maxLength: 120,
                buildCounter: (
                  _, {
                  required currentLength,
                  required isFocused,
                  maxLength,
                }) => null,
                onTap: () => _undo.add(jsonEncode(proposal)),
                onChanged: (v) => _change(
                  () => proposal['title'] = v.trim().isEmpty
                      ? l10n.outlineUntitled
                      : v.trim(),
                  snapshot: false,
                ),
              ),
            ),
            if (widget.narrow)
              SegmentedButton<bool>(
                showSelectedIcon: false,
                style: const ButtonStyle(visualDensity: VisualDensity.compact),
                segments: [
                  ButtonSegment(
                    value: false,
                    label: Text(l10n.outlinePeekOutline),
                  ),
                  ButtonSegment(
                    value: true,
                    label: Text(l10n.outlinePeekSource),
                  ),
                ],
                selected: {owner._reviewPeek},
                onSelectionChanged: (v) =>
                    owner._update(() => owner._reviewPeek = v.first),
              ),
            IconButton(
              key: const ValueKey('outline-undo'),
              tooltip: l10n.outlineUndo,
              onPressed: _undo.isEmpty ? null : _undoLast,
              icon: const Icon(Icons.undo_rounded, size: 20),
            ),
            IconButton(
              key: const ValueKey('outline-close'),
              tooltip: l10n.outlineClose,
              onPressed: owner._closeReview,
              icon: const Icon(Icons.close_rounded, size: 20),
            ),
          ],
        ),
        if (!(widget.narrow && owner._reviewPeek)) ...[
          const SizedBox(height: Space.xs),
          Row(
            children: [
              Text(
                l10n.outlineReviewed(stats.reviewed, stats.total),
                key: const ValueKey('outline-progress'),
                style: tt.labelMedium?.copyWith(color: cs.onSurfaceVariant),
              ),
              const SizedBox(width: Space.sm),
              Expanded(
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(Corners.full),
                  child: TweenAnimationBuilder<double>(
                    tween: Tween(
                      end: stats.total == 0 ? 0 : stats.reviewed / stats.total,
                    ),
                    duration: Motion.medium2,
                    curve: Motion.emphasized,
                    builder: (_, v, _) => LinearProgressIndicator(
                      value: v,
                      minHeight: 3,
                      backgroundColor: cs.surfaceContainerHighest,
                    ),
                  ),
                ),
              ),
            ],
          ),
          if (stats.stale + stats.marks + stats.unverified > 0) ...[
            const SizedBox(height: Space.sm),
            Wrap(
              spacing: Space.xs,
              runSpacing: Space.xs,
              children: [
                if (stats.marks > 0)
                  jump(
                    l10n.outlineMarksCount(stats.marks),
                    cs.tertiary,
                    (i) => i['review'] == 'needs_interpretation',
                  ),
                if (stats.stale > 0)
                  jump(
                    l10n.outlineStaleCount(stats.stale),
                    cs.error,
                    (i) => outlineEvidenceIds(i).any(stale.contains),
                  ),
                if (stats.unverified > 0)
                  jump(
                    l10n.outlineUnverifiedCount(stats.unverified),
                    _amber(cs),
                    (i) => i['kind'] == 'fact' && !outlineVerified(i, stale),
                  ),
              ],
            ),
          ],
        ],
      ],
    );
  }

  Widget _sectionRow(BuildContext context, Map s, int index) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final id = '${s['id']}';
    final ctl = _editors[id] ??= TextEditingController(text: '${s['heading']}');
    final level = s['level'] is int ? s['level'] as int : 1;
    return Padding(
      key: _rowKeys.putIfAbsent(id, GlobalKey.new),
      padding: EdgeInsets.only(
        top: index == 0 ? 0 : Space.md,
        bottom: Space.xs,
      ),
      child: Row(
        children: [
          ReorderableDragStartListener(
            index: index,
            child: MouseRegion(
              cursor: SystemMouseCursors.grab,
              child: Icon(
                Icons.drag_indicator_rounded,
                size: 18,
                color: cs.outline,
              ),
            ),
          ),
          const SizedBox(width: Space.xs),
          Tooltip(
            message: l10n.outlineLevel,
            child: InkWell(
              borderRadius: BorderRadius.circular(Corners.xs),
              onTap: () => _change(() => s['level'] = level % 3 + 1),
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: Space.xs,
                  vertical: Space.xxs,
                ),
                child: Text(
                  'H$level',
                  style: tt.labelSmall?.copyWith(
                    color: cs.primary,
                    fontFeatures: const [FontFeature.tabularFigures()],
                  ),
                ),
              ),
            ),
          ),
          SizedBox(width: Space.sm * (level - 1) + Space.xs),
          Expanded(
            child: TextField(
              key: ValueKey('outline-heading-$id'),
              controller: ctl,
              style: (level == 1 ? tt.titleSmall : tt.labelLarge)?.copyWith(
                fontWeight: FontWeight.w600,
                color: level == 3 ? cs.onSurfaceVariant : null,
              ),
              decoration: const InputDecoration.collapsed(hintText: ''),
              onTap: () => _undo.add(jsonEncode(proposal)),
              onChanged: (v) =>
                  _change(() => s['heading'] = v, snapshot: false),
            ),
          ),
          MenuAnchor(
            menuChildren: [
              MenuItemButton(
                leadingIcon: const Icon(Icons.add_rounded),
                onPressed: () => _addItem(s),
                child: Text(l10n.outlineAddItem),
              ),
              MenuItemButton(
                leadingIcon: const Icon(Icons.delete_outline_rounded),
                onPressed: sections.length > 1 ? () => _deleteSection(s) : null,
                child: Text(l10n.outlineDeleteSection),
              ),
            ],
            builder: (_, menu, _) => IconButton(
              visualDensity: VisualDensity.compact,
              tooltip: l10n.outlineMore,
              icon: Icon(
                Icons.more_horiz_rounded,
                size: 18,
                color: cs.onSurfaceVariant,
              ),
              onPressed: () => menu.isOpen ? menu.close() : menu.open(),
            ),
          ),
        ],
      ),
    );
  }

  Widget _itemRow(
    BuildContext context,
    Map item,
    int index,
    Map<String, Map> byId,
    Map<String, int> numbers,
    Set<String> stale,
  ) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final id = '${item['id']}';
    final review = '${item['review']}';
    final mark = item['kind'] == 'unresolved_mark';
    final ids = outlineEvidenceIds(item).where(byId.containsKey).toList();
    final broken = ids.where(stale.contains).toList();
    final unverified = item['kind'] == 'fact' && !outlineVerified(item, stale);
    final editing = _editing == id;
    final focus = _rowFocus.putIfAbsent(
      id,
      () => FocusNode(debugLabel: 'outline-$id'),
    );
    final dim = review != 'confirmed' && !mark;
    final tags = [
      if (item['kind'] == 'question') (l10n.outlineTagQuestion, cs.primary),
      if (item['kind'] == 'summary')
        (l10n.outlineKindSummary, cs.onSurfaceVariant),
      if (review == 'to_check') (l10n.outlineTagCheck, _amber(cs)),
      if (unverified && broken.isEmpty) (l10n.outlineTagUnverified, _amber(cs)),
      if (broken.isNotEmpty) (l10n.outlineEvidenceGone, cs.error),
    ];
    final text = editing
        ? CallbackShortcuts(
            bindings: {
              const SingleActivator(LogicalKeyboardKey.enter): () =>
                  _commit(item),
              const SingleActivator(LogicalKeyboardKey.escape): () =>
                  _cancelEdit(item),
            },
            child: TextField(
              key: ValueKey('outline-edit-$id'),
              controller: _editors[id],
              autofocus: true,
              maxLines: null,
              style: tt.bodyMedium,
              decoration: InputDecoration(
                isDense: true,
                filled: true,
                fillColor: cs.surfaceContainerHighest.withValues(alpha: .6),
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: Space.sm,
                  vertical: Space.sm,
                ),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(Corners.sm),
                  borderSide: BorderSide.none,
                ),
                helperText: l10n.outlineEditHint,
                helperStyle: tt.labelSmall?.copyWith(
                  color: cs.onSurfaceVariant,
                ),
              ),
              onTapOutside: (_) => _commit(item),
            ),
          )
        : GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: mark ? null : () => _edit(item),
            child: Text.rich(
              TextSpan(
                children: [
                  ..._outlineSpans('${item['text']}', tt.bodyMedium!, cs),
                  for (final e in ids)
                    WidgetSpan(
                      alignment: PlaceholderAlignment.middle,
                      child: _CitationMark(
                        owner: owner,
                        evidence: byId[e]!,
                        number: numbers[e] ?? 0,
                      ),
                    ),
                ],
              ),
              style: tt.bodyMedium?.copyWith(
                height: 1.45,
                color: dim ? cs.onSurface.withValues(alpha: .78) : cs.onSurface,
                decoration: broken.isNotEmpty
                    ? TextDecoration.lineThrough
                    : null,
                decorationColor: cs.error.withValues(alpha: .5),
              ),
            ),
          );
    final lead = mark
        ? _InkThumb(
            strokes: owner._markInk(item),
            onTap: () {
              final a = item['anchor'];
              if (a is Map) {
                unawaited(
                  owner._followEvidence({
                    'id': id,
                    'kind': 'visual_observation',
                    'status': 'verified',
                    'anchor': a,
                  }),
                );
              }
            },
          )
        : _ReviewTick(review: review, onTap: () => _toggleConfirm(item));
    return Focus(
      focusNode: focus,
      onFocusChange: (on) {
        if (on) owner._reviewFocus = focus;
      },
      onKeyEvent: (_, e) => _rowKey(item, e),
      child: Builder(
        builder: (context) {
          final focused = Focus.of(context).hasFocus;
          return Listener(
            // Pointer down takes the row for keys; a field or mark under it takes focus after.
            onPointerDown: (_) {
              owner._reviewFocus = focus;
              focus.requestFocus();
            },
            child: MouseRegion(
              onEnter: (_) {
                final first = ids.where((e) => !stale.contains(e)).firstOrNull;
                final a = item['anchor'];
                owner._previewEvidence(
                  first != null
                      ? byId[first]
                      : a is Map
                      ? {
                          'id': id,
                          'kind': 'visual_observation',
                          'status': 'verified',
                          'anchor': a,
                        }
                      : null,
                );
              },
              onExit: (_) => owner._previewEvidence(null),
              child: AnimatedContainer(
                key: _rowKeys.putIfAbsent(id, GlobalKey.new),
                duration: Motion.short4,
                curve: Motion.standard,
                margin: const EdgeInsets.only(bottom: Space.xxs),
                padding: const EdgeInsets.fromLTRB(
                  Space.xs,
                  Space.sm,
                  0,
                  Space.sm,
                ),
                decoration: BoxDecoration(
                  color: focused
                      ? cs.primary.withValues(alpha: .06)
                      : Colors.transparent,
                  borderRadius: BorderRadius.circular(Corners.md),
                  border: Border(
                    left: BorderSide(
                      color: review == 'to_check'
                          ? _amber(cs)
                          : Colors.transparent,
                      width: 2,
                    ),
                  ),
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    lead,
                    const SizedBox(width: Space.sm),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          text,
                          if (tags.isNotEmpty && !editing) ...[
                            const SizedBox(height: Space.xs),
                            Wrap(
                              spacing: Space.sm,
                              runSpacing: Space.xxs,
                              crossAxisAlignment: WrapCrossAlignment.center,
                              children: [
                                for (final (label, color) in tags)
                                  Text(
                                    label,
                                    style: tt.labelSmall?.copyWith(
                                      color: color,
                                      fontWeight: FontWeight.w600,
                                      letterSpacing: .2,
                                    ),
                                  ),
                                if (broken.isNotEmpty) ...[
                                  _TinyAction(
                                    label: l10n.outlineKeepAsCheck,
                                    onTap: () => _review(item, 'to_check'),
                                  ),
                                  _TinyAction(
                                    label: l10n.outlineDropEvidence,
                                    onTap: () => _change(
                                      () => item['evidence_ids'] = ids
                                          .where((e) => !broken.contains(e))
                                          .toList(),
                                    ),
                                  ),
                                ],
                              ],
                            ),
                          ],
                          if (mark) _markChoices(context, item),
                        ],
                      ),
                    ),
                    Column(
                      children: [
                        MenuAnchor(
                          menuChildren: [
                            if (!mark) ...[
                              MenuItemButton(
                                leadingIcon: Icon(
                                  review == 'confirmed'
                                      ? Icons.remove_done_rounded
                                      : Icons.done_rounded,
                                ),
                                shortcut: const SingleActivator(
                                  LogicalKeyboardKey.keyA,
                                ),
                                onPressed: () => _toggleConfirm(item),
                                child: Text(
                                  review == 'confirmed'
                                      ? l10n.outlineUnaccept
                                      : l10n.outlineAccept,
                                ),
                              ),
                              MenuItemButton(
                                leadingIcon: const Icon(Icons.flag_outlined),
                                shortcut: const SingleActivator(
                                  LogicalKeyboardKey.keyF,
                                ),
                                onPressed: () => _toggleCheck(item),
                                child: Text(
                                  review == 'to_check'
                                      ? l10n.outlineUncheck
                                      : l10n.outlineToCheck,
                                ),
                              ),
                              MenuItemButton(
                                leadingIcon: const Icon(Icons.edit_outlined),
                                shortcut: const SingleActivator(
                                  LogicalKeyboardKey.enter,
                                ),
                                onPressed: () => _edit(item),
                                child: Text(l10n.outlineEdit),
                              ),
                              MenuItemButton(
                                leadingIcon: const Icon(
                                  Icons.call_split_rounded,
                                ),
                                onPressed: () => _split(item),
                                child: Text(l10n.outlineSplit),
                              ),
                              MenuItemButton(
                                leadingIcon: const Icon(Icons.merge_rounded),
                                onPressed: () => _mergeNext(item),
                                child: Text(l10n.outlineMerge),
                              ),
                            ],
                            MenuItemButton(
                              leadingIcon: const Icon(
                                Icons.arrow_upward_rounded,
                              ),
                              shortcut: const SingleActivator(
                                LogicalKeyboardKey.arrowUp,
                                alt: true,
                              ),
                              onPressed: () => _move(item, -1),
                              child: Text(l10n.outlineMoveUp),
                            ),
                            MenuItemButton(
                              leadingIcon: const Icon(
                                Icons.arrow_downward_rounded,
                              ),
                              shortcut: const SingleActivator(
                                LogicalKeyboardKey.arrowDown,
                                alt: true,
                              ),
                              onPressed: () => _move(item, 1),
                              child: Text(l10n.outlineMoveDown),
                            ),
                            MenuItemButton(
                              leadingIcon: Icon(
                                Icons.delete_outline_rounded,
                                color: cs.error,
                              ),
                              shortcut: const SingleActivator(
                                LogicalKeyboardKey.delete,
                              ),
                              onPressed: () => _delete(item),
                              child: Text(
                                l10n.outlineDelete,
                                style: TextStyle(color: cs.error),
                              ),
                            ),
                          ],
                          builder: (_, menu, _) => IconButton(
                            key: ValueKey('outline-menu-$id'),
                            visualDensity: VisualDensity.compact,
                            tooltip: l10n.outlineMore,
                            icon: Icon(
                              Icons.more_horiz_rounded,
                              size: 18,
                              color: cs.onSurfaceVariant,
                            ),
                            onPressed: () =>
                                menu.isOpen ? menu.close() : menu.open(),
                          ),
                        ),
                        ReorderableDragStartListener(
                          index: index,
                          child: MouseRegion(
                            cursor: SystemMouseCursors.grab,
                            child: Padding(
                              padding: const EdgeInsets.all(Space.xs),
                              child: Icon(
                                Icons.drag_indicator_rounded,
                                size: 16,
                                color: cs.outlineVariant,
                              ),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _markChoices(BuildContext context, Map item) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final choice = '${item['choice'] ?? ''}';
    final id = '${item['id']}';
    final custom = _editors['$id-custom'] ??= TextEditingController(
      text: '${item['choice_text'] ?? ''}',
    );
    return Padding(
      padding: const EdgeInsets.only(top: Space.sm),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (choice.isEmpty)
            Text(
              l10n.outlineMarkHint,
              style: tt.labelSmall?.copyWith(color: cs.tertiary),
            ),
          const SizedBox(height: Space.xs),
          Wrap(
            spacing: Space.xs,
            runSpacing: Space.xs,
            children: [
              for (final c in outlineChoices)
                ChoiceChip(
                  key: ValueKey('outline-choice-$id-$c'),
                  label: Text(outlineChoiceLabel(c)),
                  selected: choice == c,
                  showCheckmark: false,
                  visualDensity: VisualDensity.compact,
                  materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  onSelected: (on) => c == 'custom' && on
                      ? _choose(
                          item,
                          'custom',
                          custom.text.trim().isEmpty
                              ? l10n.outlineChoiceCustom
                              : custom.text.trim(),
                        )
                      : _choose(item, on ? c : null),
                ),
            ],
          ),
          if (choice == 'custom')
            Padding(
              padding: const EdgeInsets.only(top: Space.xs),
              child: TextField(
                key: ValueKey('outline-custom-$id'),
                controller: custom,
                maxLength: 60,
                style: tt.bodySmall,
                decoration: InputDecoration(
                  isDense: true,
                  hintText: l10n.outlineCustomHint,
                  counterText: '',
                ),
                onChanged: (v) => _change(
                  () => item['choice_text'] = v.trim().isEmpty
                      ? l10n.outlineChoiceCustom
                      : v.trim(),
                  snapshot: false,
                ),
              ),
            ),
          if (choice.isNotEmpty)
            _TinyAction(
              label: l10n.outlineMarkUndo,
              onTap: () => _choose(item, null),
            ),
        ],
      ),
    );
  }

  Widget _appendix(
    BuildContext context,
    Map<String, Map> byId,
    Map<String, int> numbers,
  ) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final p = proposal;
    final conflicts = (p['conflicts'] as List? ?? const [])
        .whereType<Map>()
        .toList();
    final questions = [
      for (final q in p['unresolved_questions'] as List? ?? const []) '$q',
    ];
    final missing = [
      for (final q in p['missing_sources'] as List? ?? const []) '$q',
    ];
    if (conflicts.isEmpty && questions.isEmpty && missing.isEmpty) {
      return const SizedBox(height: Space.sm);
    }
    Widget group(String title, List<Widget> children) => Padding(
      padding: const EdgeInsets.only(top: Space.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: tt.labelMedium?.copyWith(
              color: cs.onSurfaceVariant,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: Space.xs),
          ...children,
        ],
      ),
    );
    Widget line(Widget child) => Padding(
      padding: const EdgeInsets.only(bottom: Space.xs),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 7, right: Space.sm),
            child: Icon(Icons.circle, size: 5, color: cs.outline),
          ),
          Expanded(child: child),
        ],
      ),
    );
    return Container(
      margin: const EdgeInsets.only(top: Space.lg),
      padding: const EdgeInsets.all(Space.md),
      decoration: BoxDecoration(
        color: cs.surfaceContainerHighest.withValues(alpha: .45),
        borderRadius: BorderRadius.circular(Corners.lg),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.outlineAppendix,
            style: tt.titleSmall?.copyWith(fontWeight: FontWeight.w600),
          ),
          Text(
            l10n.outlineAppendixNote,
            style: tt.labelSmall?.copyWith(color: cs.onSurfaceVariant),
          ),
          if (conflicts.isNotEmpty)
            group(l10n.outlineConflicts, [
              for (final c in conflicts)
                line(
                  Text.rich(
                    TextSpan(
                      children: [
                        ..._outlineSpans('${c['text']}', tt.bodySmall!, cs),
                        for (final e in [
                          for (final id
                              in c['evidence_ids'] as List? ?? const [])
                            '$id',
                        ].where(byId.containsKey))
                          WidgetSpan(
                            alignment: PlaceholderAlignment.middle,
                            child: _CitationMark(
                              owner: owner,
                              evidence: byId[e]!,
                              number: numbers.putIfAbsent(
                                e,
                                () => numbers.length + 1,
                              ),
                            ),
                          ),
                      ],
                    ),
                    style: tt.bodySmall?.copyWith(height: 1.45),
                  ),
                ),
            ]),
          if (questions.isNotEmpty)
            group(l10n.outlineQuestions, [
              for (final q in questions)
                line(
                  Text.rich(
                    TextSpan(children: _outlineSpans(q, tt.bodySmall!, cs)),
                    style: tt.bodySmall,
                  ),
                ),
            ]),
          if (missing.isNotEmpty)
            group(l10n.outlineUnread, [
              for (final q in missing)
                line(
                  Text(
                    q,
                    style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
                  ),
                ),
            ]),
        ],
      ),
    );
  }

  Widget _footer(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final accepted = proposal['state'] == 'accepted';
    final dirty = _dirty;
    final conversation = widget.conversation.widget.card.conversationId;
    final status =
        _error ??
        _notice ??
        (accepted ? (dirty ? l10n.outlineDirty : l10n.outlineSaved) : null);
    return Padding(
      padding: const EdgeInsets.only(top: Space.sm, right: Space.sm),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AnimatedSize(
            duration: Motion.short4,
            curve: Motion.standard,
            alignment: Alignment.topCenter,
            child: status == null
                ? const SizedBox(width: double.infinity)
                : Padding(
                    padding: const EdgeInsets.only(bottom: Space.sm),
                    child: Row(
                      children: [
                        Icon(
                          _error != null
                              ? Icons.error_outline_rounded
                              : dirty
                              ? Icons.edit_note_rounded
                              : Icons.check_circle_outline_rounded,
                          size: 16,
                          color: _error != null
                              ? cs.error
                              : cs.onSurfaceVariant,
                        ),
                        const SizedBox(width: Space.xs),
                        Expanded(
                          child: Text(
                            status,
                            key: const ValueKey('outline-status'),
                            style: tt.labelSmall?.copyWith(
                              color: _error != null
                                  ? cs.error
                                  : cs.onSurfaceVariant,
                            ),
                          ),
                        ),
                        if (_conflict)
                          TextButton(
                            onPressed: () => _save(copy: true),
                            child: Text(l10n.outlineSaveCopy),
                          )
                        else if (_error != null)
                          TextButton(
                            onPressed: _save,
                            child: Text(l10n.outlineRetry),
                          ),
                      ],
                    ),
                  ),
          ),
          Row(
            children: [
              Expanded(
                child: FilledButton.icon(
                  key: const ValueKey('outline-save'),
                  onPressed: _saving || (accepted && !dirty) ? null : _save,
                  icon: _saving
                      ? const SizedBox.square(
                          dimension: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : Icon(
                          accepted
                              ? Icons.sync_rounded
                              : Icons.note_add_outlined,
                          size: 18,
                        ),
                  label: Text(
                    _saving
                        ? l10n.outlineSaving
                        : accepted
                        ? l10n.outlineUpdate
                        : l10n.outlineSave,
                  ),
                ),
              ),
              const SizedBox(width: Space.xs),
              MenuAnchor(
                menuChildren: [
                  MenuItemButton(
                    leadingIcon: const Icon(Icons.description_outlined),
                    onPressed: _exportMarkdown,
                    child: Text(l10n.outlineExportMarkdown),
                  ),
                  MenuItemButton(
                    key: const ValueKey('outline-export-pdf'),
                    leadingIcon: const Icon(Icons.picture_as_pdf_outlined),
                    onPressed: () => owner._exportOutlinePdf(widget.turn),
                    child: Text(l10n.outlineExportPdf),
                  ),
                ],
                builder: (_, menu, _) => IconButton.outlined(
                  key: const ValueKey('outline-export'),
                  tooltip: l10n.outlineExport,
                  onPressed: () => menu.isOpen ? menu.close() : menu.open(),
                  icon: const Icon(Icons.ios_share_rounded, size: 18),
                ),
              ),
              _PinButton(
                key: const ValueKey('outline-pin'),
                pinned: owner._pinnedCard(widget.turn, conversation, const []),
                onPin: _pin,
                onShow: owner._revealCard,
              ),
            ],
          ),
          if (!widget.narrow)
            Padding(
              padding: const EdgeInsets.only(top: Space.sm),
              child: Text(
                l10n.outlineKeys,
                style: tt.labelSmall?.copyWith(color: cs.outline),
                textAlign: TextAlign.center,
              ),
            ),
        ],
      ),
    );
  }
}

Color _amber(ColorScheme cs) => cs.brightness == Brightness.dark
    ? const Color(0xffe0a85a)
    : const Color(0xff9a5b08);

/// The review state in an item's margin: a ring until accepted, a filled
/// check after, a flag while it waits to be checked.
class _ReviewTick extends StatelessWidget {
  const _ReviewTick({required this.review, required this.onTap});
  final String review;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final (icon, color, tip) = switch (review) {
      'confirmed' => (
        Icons.check_circle_rounded,
        cs.primary,
        l10n.outlineUnaccept,
      ),
      'to_check' => (Icons.flag_circle_rounded, _amber(cs), l10n.outlineAccept),
      _ => (
        Icons.radio_button_unchecked_rounded,
        cs.outline,
        l10n.outlineAccept,
      ),
    };
    return Tooltip(
      message: tip,
      child: InkResponse(
        onTap: onTap,
        radius: 18,
        child: Padding(
          padding: const EdgeInsets.all(Space.xxs),
          child: AnimatedSwitcher(
            duration: Motion.short4,
            switchInCurve: Motion.emphasizedDecelerate,
            transitionBuilder: (child, a) => ScaleTransition(
              scale: Tween(begin: .6, end: 1.0).animate(a),
              child: FadeTransition(opacity: a, child: child),
            ),
            child: Icon(icon, key: ValueKey(review), size: 20, color: color),
          ),
        ),
      ),
    );
  }
}

/// The selected handwriting a mark came from, drawn small; a tap carries
/// the canvas to it.
class _InkThumb extends StatelessWidget {
  const _InkThumb({required this.strokes, required this.onTap});
  final List<StudyStroke> strokes;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Tooltip(
      message: strokes.isEmpty ? l10n.outlineNoInk : l10n.outlineShowInk,
      child: InkWell(
        onTap: strokes.isEmpty ? null : onTap,
        borderRadius: BorderRadius.circular(Corners.sm),
        child: Container(
          width: 44,
          height: 44,
          decoration: BoxDecoration(
            color: const Color(0xfffdfcf8),
            borderRadius: BorderRadius.circular(Corners.sm),
            border: Border.all(color: cs.outlineVariant),
          ),
          child: strokes.isEmpty
              ? Icon(Icons.gesture_rounded, size: 18, color: cs.outline)
              : CustomPaint(painter: _InkThumbPainter(strokes)),
        ),
      ),
    );
  }
}

class _InkThumbPainter extends CustomPainter {
  _InkThumbPainter(this.strokes);
  final List<StudyStroke> strokes;

  @override
  void paint(Canvas canvas, Size size) =>
      paintStudyInk(canvas, size, strokes, inset: 5);

  @override
  bool shouldRepaint(_InkThumbPainter old) => old.strokes != strokes;
}

/// Draws [strokes] fitted into [size], keeping their proportions.
void paintStudyInk(
  Canvas canvas,
  Size size,
  List<StudyStroke> strokes, {
  double inset = 0,
}) {
  if (strokes.isEmpty) return;
  final b = strokes.map((s) => s.bounds).reduce((a, c) => a.expandToInclude(c));
  final scale = math.min(
    (size.width - 2 * inset) / math.max(b.width, 1),
    (size.height - 2 * inset) / math.max(b.height, 1),
  );
  canvas.save();
  canvas.translate(size.width / 2, size.height / 2);
  canvas.scale(scale);
  canvas.translate(-b.center.dx, -b.center.dy);
  for (final s in strokes) {
    s.paint(canvas);
  }
  canvas.restore();
}

class _TinyAction extends StatelessWidget {
  const _TinyAction({required this.label, required this.onTap});
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(Corners.xs),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: Space.xxs,
          vertical: Space.xxs,
        ),
        child: Text(
          label,
          style: Theme.of(context).textTheme.labelSmall?.copyWith(
            color: cs.primary,
            decoration: TextDecoration.underline,
            decorationColor: cs.primary.withValues(alpha: .4),
          ),
        ),
      ),
    );
  }
}

/// An item's text set as the PDF sets it: bold, code and formulas, from
/// the same runs ([outlineInline]); a formula that does not parse stays
/// its TeX source.
List<InlineSpan> _outlineSpans(String text, TextStyle style, ColorScheme cs) {
  final size = style.fontSize ?? 14;
  return [
    for (final (i, line) in text.split('\n').indexed) ...[
      if (i > 0) const TextSpan(text: '\n'),
      for (final r in outlineInline(line))
        switch (r.kind) {
          OutlineRunKind.text => TextSpan(text: r.text),
          OutlineRunKind.bold => TextSpan(
            text: r.text,
            style: const TextStyle(fontWeight: FontWeight.w600),
          ),
          OutlineRunKind.code => TextSpan(
            text: r.text,
            style: TextStyle(
              fontFamily: 'monospace',
              fontSize: size * .92,
              backgroundColor: cs.onSurface.withValues(alpha: .07),
            ),
          ),
          OutlineRunKind.math || OutlineRunKind.displayMath => WidgetSpan(
            alignment: PlaceholderAlignment.baseline,
            baseline: TextBaseline.alphabetic,
            child: Math.tex(
              r.kind == OutlineRunKind.displayMath
                  ? '\\displaystyle ${r.text}'
                  : r.text,
              // Inline math a tenth above text size, as in chat and notes.
              textStyle: TextStyle(
                fontSize: size * 1.1,
                color: style.color ?? cs.onSurface,
              ),
              parseOptions: const ParseOptions(strict: Strict.ignore),
              onError: (_, _) =>
                  Text(r.text, style: style.copyWith(fontFamily: 'monospace')),
            ),
          ),
        },
    ],
  ];
}
