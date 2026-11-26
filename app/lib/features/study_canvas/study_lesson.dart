part of 'study_canvas.dart';

extension _StudyLesson on StudyCanvasState {
  Future<Map<String, dynamic>> _lessonRequest(
    String action, [
    Map<String, dynamic> data = const {},
  ]) async {
    final id = Uri.encodeComponent(widget.ctl.lectureId);
    final raw = await widget.api.post('/agent/lessons/$id/study', {
      'action': action,
      ...data,
    }, const Duration(minutes: 6));
    return Map<String, dynamic>.from(raw as Map);
  }

  Future<void> _openLesson({bool reveal = true}) async {
    final d = document;
    if (widget.standalone || _lessonBusy || d == null) return;
    _update(() => _lessonBusy = true);
    if (reveal && _lessonState.isEmpty) _toast(l10n.lessonLoading);
    try {
      if (!reveal) {
        // The canvas cache remains readable offline. Refresh real note bodies
        // even when the teaching plugin has subsequently been disabled.
        for (final c in d.cards.where((c) => c.lessonNote).toList()) {
          final note =
              await widget.api.get('/api/notes/${c.documentId}') as Map;
          if (!mounted || document != d) return;
          if (note['lecture_id'] == widget.ctl.lectureId &&
              note['body'] is String &&
              c.text != note['body']) {
            _rebaseMarkdownNotes(c, note['body'] as String);
            c.text = note['body'] as String;
            d.changed();
          }
        }
      }
      var result = await _lessonRequest(reveal ? 'open' : 'status');
      if (!mounted || document != d) return;
      if (result['branches'] is List) {
        if (!reveal) return;
        final branch = await showDialog<String>(
          context: context,
          builder: (ctx) => SimpleDialog(
            title: Text(l10n.lessonChooseProgress),
            children: [
              for (final b in result['branches'] as List)
                SimpleDialogOption(
                  onPressed: () => Navigator.pop(ctx, b['checkpoint']),
                  child: Text(
                    '${b['unit']} · ${b['feedback'] ?? b['stage']} · ${(b['checkpoint'] as String).substring(0, 8)}',
                  ),
                ),
            ],
          ),
        );
        if (!mounted || branch == null || document != d) return;
        result = await _lessonRequest('open', {'checkpoint': branch});
      }
      if (!mounted || document != d) return;
      _update(() => _lessonState = result);
      if (result['note'] case final Map note)
        _showLessonNote(note, reveal: reveal);
    } catch (e) {
      if (mounted && reveal) _toast('$e');
    } finally {
      if (mounted) _update(() => _lessonBusy = false);
    }
  }

  void _showLessonNote(Map note, {bool reveal = true}) {
    final d = document!;
    if (note['lecture_id'] != widget.ctl.lectureId || note['body'] is! String)
      throw FormatException(l10n.studyEvidenceStale);
    var card = d.cards
        .where((c) => c.lessonNote && c.documentId == note['id'])
        .firstOrNull;
    if (card == null) {
      if (d.cards.length >= 2000 || !d.editable(d.activeLayer)) {
        _toast(d.cards.length >= 2000 ? l10n.studyCardLimit : l10n.studyLocked);
        return;
      }
      d.checkpoint();
      card = StudyCard(
        id: studyId(),
        type: 'text',
        generation: 'lesson-note',
        documentId: note['id'] as String,
        layer: d.activeLayer,
        rect: _placeNear(null, const Size(540, 500)),
        title: note['title'] as String,
        text: note['body'] as String,
      );
      d.cards.add(card);
      d.changed();
    } else if (card.text != note['body']) {
      _rebaseMarkdownNotes(card, note['body'] as String);
      card.text = note['body'] as String;
      d.changed();
    }
    if (reveal) _revealCard(card);
  }

  Future<void> _lessonAction(String action, Map<String, dynamic> data) {
    final d = document;
    return _lessonWrites = _lessonWrites.then((_) async {
      if (mounted && document == d) await _commitLessonAction(action, data);
    });
  }

  Future<void> _commitLessonAction(
    String action,
    Map<String, dynamic> data,
  ) async {
    final visible = action != 'position';
    if (_lessonBusy ||
        (!visible &&
            mapEquals(
              _lessonState['bookmark'] as Map?,
              data['bookmark'] as Map?,
            )))
      return;
    final d = document;
    if (visible) _update(() => _lessonBusy = true);
    try {
      final result = await _lessonRequest(action, {
        'checkpoint': _lessonState['checkpoint'],
        ...data,
      });
      if (!mounted || document != d) return;
      _update(() => _lessonState = result);
      if (result['suggestion'] case final Map note) _showLessonNote(note);
    } catch (e) {
      if (mounted) _toast('$e');
    } finally {
      if (mounted && visible) _update(() => _lessonBusy = false);
    }
  }

  Widget _lessonReader(StudyCard card, {Widget? readingPage}) {
    final active = _lessonState['noteId'] == card.documentId;
    return _LessonReader(
      key: ValueKey(card.id),
      body: readingPage ?? _markdownPassage(card, card.text),
      fullPage: readingPage != null,
      state: active ? _lessonState : const {},
      busy: _lessonBusy,
      action: _lessonAction,
      open: () => _openLesson(),
    );
  }
}

class _LessonReader extends StatefulWidget {
  const _LessonReader({
    super.key,
    required this.body,
    required this.fullPage,
    required this.state,
    required this.busy,
    required this.action,
    required this.open,
  });
  final Widget body;
  final bool fullPage;
  final Map<String, dynamic> state;
  final bool busy;
  final Future<void> Function(String, Map<String, dynamic>) action;
  final VoidCallback open;
  @override
  State<_LessonReader> createState() => _LessonReaderState();
}

class _LessonReaderState extends State<_LessonReader> {
  final scroll = ScrollController();
  final bodyKey = GlobalKey();
  final draft = TextEditingController();
  String? submission, previous;
  bool restored = false;

  Iterable<RenderParagraph> _paragraphs(RenderObject node) sync* {
    if (node is RenderParagraph) yield node;
    final children = <RenderObject>[];
    node.visitChildren(children.add);
    for (final child in children) {
      yield* _paragraphs(child);
    }
  }

  void _restore() {
    if (restored || !scroll.hasClients || widget.state.isEmpty) return;
    restored = true;
    final mark = widget.state['bookmark'];
    if (mark is! Map) return;
    final root = bodyKey.currentContext?.findRenderObject();
    final max = scroll.position.maxScrollExtent;
    var offset = max * (mark['ratio'] as num).toDouble();
    if (root is RenderBox && (mark['quote'] as String).isNotEmpty) {
      for (final p in _paragraphs(root)) {
        if (p.text.toPlainText().startsWith(mark['quote'] as String)) {
          offset =
              p.localToGlobal(Offset.zero, ancestor: root).dy +
              p.size.height * (mark['fraction'] as num);
          break;
        }
      }
    }
    scroll.jumpTo(offset.clamp(0.0, max));
  }

  bool _scrollEnd(ScrollEndNotification event) {
    if (event.depth != 0 ||
        !restored ||
        widget.busy ||
        widget.state.isEmpty ||
        !scroll.hasClients)
      return false;
    final pos = scroll.position;
    if (pos.maxScrollExtent <= 0) return false;
    final root = bodyKey.currentContext?.findRenderObject();
    var quote = '';
    var fraction = 0.0;
    if (root is RenderBox) {
      for (final p in _paragraphs(root)) {
        final y = p.localToGlobal(Offset.zero, ancestor: root).dy;
        final text = p.text.toPlainText();
        if (y + p.size.height > pos.pixels && text.trim().isNotEmpty) {
          quote = String.fromCharCodes(text.runes.take(70));
          fraction = ((pos.pixels - y) / math.max(1.0, p.size.height))
              .clamp(0, 1)
              .toDouble();
          break;
        }
      }
    }
    final bookmark = {
      'ratio': (pos.pixels / pos.maxScrollExtent).clamp(0, 1),
      'quote': quote,
      'fraction': fraction,
    };
    if (jsonEncode(widget.state['bookmark']) != jsonEncode(bookmark)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && !widget.busy)
          unawaited(widget.action('position', {'bookmark': bookmark}));
      });
    }
    return false;
  }

  Future<void> _answer(Map pending) async {
    final answer = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(pending['question'] as String),
        content: TextField(
          controller: draft,
          autofocus: true,
          minLines: 2,
          maxLines: 5,
          maxLength: 4000,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () {
              if (draft.text.trim().isNotEmpty)
                Navigator.pop(ctx, draft.text.trim());
            },
            child: Text(l10n.lessonSubmit),
          ),
        ],
      ),
    );
    if (!mounted ||
        answer == null ||
        widget.state['pending']?['id'] != pending['id'])
      return;
    if (previous != answer) {
      submission = studyId();
      previous = answer;
    }
    await widget.action('answer', {
      'submissionId': submission,
      'questionId': pending['id'],
      'answer': answer,
    });
  }

  @override
  void didUpdateWidget(_LessonReader old) {
    super.didUpdateWidget(old);
    if (widget.state['pending']?['id'] != old.state['pending']?['id']) {
      draft.clear();
      previous = null;
      submission = null;
    }
  }

  @override
  void dispose() {
    scroll.dispose();
    draft.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _restore();
    });
    final pending = widget.state['pending'];
    return Column(
      children: [
        Expanded(
          child: widget.fullPage ? widget.body : NotificationListener<ScrollEndNotification>(
            onNotification: _scrollEnd,
            child: SingleChildScrollView(
              controller: scroll,
              child: Padding(
                key: bodyKey,
                padding: const EdgeInsets.all(12),
                child: widget.body,
              ),
            ),
          ),
        ),
        if (pending is Map && pending['hints'] is List && (pending['hints'] as List).isNotEmpty)
          Padding(padding:const EdgeInsets.symmetric(horizontal:8),child:Text((pending['hints'] as List).last.toString())),
        if (widget.state['feedback'] case final String feedback)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8),
            child: Text(feedback, maxLines: 3, overflow: TextOverflow.ellipsis),
          ),
        Row(
          children: [
            if (widget.busy)
              const Padding(
                padding: EdgeInsets.all(8),
                child: SizedBox.square(
                  dimension: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            if (widget.state.isEmpty)
              TextButton(
                onPressed: widget.busy ? null : widget.open,
                child: Text(l10n.lessonStudyStart),
              ),
            if (pending is Map && widget.state['recommendedNext']!=true)
              Flexible(
                child: TextButton(
                  onPressed: widget.busy ? null : () => _answer(pending),
                  child: Text(l10n.lessonPractice),
                ),
              ),
            if (pending is Map && widget.state['skillTracking']==true) ...[
              if (widget.state['recommendedNext']!=true && ((pending['hints'] as List?)?.length??0)<2)
                IconButton(tooltip:l10n.lessonShowHint,icon:const Icon(Icons.lightbulb_outline),onPressed:widget.busy?null:()=>widget.action('hint',{'questionId':pending['id']})),
              TextButton(onPressed:widget.busy?null:()=>widget.action('skip',{'questionId':pending['id']}),child:Text(l10n.lessonNextTopic)),
            ],
            const Spacer(),
            if (widget.state.isNotEmpty)
              IconButton(
                tooltip: l10n.lessonRevision,
                icon: const Icon(Icons.rate_review_outlined),
                onPressed: widget.busy
                    ? null
                    : () => widget.action('revise', {}),
              ),
          ],
        ),
      ],
    );
  }
}
