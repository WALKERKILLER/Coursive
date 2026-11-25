part of 'study_canvas.dart';

/// Native selection offsets address rendered text. Only a verified, literal
/// source match is an exact UTF-16 range the daemon may slice.
class _MarkdownPassage {
  const _MarkdownPassage(
    this.quote,
    this.locator,
    this.fingerprint,
    this.range,
  );
  final String quote, locator, fingerprint;
  final TextRange? range;

  bool matches(StudyAnchor anchor) {
    if (anchor.quote != quote) return false;
    if (anchor.fingerprint == fingerprint) return anchor.id == locator;
    if (anchor.fingerprint == 'source-$fingerprint' && range != null) {
      return anchor.start == range!.start && anchor.end == range!.end;
    }
    // An edit before a repeated word can make an old offset refer to another
    // occurrence. Keep its note on the canvas without guessing a new target.
    return false;
  }
}

typedef _MarkdownMark = ({StudyAnchor anchor, bool highlight});

extension _StudyMarkdownEditing on StudyCanvasState {
  List<_MarkdownMark> _marksFor(StudyCard card, int block) {
    final d = document!;
    if (!identical(_markdownMarksDocument, d) || _markdownMarksEdit != d.edit) {
      _markdownMarksDocument = d;
      _markdownMarksEdit = d.edit;
      _markdownMarks = {};
      for (final linked in d.cards) {
        if (!linked.passageLinked || !d.visible(linked.layer)) continue;
        for (final anchor in linked.sources) {
          final parts = anchor.id.split('-');
          if (parts.length != 4 ||
              parts.first != 'md' ||
              anchor.quote.isEmpty) {
            continue;
          }
          final section = int.tryParse(parts[1]);
          if (section == null) continue;
          (_markdownMarks[anchor.objectId] ??= {})
              .putIfAbsent(section - 1, () => [])
              .add((anchor: anchor, highlight: linked.passageNote));
        }
      }
    }
    return _markdownMarks[card.id]?[block] ?? const [];
  }

  Widget _markdownPassage(StudyCard card, String body, {int block = -1}) {
    final content = CoursiveMarkdown(
      body,
      lectureId: widget.standalone ? '' : widget.ctl.lectureId,
      onRef: widget.standalone ? null : (ref) => _openMarkdownReference(ref),
      linkDecorationStyle: TextDecorationStyle.dashed,
      baseDir: card.originalPath.isEmpty
          ? ''
          : filePath.posix.dirname(card.originalPath),
      style: TextStyle(
        fontSize: 16,
        height: 1.55,
        color: StudyChrome.of(context).ink,
      ),
    );
    final selectable =
        _tool == StudyTool.select && !_pdfPinching && !_snapshotting;
    final marks = _snapshotting
        ? const <_MarkdownMark>[]
        : _marksFor(card, block);
    if (!selectable && marks.isEmpty) return content;
    return _SelectableMarkdownPassage(
      key: ValueKey('markdown/${card.id}/$block'),
      card: card,
      block: block,
      body: body,
      selectable: selectable,
      marks: marks,
      findNote: (p) => document!.cards
          .where(
            (c) =>
                c.generation == 'markdown-annotation' &&
                document!.visible(c.layer) &&
                c.sources.any((a) => a.objectId == card.id && p.matches(a)),
          )
          .firstOrNull,
      edit: card.markdownReader ? (p) => _text(card, p.range) : null,
      annotate: (p, note) => _annotatePassage(card, p, note),
      linkedAsides: (p) => document!.cards.where(
        (c) =>
            c.passageAside &&
            document!.visible(c.layer) &&
            c.sources.any((a) => a.objectId == card.id && p.matches(a)),
      ),
      reveal: _revealCard,
      evidence: _evidenceFocus,
      onPassageMissing: () => _toast(l10n.studyEvidenceStale),
      aside: widget.standalone
          ? null
          : (p) {
              final anchor = _passageAnchor(card, p);
              _startConversation(
                beside: card,
                passageAnchor: anchor,
                sourceQuery: _passageQuery(anchor),
              );
            },
      child: content,
    );
  }

  void _openMarkdownReference(CoursiveRef ref) {
    final lectureId = widget.ctl.lectureId;
    if (ref is TimeRef &&
        (ref.lectureId.isEmpty || ref.lectureId == lectureId)) {
      widget.ctl.seekTo(ref.seconds * 1000);
      final d = document!;
      final video = d.cards
          .where((c) => c.type == 'lecture' && d.visible(c.layer))
          .firstOrNull;
      if (video != null) {
        _revealCard(video);
      } else {
        widget.onClassic?.call();
      }
      return;
    }
    openRef(context, ref, lectureId: lectureId);
  }

  // Keep exact anchors in the unchanged prefix and suffix of a source edit.
  // An annotation whose passage was replaced keeps its original quotation.
  void _rebaseMarkdownNotes(StudyCard card, String next) {
    final old = card.text;
    if (!card.markdownReader || old == next) return;
    final notes = document!.cards
        .where(
          (c) =>
              c.passageLinked &&
              c.sources.length == 1 &&
              c.sources.single.objectId == card.id &&
              c.sources.single.end > c.sources.single.start,
        )
        .toList();
    if (notes.isEmpty) return;
    int prefix = 0, suffix = 0;
    final shorter = math.min(old.length, next.length);
    while (prefix < shorter &&
        old.codeUnitAt(prefix) == next.codeUnitAt(prefix)) {
      prefix++;
    }
    while (suffix < shorter - prefix &&
        old.codeUnitAt(old.length - suffix - 1) ==
            next.codeUnitAt(next.length - suffix - 1)) {
      suffix++;
    }
    final oldHash = outlineNoteHash(old), nextHash = outlineNoteHash(next);
    for (final note in notes) {
      final a = note.sources.single;
      if (a.fingerprint != oldHash && a.fingerprint != 'source-$oldHash') {
        continue;
      }
      if (a.end > old.length || old.substring(a.start, a.end) != a.quote) {
        continue;
      }
      final int delta;
      if (a.end <= prefix) {
        delta = 0;
      } else if (a.start >= old.length - suffix) {
        delta = next.length - old.length;
      } else {
        continue;
      }
      note.sources = [
        StudyAnchor.fromJson({
          ...a.json(),
          'start': a.start + delta,
          'end': a.end + delta,
          'fingerprint': 'source-$nextHash',
        }),
      ];
    }
  }

  StudyAnchor _passageAnchor(StudyCard source, _MarkdownPassage passage) =>
      StudyAnchor(
        id: passage.locator,
        objectId: source.id,
        workspaceId: widget.ctl.lectureId,
        kind: source.type,
        start: passage.range?.start ?? 0,
        end: passage.range?.end ?? 0,
        fingerprint: passage.fingerprint,
        quote: passage.quote,
      );

  Map<String, dynamic> _passageQuery(StudyAnchor a) => {
    'objects': [a.objectId],
    'include_video': false,
    'selection': {
      if (a.end > a.start) ...{
        'object_id': a.objectId,
        'start': a.start,
        'end': a.end,
      },
    },
  };

  void _annotatePassage(
    StudyCard source,
    _MarkdownPassage passage,
    StudyCard? existing,
  ) {
    final d = document!;
    if (!d.editable(existing?.layer ?? source.layer)) {
      _toast(l10n.studyLocked);
      return;
    }
    if (existing == null && d.cards.length >= 2000) {
      _toast(l10n.studyCardLimit);
      return;
    }
    final note =
        existing ??
        StudyCard(
          id: studyId(),
          type: 'text',
          layer: source.layer,
          rect: _placeNear(source.visualBounds, const Size(320, 180)),
          title: passage.quote.characters.take(60).toString(),
          generation: 'markdown-annotation',
          sources: [_passageAnchor(source, passage)],
        );
    if (existing == null) {
      d.checkpoint();
      d.cards.add(note);
      d.changed();
    }
    _passageEditCheckpoint = null;
    _editingPassage = note.id;
    _revealCard(note);
  }

  Widget _passageEditor(StudyCard card) => Padding(
    padding: const EdgeInsets.all(Space.md),
    child: TextFormField(
      key: ObjectKey(card),
      initialValue: card.text,
      autofocus: true,
      expands: true,
      maxLines: null,
      maxLength: 2000,
      textAlignVertical: TextAlignVertical.top,
      decoration: InputDecoration.collapsed(hintText: l10n.studyAnnotationHint),
      onChanged: (value) {
        final d = document!;
        if (!d.editable(card.layer) || !d.cards.contains(card)) return;
        if (_passageEditCheckpoint != card.id) {
          d.checkpoint();
          _passageEditCheckpoint = card.id;
        }
        card.text = value;
        d.changed();
      },
    ),
  );
}

class _SelectableMarkdownPassage extends StatefulWidget {
  const _SelectableMarkdownPassage({
    super.key,
    required this.card,
    required this.block,
    required this.body,
    required this.selectable,
    required this.marks,
    required this.child,
    required this.findNote,
    required this.linkedAsides,
    required this.reveal,
    required this.evidence,
    required this.onPassageMissing,
    required this.annotate,
    required this.edit,
    required this.aside,
  });
  final StudyCard card;
  final String body;
  final int block;
  final bool selectable;
  final List<_MarkdownMark> marks;
  final Widget child;
  final Iterable<StudyCard> Function(_MarkdownPassage) linkedAsides;
  final ValueChanged<StudyCard> reveal;
  final ValueListenable<Map?> evidence;
  final VoidCallback onPassageMissing;
  final StudyCard? Function(_MarkdownPassage) findNote;
  final void Function(_MarkdownPassage, StudyCard?) annotate;
  final ValueChanged<_MarkdownPassage>? aside, edit;
  @override
  State<_SelectableMarkdownPassage> createState() =>
      _SelectableMarkdownPassageState();
}

class _SelectableMarkdownPassageState
    extends State<_SelectableMarkdownPassage> {
  final _range = SelectionListenerNotifier();
  final _area = GlobalKey<SelectionAreaState>();
  final _desktopMenu = ContextMenuController();
  final _passagePaint = GlobalKey();
  final _passageBoxes = ValueNotifier<List<Rect>>(const []);
  Timer? _clearPassage;
  String _quote = '';
  String? _fingerprint;
  _MarkdownPassage? _lastPassage;
  @override
  void initState() {
    super.initState();
    widget.evidence.addListener(_scrollToCitation);
    _scrollToCitation();
  }

  List<({RenderParagraph paragraph, int start, String text})> _fragments(
    RenderObject root,
  ) {
    final fragments = <({RenderParagraph paragraph, int start, String text})>[];
    void collect(RenderObject node) {
      if (node is RenderParagraph && node.registrar != null) {
        final text = node.text.toPlainText(includeSemanticsLabels: false);
        var offset = 0;
        for (final piece in text.split('\uFFFC')) {
          if (piece.isNotEmpty) {
            fragments.add((paragraph: node, start: offset, text: piece));
          }
          offset += piece.length + 1;
        }
      }
      node.visitChildren(collect);
    }

    collect(root);
    return fragments;
  }

  Iterable<({RenderParagraph paragraph, Rect box})> _rangeBoxes(
    TextRange range,
    List<({RenderParagraph paragraph, int start, String text})> fragments, {
    ui.BoxHeightStyle boxHeightStyle = ui.BoxHeightStyle.tight,
  }) sync* {
    var offset = 0;
    for (final fragment in fragments) {
      final from = math.max(0, range.start - offset);
      final to = math.min(fragment.text.length, range.end - offset);
      offset += fragment.text.length;
      if (from < to) {
        for (final box in fragment.paragraph.getBoxesForSelection(
          TextSelection(
            baseOffset: fragment.start + from,
            extentOffset: fragment.start + to,
          ),
          boxHeightStyle: boxHeightStyle,
        )) {
          yield (paragraph: fragment.paragraph, box: box.toRect());
        }
      }
      if (offset >= range.end) break;
    }
  }

  // Translate saved anchors through the current text layout, so line breaks
  // and resizing only change the marks' geometry.
  Iterable<({Rect box, bool highlight})> _markBoxes() sync* {
    if (widget.marks.isEmpty) return;
    final root = _passagePaint.currentContext?.findRenderObject();
    if (root == null) return;
    final fragments = _fragments(root);
    final rendered = fragments.map((f) => f.text).join();
    for (final mark in widget.marks) {
      final range = _renderedRange(mark.anchor, rendered);
      if (range == null) continue;
      final boxes =
          _rangeBoxes(
            range,
            fragments,
            boxHeightStyle: mark.highlight
                ? ui.BoxHeightStyle.tight
                : ui.BoxHeightStyle.max,
          ).map(
            (part) => MatrixUtils.transformRect(
              part.paragraph.getTransformTo(root),
              part.box,
            ),
          );
      for (final box in mark.highlight ? boxes : _joinUnderlineRuns(boxes)) {
        yield (box: box, highlight: mark.highlight);
      }
    }
  }

  // Continue the dash pattern across adjacent font runs on the same line.
  Iterable<Rect> _joinUnderlineRuns(Iterable<Rect> boxes) sync* {
    final ordered = boxes.toList()
      ..sort((a, b) {
        final byLine = a.top.compareTo(b.top);
        return byLine == 0 ? a.left.compareTo(b.left) : byLine;
      });
    Rect? line;
    for (final box in ordered) {
      if (line != null) {
        if (line.top == box.top &&
            line.bottom == box.bottom &&
            box.left <= line.right + .01) {
          line = line.expandToInclude(box);
          continue;
        }
        yield line;
      }
      line = box;
    }
    if (line != null) yield line;
  }

  // The selection offsets count selectable characters, which exclude the
  // placeholders Flutter inserts for inline widgets such as formulas.
  void _scrollToCitation() {
    _clearPassage?.cancel();
    _passageBoxes.value = const [];
    final evidence = widget.evidence.value;
    if (evidence?['anchor'] is! Map) return;
    final anchor = StudyAnchor.fromJson(evidence!['anchor'] as Map);
    if (anchor.objectId != widget.card.id ||
        !anchor.id.startsWith('md-${widget.block + 1}-') ||
        anchor.quote.isEmpty)
      return;

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted ||
          !widget.selectable ||
          !identical(widget.evidence.value, evidence)) {
        return;
      }
      final paintContext = _passagePaint.currentContext;
      final root = paintContext?.findRenderObject();
      if (paintContext == null || root == null) return;
      final fragments = _fragments(root);
      final rendered = fragments.map((f) => f.text).join();
      final range = _renderedRange(anchor, rendered);
      if (range == null) {
        widget.onPassageMissing();
        return;
      }
      final boxes = <Rect>[];
      RenderParagraph? targetParagraph;
      Rect? targetLine;
      for (final (:paragraph, :box) in _rangeBoxes(range, fragments)) {
        targetParagraph ??= paragraph;
        targetLine ??= box;
        boxes.add(
          MatrixUtils.transformRect(
            paragraph.getTransformTo(root),
            box,
          ).inflate(2),
        );
      }
      if (targetParagraph == null || targetLine == null) return;
      _desktopMenu.remove();
      _area.currentState?.selectableRegion.clearSelection();
      final scroll = Scrollable.maybeOf(paintContext);
      final viewport = RenderAbstractViewport.maybeOf(root);
      if (scroll != null &&
          axisDirectionToAxis(scroll.widget.axisDirection) == Axis.vertical &&
          scroll.position.hasContentDimensions &&
          viewport != null) {
        // A horizontally scrolling table may need to reveal the cell too.
        if (RenderAbstractViewport.maybeOf(targetParagraph) != viewport) {
          targetParagraph.showOnScreen(rect: targetLine);
        }
        final position = scroll.position;
        final passage = boxes.reduce((a, b) => a.expandToInclude(b));
        final target = passage.height > position.viewportDimension
            ? boxes.first
            : passage;
        final offset = viewport
            .getOffsetToReveal(root, .5, rect: target)
            .offset;
        position.jumpTo(
          offset.clamp(position.minScrollExtent, position.maxScrollExtent),
        );
      } else {
        targetParagraph.showOnScreen(rect: targetLine);
      }
      _passageBoxes.value = boxes;
      _clearPassage = Timer(const Duration(milliseconds: 2400), () {
        _passageBoxes.value = const [];
      });
    });
  }

  TextRange? _renderedRange(StudyAnchor anchor, String rendered) {
    final hash = _fingerprint ??= outlineNoteHash(widget.body);
    if (anchor.fingerprint == hash) {
      final parts = anchor.id.split('-');
      final start = int.tryParse(parts.elementAtOrNull(2) ?? '');
      final end = int.tryParse(parts.elementAtOrNull(3) ?? '');
      if (start != null &&
          end != null &&
          start >= 0 &&
          end >= start &&
          end <= rendered.length &&
          rendered.substring(start, end) == anchor.quote) {
        return TextRange(start: start, end: end);
      }
    }
    // After an edit the rendered offset may have shifted. Resolve only a
    // candidate that still maps to the verified raw source offsets.
    if (anchor.quote.isEmpty ||
        anchor.end <= anchor.start ||
        (anchor.fingerprint != hash && anchor.fingerprint != 'source-$hash')) {
      return null;
    }
    final base = _sourceBase;
    for (
      var start = rendered.indexOf(anchor.quote);
      start >= 0;
      start = rendered.indexOf(anchor.quote, start + 1)
    ) {
      final end = start + anchor.quote.length;
      final exact = _sourceRange(start, end, anchor.quote);
      if (exact != null &&
          exact.start + base == anchor.start &&
          exact.end + base == anchor.end) {
        return TextRange(start: start, end: end);
      }
    }
    return null;
  }

  int get _sourceBase => widget.block < 0
      ? 0
      : widget.card.blocks
            .take(widget.block)
            .fold<int>(0, (n, b) => n + b.plainText.length + 1);

  @override
  void didUpdateWidget(_SelectableMarkdownPassage old) {
    super.didUpdateWidget(old);
    if (old.body != widget.body) {
      _fingerprint = null;
      _lastPassage = null;
      _quote = '';
      _passageBoxes.value = const [];
      _desktopMenu.remove();
    }
    if (old.selectable && !widget.selectable) {
      _desktopMenu.remove();
      _area.currentState?.selectableRegion.clearSelection();
    }
  }

  @override
  void dispose() {
    _desktopMenu.remove();
    widget.evidence.removeListener(_scrollToCitation);
    _range.dispose();
    _clearPassage?.cancel();
    _passageBoxes.dispose();
    super.dispose();
  }

  _MarkdownPassage? _current() {
    final r = _range.registered ? _range.selection.range : null;
    if (r == null || _quote.trim().isEmpty || _quote.length > 2000) return null;
    final start = math.min(r.startOffset, r.endOffset),
        end = math.max(r.startOffset, r.endOffset);
    final locator = 'md-${widget.block + 1}-$start-$end';
    final previous = _lastPassage;
    if (previous != null &&
        previous.quote == _quote &&
        previous.locator == locator &&
        widget.block < 0) {
      return previous;
    }
    final body = widget.body;
    final localRange = _sourceRange(start, end, _quote);
    TextRange? exact;
    if (localRange != null) {
      final base = _sourceBase;
      exact = TextRange(
        start: base + localRange.start,
        end: base + localRange.end,
      );
    }
    return _lastPassage = _MarkdownPassage(
      _quote,
      locator,
      _fingerprint ??= outlineNoteHash(body),
      exact,
    );
  }

  // Match the renderer's actual text spans to the Markdown in both directions.
  // Generated labels, reordered widgets and ambiguous matches degrade to a
  // quotation bound to the card, never an offset into an unrelated occurrence.
  TextRange? _sourceRange(int start, int end, String quote) {
    final runs = <String>[];
    void collect(RenderObject node) {
      if (node is RenderParagraph && node.registrar != null) {
        node.text.visitChildren((span) {
          if (span is TextSpan && span.text?.isNotEmpty == true) {
            runs.add(span.text!);
          }
          return true;
        });
      }
      node.visitChildren(collect);
    }

    final root = _area.currentContext?.findRenderObject();
    if (root == null) return null;
    collect(root);
    final display = runs.join();
    if (start < 0 ||
        end > display.length ||
        display.substring(start, end) != quote) {
      return null;
    }
    final raw = widget.body;
    final earliest = <int>[];
    var consumed = 0;
    for (final run in runs) {
      if (run.trim().isEmpty) {
        earliest.add(-1);
        continue;
      }
      final at = raw.indexOf(run, consumed);
      if (at < 0) return null;
      earliest.add(at);
      consumed = at + run.length;
    }
    var limit = raw.length, visibleEnd = display.length;
    int? rawStart, rawEnd;
    for (var i = runs.length - 1; i >= 0; i--) {
      final run = runs[i], visibleStart = visibleEnd - run.length;
      if (earliest[i] < 0) {
        visibleEnd = visibleStart;
        continue;
      }
      if (limit < run.length) return null;
      final at = raw.lastIndexOf(run, limit - run.length);
      if (at < 0) return null;
      if (start < visibleEnd && end > visibleStart) {
        if (earliest[i] != at) return null;
        if (start >= visibleStart) rawStart = at + start - visibleStart;
        if (end <= visibleEnd) rawEnd = at + end - visibleStart;
      }
      visibleEnd = visibleStart;
      limit = at;
      if (rawStart != null && rawEnd != null) break;
    }
    if (rawStart == null ||
        rawEnd == null ||
        raw.substring(rawStart, rawEnd) != quote) {
      return null;
    }
    return TextRange(start: rawStart, end: rawEnd);
  }

  Widget _menu(
    SelectableRegionState region,
    TextSelectionToolbarAnchors anchors,
  ) {
    final p = _current();
    final note = p == null ? null : widget.findNote(p);
    void dismiss() {
      _desktopMenu.remove();
      region.hideToolbar();
    }

    return TapRegion(
      onTapOutside: (_) => _desktopMenu.remove(),
      child: AdaptiveTextSelectionToolbar.buttonItems(
        anchors: anchors,
        buttonItems: [
          if (p != null)
            ContextMenuButtonItem(
              label: note == null
                  ? l10n.studyAddAnnotation
                  : '${l10n.studyEditAnnotation} · ${note.text.characters.take(20)}',
              onPressed: () {
                dismiss();
                widget.annotate(p, note);
              },
            ),
          if (p != null && widget.aside != null)
            ContextMenuButtonItem(
              label: l10n.studyHandoutAside,
              onPressed: () {
                dismiss();
                widget.aside!(p);
              },
            ),
          if (p != null)
            for (final (index, card) in widget.linkedAsides(p).indexed)
              ContextMenuButtonItem(
                label: l10n.studyOpenPassageAside(index + 1),
                onPressed: () {
                  dismiss();
                  widget.reveal(card);
                },
              ),
          if (p != null && widget.edit != null)
            ContextMenuButtonItem(
              label: l10n.studyEditMarkdown,
              onPressed: () {
                dismiss();
                widget.edit!(p);
              },
            ),
          for (final item in readingSelectionActions(region))
            item.copyWith(onPressed: () {
              _desktopMenu.remove();
              item.onPressed?.call();
            }),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final accent = StudyChrome.of(context).accent;
    final dark = Theme.of(context).brightness == Brightness.dark;
    return IgnorePointer(
      ignoring: !widget.selectable,
      child: Listener(
        onPointerDown: (_) => _desktopMenu.remove(),
        onPointerSignal: (_) => _desktopMenu.remove(),
        onPointerPanZoomStart: (_) => _desktopMenu.remove(),
        onPointerUp: (event) {
          if (event.kind != PointerDeviceKind.mouse) return;
          // Desktop SelectionArea normally only shows a menu on secondary click.
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (!mounted || _quote.isEmpty || _area.currentState == null)
              return;
            final region = _area.currentState!.selectableRegion;
            if (!_range.registered || _range.selection.range == null) return;
            final anchors = region.contextMenuAnchors;
            _desktopMenu.show(
              context: context,
              contextMenuBuilder: (_) => _menu(region, anchors),
            );
          });
        },
        child: SelectionArea(
          key: _area,
          onSelectionChanged: (value) {
            _quote = value?.plainText ?? '';
            if (_quote.isEmpty) _desktopMenu.remove();
          },
          contextMenuBuilder: (_, region) =>
              _menu(region, region.contextMenuAnchors),
          child: SelectionListener(
            selectionNotifier: _range,
            child: CustomPaint(
              key: _passagePaint,
              foregroundPainter: _MarkdownPassagePainter(
                boxes: _passageBoxes,
                color: accent.withValues(alpha: .24),
                marks: widget.marks,
                body: widget.body,
                locateMarks: _markBoxes,
                lineColor: accent.withValues(alpha: .78),
                highlightColor: dark
                    ? const Color(0x45CA9238)
                    : const Color(0x59FFD76A),
              ),
              child: widget.child,
            ),
          ),
        ),
      ),
    );
  }
}

class _MarkdownPassagePainter extends CustomPainter {
  _MarkdownPassagePainter({
    required this.boxes,
    required this.color,
    required this.marks,
    required this.body,
    required this.locateMarks,
    required this.lineColor,
    required this.highlightColor,
  }) : super(repaint: boxes);

  final ValueListenable<List<Rect>> boxes;
  final Color color, lineColor, highlightColor;
  final List<_MarkdownMark> marks;
  final String body;
  final Iterable<({Rect box, bool highlight})> Function() locateMarks;

  @override
  void paint(Canvas canvas, Size size) {
    if (marks.isNotEmpty) _paintMarks(canvas, size);
    final ink = Paint()..color = color;
    for (final rect in boxes.value) {
      canvas.drawRRect(
        RRect.fromRectAndRadius(rect, const Radius.circular(Corners.xs)),
        ink,
      );
    }
  }

  void _paintMarks(Canvas canvas, Size size) {
    final highlights = Path();
    final underline = Path();
    for (final (:box, :highlight) in locateMarks()) {
      if (highlight) {
        highlights.addRRect(
          RRect.fromRectAndRadius(
            box.inflate(1),
            const Radius.circular(Corners.xs),
          ),
        );
      } else {
        final y = math.min(box.bottom + 1, size.height - 1);
        for (var x = box.left; x < box.right; x += 7) {
          underline
            ..moveTo(x, y)
            ..lineTo(math.min(x + 4, box.right), y);
        }
      }
    }
    canvas.drawPath(highlights, Paint()..color = highlightColor);
    canvas.drawPath(
      underline,
      Paint()
        ..color = lineColor
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.25
        ..strokeCap = StrokeCap.round,
    );
  }

  @override
  bool shouldRepaint(_MarkdownPassagePainter old) =>
      old.boxes != boxes ||
      old.body != body ||
      old.color != color ||
      old.lineColor != lineColor ||
      old.highlightColor != highlightColor ||
      !listEquals(old.marks, marks);
}
