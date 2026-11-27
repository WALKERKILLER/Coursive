import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart' show RenderParagraph, ScrollDirection;

import '../theme/tokens.dart';
import 'coursive_markdown.dart';
import 'reading_selection_menu.dart';
import '../l10n/l10n.dart';

/// A heading of a reader document; [section] is the section it opens.
class MdHeading {
  const MdHeading(this.level, this.title, this.section);
  final int level;
  final String title;
  final int section;
}

/// Markdown split at headings (outside code fences), so long documents build
/// lazily and the reader can jump to and track sections.
class MdDoc {
  MdDoc(String md) {
    final heading = RegExp(r'^ {0,3}(#{1,6})[ \t]+(.+?)[ \t]*$');
    final fence = RegExp(r'^\s*(```|~~~)');
    final ids = MarkdownHeadingIds();
    var buf = StringBuffer();
    var inFence = false;
    var offset = 0, sectionStart = 0;
    for (final line in md.split('\n')) {
      if (fence.hasMatch(line)) inFence = !inFence;
      final m = inFence ? null : heading.firstMatch(line);
      if (m != null) {
        if (buf.toString().trim().isNotEmpty || sections.isNotEmpty) { sections.add(buf.toString()); starts.add(sectionStart); }
        sectionStart = offset;
        buf = StringBuffer();
        final title = _plain(m[2]!);
        headings.add(MdHeading(m[1]!.length, title, sections.length));
        headingSections[ids.add(title)] = sections.length;
      }
      buf.writeln(line);
      offset += line.length + 1;
    }
    if (buf.toString().trim().isNotEmpty) { sections.add(buf.toString()); starts.add(sectionStart); }
    lower = [for (final s in sections) s.toLowerCase()];
  }

  final sections = <String>[];
  final starts = <int>[];
  final headings = <MdHeading>[];
  final headingSections = <String, int>{};
  late final List<String> lower;

  static String _plain(String s) => s
      .replaceFirst(RegExp(r'[ \t]+#+[ \t]*$'), '')
      .replaceAllMapped(RegExp(r'\[([^\]]*)\]\([^)]*\)'), (m) => m[1]!)
      .replaceAll(RegExp(r'[*_`]|\$'), '')
      .trim();

  /// Sections containing [q] (case-insensitive), in order.
  List<int> find(String q) {
    final k = q.trim().toLowerCase();
    if (k.isEmpty) return const [];
    return [for (var i = 0; i < lower.length; i++) if (lower[i].contains(k)) i];
  }
}

/// Lets a parent drive a [MarkdownReader]: jump, search, and follow position.
class MdReaderController extends ChangeNotifier {
  _MarkdownReaderState? _state;

  /// Index of the section at the top of the viewport.
  final _section = ValueNotifier<int>(0);
  int get current => _section.value;

  /// 0…1 reading progress.
  double progress = 0;

  void jumpToSection(int i) => _state?._jump(i);
  void jumpToHeading(MdHeading h) => _state?._jump(h.section);

  /// Jumps to the first section containing [q]; returns false when absent.
  bool reveal(String q) {
    final s = _state;
    if (s == null) return false;
    final hits = s.doc.find(q);
    if (hits.isEmpty) return false;
    s._jump(hits.first, flash: true);
    return true;
  }

  void _update(int c, double p) {
    if (c == current && (p - progress).abs() < 0.005) return;
    _section.value = c;
    progress = p;
    notifyListeners();
  }

  @override
  void dispose() {
    _section.dispose();
    super.dispose();
  }
}

typedef MarkdownSectionDecorator = Widget Function(String text, int sourceOffset, Widget section);

/// Reading surface: a centred column of Markdown sections with a header.
class MarkdownReader extends StatefulWidget {
  const MarkdownReader({
    super.key,
    required this.doc,
    required this.controller,
    this.header,
    this.lectureId = '',
    this.baseDir = '',
    this.onRef,
    this.maxWidth = 760,
    this.padding = const EdgeInsets.fromLTRB(Space.xl, Space.lg, Space.xl, Space.huge),
    this.initialQuery,
    this.decorateSection,
    this.bookmark,
    this.onBookmark,
  });

  final MdDoc doc;
  final MdReaderController controller;
  final Widget? header;
  final String lectureId, baseDir;
  final ValueChanged<CoursiveRef>? onRef;
  final double maxWidth;
  final EdgeInsets padding;

  /// Jump to the first passage containing this text once laid out.
  final String? initialQuery;
  final MarkdownSectionDecorator? decorateSection;
  final Map? bookmark;
  final ValueChanged<Map<String, dynamic>>? onBookmark;

  @override
  State<MarkdownReader> createState() => _MarkdownReaderState();
}

class _MarkdownReaderState extends State<MarkdownReader> {
  final _scroll = ScrollController();
  final _keys = <int, GlobalKey>{};
  int? _flash;
  Timer? _clearFlash;
  int _jumpRequest = 0;
  int _inputEpoch = 0;
  int? _pendingJump, _endSection;
  bool _trackScheduled = false;
  bool _restoredBookmark = false;
  int? _restoreEpoch;
  bool get _restoring => _restoreEpoch == _inputEpoch;

  Iterable<RenderParagraph> _paragraphs(RenderObject root) sync* {
    if (root is RenderParagraph) yield root;
    final children = <RenderObject>[];
    root.visitChildren(children.add);
    for (final child in children) { yield* _paragraphs(child); }
  }

  Future<void> _restoreBookmark(Map mark) async {
    if (!mounted || !_scroll.hasClients || _restoredBookmark) return;
    _restoredBookmark = true;
    final epoch = _inputEpoch;
    _restoreEpoch = epoch;
    try {
      final quote = mark['quote'] as String? ?? '';
      final ratio = (mark['ratio'] as num? ?? 0).clamp(0, 1);
      _scroll.jumpTo(_scroll.position.maxScrollExtent * ratio);
      await WidgetsBinding.instance.endOfFrame;
      if (!mounted || epoch != _inputEpoch || !_scroll.hasClients || quote.isEmpty) return;
      final hits = doc.find(quote);
      if (hits.length == 1 && _keys[hits.first]?.currentContext == null) {
        await _jump(hits.first, restoring: true);
      }
      if (!mounted || epoch != _inputEpoch || !_scroll.hasClients) return;
      final viewport = context.findRenderObject()!;
      for (final paragraph in _paragraphs(viewport)) {
        if (!paragraph.text.toPlainText().startsWith(quote)) continue;
        final y = paragraph.localToGlobal(Offset.zero, ancestor: viewport).dy;
        final target = _scroll.offset + y + paragraph.size.height * (mark['fraction'] as num? ?? 0);
        _scroll.jumpTo(target.clamp(0.0, _scroll.position.maxScrollExtent));
        break;
      }
    } finally {
      if (_restoreEpoch == epoch) _restoreEpoch = null;
    }
  }

  bool _onScroll(ScrollNotification event) {
    if (event.depth != 0) return false;
    if ((event is ScrollStartNotification && event.dragDetails != null) ||
        (event is UserScrollNotification && event.direction != ScrollDirection.idle)) {
      _inputEpoch++;
      _restoredBookmark = true;
      _jumpRequest++;
      _pendingJump = _endSection = null;
    }
    if (event is ScrollEndNotification && _pendingJump == null) _saveBookmark();
    return false;
  }

  void _saveBookmark() {
    if (_restoring || !_scroll.hasClients || widget.onBookmark == null) return;
    final pos = _scroll.position;
    if (pos.maxScrollExtent <= 0) return;
    final viewport = context.findRenderObject()!;
    var quote = '';
    var fraction = 0.0;
    for (final p in _paragraphs(viewport)) {
      final y = p.localToGlobal(Offset.zero, ancestor: viewport).dy;
      final text = p.text.toPlainText();
      if (y + p.size.height <= 0 || text.trim().isEmpty) continue;
      quote = String.fromCharCodes(text.runes.take(70));
      fraction = (-y / p.size.height).clamp(0.0, 1.0);
      break;
    }
    widget.onBookmark!({'ratio': (pos.pixels / pos.maxScrollExtent).clamp(0.0, 1.0), 'quote': quote, 'fraction': fraction});
  }

  MdDoc get doc => widget.doc;

  @override
  void initState() {
    super.initState();
    widget.controller._state = this;
    _scroll.addListener(_scheduleTrack);
    final q = widget.initialQuery;
    if (q != null && q.isNotEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) widget.controller.reveal(q);
      });
    }
  }

  @override
  void didUpdateWidget(MarkdownReader old) {
    super.didUpdateWidget(old);
    if (old.controller != widget.controller) {
      if (old.controller._state == this) old.controller._state = null;
      widget.controller._state = this;
    }
    if (old.doc != doc) {
      _inputEpoch++;
      _jumpRequest++;
      _pendingJump = _endSection = null;
      _restoredBookmark = false;
    }
  }

  @override
  void dispose() {
    if (widget.controller._state == this) widget.controller._state = null;
    _clearFlash?.cancel();
    _scroll.dispose();
    super.dispose();
  }

  void _scheduleTrack() {
    if (_trackScheduled) return;
    _trackScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _trackScheduled = false;
      if (mounted) _track();
    });
  }

  void _track() {
    _keys.removeWhere((_, key) => key.currentContext == null);
    if (!_scroll.hasClients) return;
    final pos = _scroll.position;
    final viewport = context.findRenderObject() as RenderBox?;
    if (viewport == null) return;
    var cur = 0;
    for (final e in _keys.entries) {
      final box = e.value.currentContext?.findRenderObject() as RenderBox?;
      if (box == null || !box.attached) continue;
      final top = box.localToGlobal(Offset.zero, ancestor: viewport).dy;
      if (top <= 96 && e.key > cur) cur = e.key;
    }
    widget.controller._update(pos.extentAfter < 1 ? _endSection ?? cur : cur, pos.maxScrollExtent <= 0 ? 1 : (pos.pixels / pos.maxScrollExtent).clamp(0.0, 1.0));
  }

  Future<void> _jump(int i, {bool flash = false, bool restoring = false}) async {
    if (!_scroll.hasClients || i < 0 || i >= doc.sections.length) return;
    if (!restoring) {
      _inputEpoch++;
      _restoredBookmark = true;
    }
    final request = ++_jumpRequest;
    final replacing = _pendingJump != null;
    _pendingJump = request;
    _endSection = null;
    _clearFlash?.cancel();
    if (_flash != null) setState(() => _flash = null);
    if (replacing && _scroll.position.isScrollingNotifier.value) {
      _scroll.jumpTo(_scroll.offset);
    }

    double? lower, upper;
    try {
      // Each layout tells us which side of the target we reached. Extents can
      // change as the lazy list lays out unusually long or short sections.
      if (replacing) await WidgetsBinding.instance.endOfFrame;
      for (var probe = 0; probe < 48; probe++) {
        if (!mounted || request != _jumpRequest || !_scroll.hasClients) return;
        final ctx = _keys[i]?.currentContext;
        if (ctx != null && ctx.mounted) {
          await Scrollable.ensureVisible(ctx, alignment: 0,
              duration: Motion.durationOf(context, Motion.short4), curve: Motion.emphasized);
          await WidgetsBinding.instance.endOfFrame;
          if (!mounted || request != _jumpRequest || !_scroll.hasClients) return;
          _endSection = _scroll.position.extentAfter < 1 ? i : null;
          _track();
          if (!restoring) _saveBookmark();
          if (flash) {
            setState(() => _flash = i);
            _clearFlash = Timer(const Duration(milliseconds: 1600), () => setState(() => _flash = null));
          }
          return;
        }
        final pos = _scroll.position;
        final live = _keys.entries.where((entry) => entry.value.currentContext != null);
        if (live.isEmpty) return;
        if (live.every((entry) => entry.key < i)) {
          lower = pos.pixels;
        } else {
          upper = pos.pixels;
        }
        final low = (lower ?? pos.minScrollExtent).clamp(pos.minScrollExtent, pos.maxScrollExtent);
        final high = (upper ?? pos.maxScrollExtent).clamp(low, pos.maxScrollExtent);
        final estimate = pos.maxScrollExtent * i / doc.sections.length;
        var offset = probe == 0 && estimate > low && estimate < high
            ? estimate : (low + high) / 2;
        if ((offset - pos.pixels).abs() < 1) {
          offset = lower == pos.pixels ? high : low;
          if (offset == pos.pixels) return;
        }
        _scroll.jumpTo(offset);
        await WidgetsBinding.instance.endOfFrame;
      }
    } finally {
      if (request == _jumpRequest) _pendingJump = null;
    }
  }

  bool _followFragment(String fragment) {
    final section = fragment.isEmpty ? 0 : doc.headingSections[fragment];
    if (section == null || section >= doc.sections.length) return false;
    _jump(section);
    return true;
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final n = doc.sections.length + (widget.header == null ? 0 : 1);
    if (widget.bookmark case final Map mark when !_restoredBookmark) {
      WidgetsBinding.instance.addPostFrameCallback((_) => _restoreBookmark(mark));
    }
    Widget decorate(int i, Widget child) => widget.decorateSection?.call(doc.sections[i], doc.starts[i], child) ?? child;
    final list = ListView.builder(
        controller: _scroll,
        padding: widget.padding,
        itemCount: n,
        itemBuilder: (context, k) {
          Widget child;
          if (widget.header != null && k == 0) {
            child = widget.header!;
          } else {
            final i = k - (widget.header == null ? 0 : 1);
            child = KeyedSubtree(
              key: _keys.putIfAbsent(i, GlobalKey.new),
              child: AnimatedContainer(
                duration: Motion.durationOf(context, Motion.long2),
                curve: Motion.standard,
                padding: const EdgeInsets.symmetric(horizontal: Space.sm),
                decoration: BoxDecoration(
                  color: _flash == i ? cs.tertiaryContainer.withValues(alpha: 0.45) : Colors.transparent,
                  borderRadius: BorderRadius.circular(Corners.md),
                ),
                child: decorate(i, CoursiveMarkdown(
                  doc.sections[i],
                  style: tt.bodyLarge?.copyWith(height: 1.75),
                  lectureId: widget.lectureId,
                  baseDir: widget.baseDir,
                  onRef: widget.onRef,
                  onFragment: _followFragment,
                )),
              ),
            );
          }
          return Center(child: ConstrainedBox(constraints: BoxConstraints(maxWidth: widget.maxWidth), child: child));
        },
    );
    final body = NotificationListener<ScrollNotification>(onNotification: _onScroll, child: list);
    return widget.decorateSection != null ? body :
        SelectionArea(contextMenuBuilder: readingSelectionMenu, child: body);
  }
}

/// Contents list in the style of an M3 navigation drawer: indented by level,
/// the section being read marked with a pill.
class ReaderToc extends StatelessWidget {
  const ReaderToc({super.key, required this.doc, required this.controller, this.onPicked});
  final MdDoc doc;
  final MdReaderController controller;
  final VoidCallback? onPicked;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    if (doc.headings.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(Space.lg),
        child: Text(l10n.readerNoHeadings, style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant)),
      );
    }
    final top = doc.headings.map((h) => h.level).reduce((a, b) => a < b ? a : b);
    return ListenableBuilder(
      listenable: controller._section,
      builder: (context, _) {
        // The heading whose section is being read: the last one at or above it.
        var active = 0;
        for (var i = 0; i < doc.headings.length; i++) {
          if (doc.headings[i].section <= controller.current) active = i;
        }
        return ListView.builder(
          padding: const EdgeInsets.symmetric(horizontal: Space.md, vertical: Space.sm),
          itemCount: doc.headings.length,
          itemBuilder: (context, i) {
            final h = doc.headings[i];
            final on = i == active;
            final depth = (h.level - top).clamp(0, 3);
            return Padding(
              padding: const EdgeInsets.only(bottom: 2),
              child: Material(
                color: on ? cs.secondaryContainer : Colors.transparent,
                shape: const StadiumBorder(),
                clipBehavior: Clip.antiAlias,
                child: InkWell(
                  onTap: () {
                    controller.jumpToHeading(h);
                    onPicked?.call();
                  },
                  child: Padding(
                    padding: EdgeInsets.fromLTRB(Space.lg + depth * Space.md, 10, Space.lg, 10),
                    child: Text(
                      h.title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: (depth == 0 ? tt.labelLarge : tt.bodyMedium)?.copyWith(
                        color: on ? cs.onSecondaryContainer : (depth == 0 ? cs.onSurface : cs.onSurfaceVariant),
                        fontWeight: on ? FontWeight.w600 : null,
                      ),
                    ),
                  ),
                ),
              ),
            );
          },
        );
      },
    );
  }
}

/// In-document search bar: query field, match counter, previous/next.
class ReaderSearchBar extends StatefulWidget {
  const ReaderSearchBar({super.key, required this.doc, required this.controller, required this.onClose});
  final MdDoc doc;
  final MdReaderController controller;
  final VoidCallback onClose;
  @override
  State<ReaderSearchBar> createState() => _ReaderSearchBarState();
}

class _ReaderSearchBarState extends State<ReaderSearchBar> {
  final _q = TextEditingController();
  List<int> _hits = const [];
  int _at = 0;

  @override
  void dispose() {
    _q.dispose();
    super.dispose();
  }

  void _go(int d) {
    if (_hits.isEmpty) return;
    setState(() => _at = (_at + d) % _hits.length);
    widget.controller._state?._jump(_hits[_at], flash: true);
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.fromLTRB(Space.lg, 0, Space.lg, Space.sm),
      child: Material(
        color: cs.surfaceContainerHigh,
        shape: const StadiumBorder(),
        child: Row(children: [
          const SizedBox(width: Space.lg),
          Icon(Icons.search_rounded, color: cs.onSurfaceVariant),
          const SizedBox(width: Space.sm),
          Expanded(
            child: TextField(
              controller: _q,
              autofocus: true,
              decoration: InputDecoration(hintText: l10n.readerFindHint, border: InputBorder.none, filled: false, isDense: true),
              onChanged: (v) => setState(() {
                _hits = widget.doc.find(v);
                _at = 0;
                if (_hits.isNotEmpty) widget.controller._state?._jump(_hits.first, flash: true);
              }),
              onSubmitted: (_) => _go(1),
            ),
          ),
          if (_q.text.isNotEmpty)
            Text(_hits.isEmpty ? l10n.readerNoMatches : '${_at + 1}/${_hits.length}', style: TextStyle(color: cs.onSurfaceVariant)),
          IconButton(tooltip: l10n.readerPreviousMatch, onPressed: _hits.isEmpty ? null : () => _go(-1), icon: const Icon(Icons.keyboard_arrow_up_rounded)),
          IconButton(tooltip: l10n.readerNextMatch, onPressed: _hits.isEmpty ? null : () => _go(1), icon: const Icon(Icons.keyboard_arrow_down_rounded)),
          IconButton(tooltip: l10n.commonClose, onPressed: widget.onClose, icon: const Icon(Icons.close_rounded)),
          const SizedBox(width: Space.xs),
        ]),
      ),
    );
  }
}
