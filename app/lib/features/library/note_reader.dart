import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:crypto/crypto.dart' show sha256;
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'package:path/path.dart' as p;
import 'package:provider/provider.dart';

import '../../core/activity_tracker.dart';
import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/format.dart';
import '../../core/models.dart';
import '../../theme/tokens.dart';
import '../../widgets/coursive_markdown.dart';
import '../../widgets/md_reader.dart';
import '../../widgets/report_screen.dart';
import '../../widgets/states.dart';
import '../player/player_page.dart';
import '../plugins/plugin_views.dart';
import 'note_editor.dart';
import '../../l10n/l10n.dart';

/// A note opened for reading: contents on the left, the text in a readable
/// column, and the sources a reference points to (recording, slide or
/// courseware) in a supporting pane — a bottom sheet on narrow windows.
class NotePage extends StatefulWidget {
  const NotePage({super.key, this.noteId, this.courseId = ''})
      : document = null, onBack = null, onEdit = null, onPlay = null, decorateSection = null, bookmark = null, onBookmark = null;
  const NotePage.canvas({super.key, required Note this.document,
    required VoidCallback this.onBack, this.onEdit, required this.onPlay, this.decorateSection, this.bookmark, this.onBookmark})
      : noteId = null, courseId = '';
  final Note? document;
  final MarkdownSectionDecorator? decorateSection;
  final Map? bookmark;
  final ValueChanged<Map<String, dynamic>>? onBookmark;
  final VoidCallback? onBack, onEdit;
  final void Function(String, int)? onPlay;
  final String? noteId;
  final String courseId;
  @override
  State<NotePage> createState() => _NotePageState();
}

class _NotePageState extends State<NotePage> {
  Note? _note;
  MdDoc? _doc;
  bool _loading = true;
  bool _editing = false;
  bool _search = false;
  Object? _error;
  CoursiveRef? _ref;
  final _reader = MdReaderController();

  @override
  void initState() {
    super.initState();
    if (widget.document case final note?) {
      _note = note;
      _doc = MdDoc(note.body);
      _loading = false;
    } else if (widget.noteId == null) {
      _loading = false;
      _editing = true;
    } else {
      _load();
    }
  }

  @override
  void didUpdateWidget(NotePage old) {
    super.didUpdateWidget(old);
    if (widget.document case final note?) {
      if (note.body != _note?.body) _doc = MdDoc(note.body);
      _note = note;
    }
  }

  @override
  void dispose() {
    _reader.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final r = await context.read<AppState>().api.get('/api/notes/${widget.noteId}');
      _set(Note((r as Map).cast<String, dynamic>()));
    } catch (e) {
      _error = e;
    }
    if (mounted) setState(() => _loading = false);
  }

  void _set(Note n) {
    _note = n;
    _doc = MdDoc(n.body);
    if (mounted) setState(() {});
  }

  String get _lectureId => _note?.lectureId ?? '';

  void _open(CoursiveRef ref, bool wide) {
    final play = widget.onPlay;
    if (play != null && ref is TimeRef && ref.quote == null) {
      play(ref.lectureId.isEmpty ? _lectureId : ref.lectureId, ref.startMs ?? ref.seconds * 1000);
      return;
    }
    if (ref is LectureRef || ref is FileRef || ref is NoteRef || ref is CanvasRef) {
      openRef(context, ref);
      return;
    }
    if (wide) {
      setState(() => _ref = ref);
      return;
    }
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      useSafeArea: true,
      builder: (sheet) => FractionallySizedBox(heightFactor: 0.85, child: SourcePane(ref: ref, lectureId: _lectureId,
        onPlay: play == null ? null : (id, ms) { Navigator.pop(sheet); play(id, ms); },
      )),
    );
  }

  Future<void> _delete() async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        icon: const Icon(Icons.delete_outline_rounded),
        title: Text(l10n.noteDeleteTitle),
        content: Text(l10n.noteDeleteBody),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: Text(l10n.commonCancel)),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: Text(l10n.commonDelete)),
        ],
      ),
    );
    if (ok != true || !mounted) return;
    await context.read<AppState>().api.delete('/api/notes/${_note!.id}');
    if (mounted) Navigator.pop(context);
  }

  void _showToc() => showModalBottomSheet(
        context: context,
        showDragHandle: true,
        builder: (c) => SizedBox(
          height: MediaQuery.sizeOf(c).height * 0.6,
          child: ReaderToc(doc: _doc!, controller: _reader, onPicked: () => Navigator.pop(c)),
        ),
      );

  @override
  Widget build(BuildContext context) {
    final app = context.read<AppState?>();
    final n = _note;
    final cs = Theme.of(context).colorScheme;
    final canvas = widget.document != null;
    final page = LayoutBuilder(builder: (context, c) {
        final tocPane = c.maxWidth >= 1040;
        final sidePane = c.maxWidth >= 1200;
        final reading = !_loading && _error == null && !_editing && n != null;
        return Scaffold(
          backgroundColor: canvas ? cs.surface : Colors.transparent,
          appBar: AppBar(
            backgroundColor: Colors.transparent,
            leading: widget.onBack == null ? null : BackButton(onPressed: widget.onBack),
            title: Text(n == null ? l10n.noteNew : n.title, maxLines: 1, overflow: TextOverflow.ellipsis),
            actions: [
              if (reading) ...[
                IconButton(tooltip: l10n.commonFind, isSelected: _search, onPressed: () => setState(() => _search = !_search), icon: const Icon(Icons.search_rounded)),
                if (!tocPane && _doc!.headings.isNotEmpty) IconButton(tooltip: l10n.commonContents, onPressed: _showToc, icon: const Icon(Icons.toc_rounded)),
                if (!canvas || widget.onEdit != null) IconButton(tooltip: l10n.commonEdit, onPressed: widget.onEdit ?? () => setState(() => _editing = true), icon: const Icon(Icons.edit_outlined)),
              ],
              if (reading && !canvas)
                PluginMenuButton(
                  surface: 'note.toolbar',
                  noteViews: true,
                  exportScope: 'note',
                  context_: {'note_id': n.id, if (n.lectureId.isNotEmpty) 'lecture_id': n.lectureId, if (n.courseId.isNotEmpty) 'course_id': n.courseId},
                ),
              if (!canvas && n != null && n.lectureId.isNotEmpty)
                IconButton(
                  tooltip: l10n.noteOpenLecture,
                  onPressed: () => Navigator.of(context, rootNavigator: true).push(MaterialPageRoute(builder: (_) => PlayerPage(lectureId: n.lectureId))),
                  icon: const Icon(Icons.ondemand_video_rounded),
                ),
              if (n != null)
                MenuAnchor(
                  menuChildren: [
                    MenuItemButton(
                      leadingIcon: const Icon(Icons.copy_rounded),
                      onPressed: () {
                        Clipboard.setData(ClipboardData(text: n.body));
                        showSnack(context, l10n.noteCopiedMarkdown);
                      },
                      child: Text(l10n.noteCopyMarkdown),
                    ),
                    if (!canvas) MenuItemButton(leadingIcon: const Icon(Icons.delete_outline_rounded), onPressed: _delete, child: Text(l10n.commonDelete)),
                  ],
                  builder: (context, m, _) => IconButton(tooltip: l10n.commonMore, onPressed: () => m.isOpen ? m.close() : m.open(), icon: const Icon(Icons.more_vert_rounded)),
                ),
              const SizedBox(width: Space.sm),
            ],
            bottom: reading
                ? PreferredSize(
                    preferredSize: const Size.fromHeight(2),
                    child: ListenableBuilder(
                      listenable: _reader,
                      builder: (_, _) => LinearProgressIndicator(value: _reader.progress, minHeight: 2, backgroundColor: Colors.transparent),
                    ),
                  )
                : null,
          ),
          body: _loading
              ? const CenteredLoader()
              : _error != null
                  ? ErrorState(error: _error!)
                  : _editing || n == null
                      ? ContentWidth(
                          max: 860,
                          child: Padding(
                            padding: const EdgeInsets.fromLTRB(Space.xl, 0, Space.xl, Space.xl),
                            child: NoteEditor(
                              note: n,
                              courseId: widget.courseId,
                              edit: true,
                              onSaved: (saved) => _set(saved),
                              onClose: n == null ? null : () => setState(() => _editing = false),
                            ),
                          ),
                        )
                      : Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                          if (tocPane && _doc!.headings.isNotEmpty)
                            SizedBox(
                              width: 272,
                              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                                Padding(
                                  padding: const EdgeInsets.fromLTRB(Space.xl, Space.lg, Space.lg, Space.xs),
                                  child: Text(l10n.commonContents, style: Theme.of(context).textTheme.titleSmall?.copyWith(color: cs.onSurfaceVariant)),
                                ),
                                Expanded(child: ReaderToc(doc: _doc!, controller: _reader)),
                              ]),
                            ),
                          Expanded(
                            child: Column(children: [
                              AnimatedSize(
                                duration: Motion.durationOf(context, Motion.medium2),
                                curve: Motion.emphasized,
                                child: _search ? ReaderSearchBar(doc: _doc!, controller: _reader, onClose: () => setState(() => _search = false)) : const SizedBox(width: double.infinity),
                              ),
                              Expanded(
                                child: MarkdownReader(
                                  key: ValueKey(n.updatedAt),
                                  doc: _doc!,
                                  controller: _reader,
                                  decorateSection: widget.decorateSection,
                                  bookmark: widget.bookmark,
                                  onBookmark: widget.onBookmark,
                                  lectureId: n.lectureId,
                                  baseDir: p.posix.dirname(n.path),
                                  onRef: (r) => _open(r, sidePane),
                                  header: _NoteHeader(note: n, courseTitle: app?.course(n.courseId)?.title ?? ''),
                                ),
                              ),
                            ]),
                          ),
                          AnimatedSize(
                            duration: Motion.durationOf(context, Motion.medium4),
                            curve: Motion.emphasized,
                            alignment: Alignment.centerLeft,
                            child: sidePane && _ref != null
                                ? Padding(
                                    padding: const EdgeInsets.fromLTRB(0, Space.sm, Space.lg, Space.lg),
                                    child: SizedBox(
                                      width: 400,
                                      child: Material(
                                        color: cs.surfaceContainerLow,
                                        borderRadius: BorderRadius.circular(Corners.xl),
                                        clipBehavior: Clip.antiAlias,
                                        child: SourcePane(
                                          key: ValueKey(_ref),
                                          ref: _ref!,
                                          onPlay: widget.onPlay,
                                          lectureId: _lectureId,
                                          onClose: () => setState(() => _ref = null),
                                        ),
                                      ),
                                    ),
                                  )
                                : const SizedBox(height: double.infinity),
                          ),
                        ]),
        );
    });
    if (canvas) return page;
    return ReportScreen(
      value: {'screen': l10n.commonNotes, if (n != null) 'note_id': n.id, if (n != null) 'note_title': n.title, if (n != null && n.lectureId.isNotEmpty) 'lecture_id': n.lectureId},
      child: ActivityRegion(kind: 'read', lectureId: _lectureId, child: page),
    );
  }
}

class _NoteHeader extends StatelessWidget {
  const _NoteHeader({required this.note, required this.courseTitle});
  final Note note;
  final String courseTitle;
  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final agent = note.source == 'agent';
    return Padding(
      padding: const EdgeInsets.fromLTRB(Space.sm, Space.sm, Space.sm, Space.xl),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Text(note.title, style: tt.headlineSmall),
        const SizedBox(height: Space.sm),
        Wrap(spacing: Space.sm, runSpacing: Space.xs, crossAxisAlignment: WrapCrossAlignment.center, children: [
          if (courseTitle.isNotEmpty) Text(courseTitle, style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant)),
          if (note.updatedAt > 0)
            Text(l10n.noteUpdated(fmtRelative(DateTime.fromMillisecondsSinceEpoch(note.updatedAt * 1000))), style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant)),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
            decoration: BoxDecoration(color: agent ? cs.tertiaryContainer : cs.secondaryContainer, borderRadius: BorderRadius.circular(Corners.sm)),
            child: Row(mainAxisSize: MainAxisSize.min, children: [
              Icon(agent ? Icons.auto_awesome_outlined : Icons.edit_note_rounded, size: 14, color: agent ? cs.onTertiaryContainer : cs.onSecondaryContainer),
              const SizedBox(width: 4),
              Text(agent ? l10n.noteByAssistant : l10n.noteMine, style: tt.labelSmall?.copyWith(color: agent ? cs.onTertiaryContainer : cs.onSecondaryContainer)),
            ]),
          ),
        ]),
        const SizedBox(height: Space.lg),
        Divider(height: 1, color: cs.outlineVariant),
      ]),
    );
  }
}

// ---------------------------------------------------------------- sources

/// What a reference points to: the moment in the recording (slide on screen
/// and what was said), a slide, or a courseware passage.
class SourcePane extends StatelessWidget {
  const SourcePane({super.key, required this.ref, required this.lectureId, this.onClose, this.onPlay});
  final CoursiveRef ref;

  /// Lecture that references without one belong to.
  final String lectureId;
  final VoidCallback? onClose;
  final void Function(String, int)? onPlay;

  @override
  Widget build(BuildContext context) {
    final r = ref;
    return switch (r) {
      TimeRef() => _MomentSource(lectureId: r.lectureId.isNotEmpty ? r.lectureId : lectureId, ms: r.startMs ?? r.seconds * 1000, endMs: r.endMs, quote: r.quote, onClose: onClose, onPlay: onPlay),
      SlideRef() => _MomentSource(lectureId: r.lectureId, slideIdx: r.idx, snapshotOnly: r.snapshotOnly, imageRevision: r.imageRevision, onClose: onClose),
      CoursewareRef() when r.page!=null && r.indexRevision!=null => _ClassroomPage(ref:r,onClose:onClose),
      CoursewareRef() => CoursewareSource(lectureId: r.lectureId, file: r.file, query: r.query, verified: r.verified, page: r.page, indexRevision:r.indexRevision, onClose: onClose),
      // Lectures and files open in place of a pane (openRef).
      LectureRef() || FileRef() || NoteRef() || CanvasRef() => const SizedBox.shrink(),
    };
  }
}

class _PaneHeader extends StatelessWidget {
  const _PaneHeader({required this.icon, required this.title, this.onClose, this.trailing});
  final IconData icon;
  final String title;
  final VoidCallback? onClose;
  final Widget? trailing;
  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.fromLTRB(Space.lg, Space.sm, Space.xs, Space.xs),
      child: Row(children: [
        Icon(icon, size: 20, color: cs.primary),
        const SizedBox(width: Space.sm),
        Expanded(child: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: Theme.of(context).textTheme.titleMedium)),
        ?trailing,
        if (onClose != null) IconButton(tooltip: l10n.commonClose, onPressed: onClose, icon: const Icon(Icons.close_rounded)),
      ]),
    );
  }
}

class _MomentSource extends StatefulWidget {
  const _MomentSource({required this.lectureId, this.ms, this.endMs, this.slideIdx, this.quote, this.imageRevision, this.snapshotOnly = false, this.onClose, this.onPlay});
  final String lectureId;
  final int? ms, endMs, slideIdx;
  final String? quote, imageRevision;
  final bool snapshotOnly;
  final VoidCallback? onClose;
  final void Function(String, int)? onPlay;
  @override
  State<_MomentSource> createState() => _MomentSourceState();
}

class _MomentSourceState extends State<_MomentSource> {
  late final Future<(Slide?, List<Segment>, int)> _data = _load();

  Future<(Slide?, List<Segment>, int)> _load() async {
    final api = context.read<AppState>().api;
    final slides = widget.quote != null ? <Slide>[] : (jsonList(await api.get('/api/lectures/${widget.lectureId}/slides').catchError((_) => <dynamic>[]))
        .map((e) => Slide((e as Map).cast<String, dynamic>()))
        .toList()
      ..sort((a, b) => a.tSec.compareTo(b.tSec)));
    final keys = slides.any((s) => s.isKey) ? slides.where((s) => s.isKey).toList() : slides;
    Slide? slide;
    var ms = widget.ms ?? 0;
    if (widget.slideIdx != null) {
      slide = slides.where((s) => s.idx == widget.slideIdx).firstOrNull;
      ms = (slide?.tSec ?? 0) * 1000;
    } else {
      for (final s in keys) {
        if (s.tSec * 1000 <= ms + 5000) slide = s;
      }
    }
    if (widget.snapshotOnly) {
      if (slide == null) throw Exception(l10n.studyCitationUnavailable);
      if (widget.imageRevision != null) {
        final url = slideImageUrl(api.baseUrl,api.token,widget.lectureId,slide.idx);
        final response = await http.get(Uri.parse(url));
        if (response.statusCode != 200 || sha256.convert(response.bodyBytes).toString() != widget.imageRevision) {
          throw Exception(l10n.studyCitationUnavailable);
        }
      }
      return (slide, <Segment>[], ms);
    }
    final from = (ms - 30000).clamp(0, 1 << 40), to = (widget.endMs ?? (ms+90000)).clamp(ms+90000,ms+300000);
    final segs = jsonList(await api.get('/api/lectures/${widget.lectureId}/transcript', {'from_ms': '$from', 'to_ms': '$to'}).catchError((_) => <dynamic>[]))
        .map((e) => Segment((e as Map).cast<String, dynamic>()))
        .toList();
    if (widget.quote case final String quote) {
      final start = segs.indexWhere((s) => s.startMs == ms || (widget.endMs == null && s.startMs ~/ 1000 == ms ~/ 1000));
      if (start < 0 || !segs.skip(start).takeWhile((s) => widget.endMs == null || s.startMs < widget.endMs!).map((s)=>s.text.trim()).join('\n').contains(quote)) {
        throw Exception(l10n.studyCitationUnavailable);
      }
    }
    return (slide, segs, ms);
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final api = context.read<AppState>().api;
    return FutureBuilder(
      future: _data,
      builder: (context, snap) {
        final d = snap.data;
        final ms = d?.$3 ?? widget.ms ?? 0;
        return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          _PaneHeader(
            icon: widget.slideIdx != null ? Icons.slideshow_outlined : Icons.graphic_eq_rounded,
            title: widget.snapshotOnly ? l10n.noteSnapshotNumber(widget.slideIdx!) : widget.slideIdx != null ? l10n.noteSlideRef(widget.slideIdx!, fmtClock(ms)) : l10n.noteAudioRef(fmtClock(ms)),
            onClose: widget.onClose,
          ),
          Expanded(
            child: snap.hasError ? ErrorState(error: snap.error!) : d == null
                ? const CenteredLoader()
                : ListView(padding: const EdgeInsets.fromLTRB(Space.lg, Space.xs, Space.lg, Space.lg), children: [
                    if (d.$1 != null) ...[
                      GestureDetector(
                        onTap: () => showImageViewer(context, slideImageUrl(api.baseUrl, api.token, widget.lectureId, d.$1!.idx)),
                        child: MouseRegion(
                          cursor: SystemMouseCursors.zoomIn,
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(Corners.lg),
                            child: AspectRatio(
                              aspectRatio: 16 / 9,
                              child: Image.network(
                                slideImageUrl(api.baseUrl, api.token, widget.lectureId, d.$1!.idx),
                                fit: BoxFit.cover,
                                errorBuilder: (_, _, _) => ColoredBox(color: cs.surfaceContainerHighest, child: const Icon(Icons.broken_image_outlined)),
                              ),
                            ),
                          ),
                        ),
                      ),
                      Padding(
                        padding: const EdgeInsets.only(top: Space.xs, bottom: Space.lg),
                        child: Text(widget.snapshotOnly ? l10n.noteSnapshotNumber(d.$1!.idx) : l10n.noteSlideAt(d.$1!.idx, fmtClock(d.$1!.tSec * 1000)), style: tt.labelMedium?.copyWith(color: cs.onSurfaceVariant)),
                      ),
                    ],
                    if (!widget.snapshotOnly) FilledButton.tonalIcon(
                      onPressed: () {
                        if (widget.onPlay case final play?) {
                          play(widget.lectureId, ms);
                        } else {
                          seekOrOpen(context, widget.lectureId, ms ~/ 1000);
                        }
                      },
                      icon: const Icon(Icons.play_arrow_rounded),
                      label: Text(l10n.notePlayFrom(fmtClock(ms))),
                    ),
                    const SizedBox(height: Space.lg),
                    if (d.$2.isEmpty && !widget.snapshotOnly)
                      Text(l10n.noteNoTranscriptHere, style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant))
                    else
                      for (final s in d.$2) _SegmentLine(seg: s, current: s.startMs <= ms && ms < s.endMs, lectureId: widget.lectureId),
                  ]),
          ),
        ]);
      },
    );
  }
}

class _SegmentLine extends StatelessWidget {
  const _SegmentLine({required this.seg, required this.current, required this.lectureId});
  final Segment seg;
  final bool current;
  final String lectureId;
  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    return Material(
      color: current ? cs.secondaryContainer : Colors.transparent,
      borderRadius: BorderRadius.circular(Corners.md),
      child: InkWell(
        borderRadius: BorderRadius.circular(Corners.md),
        onTap: () => seekOrOpen(context, lectureId, seg.startMs ~/ 1000),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: Space.sm, vertical: 6),
          child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
            SizedBox(
              width: 48,
              child: Text(fmtClock(seg.startMs), style: tt.labelSmall?.copyWith(color: current ? cs.onSecondaryContainer : cs.primary, height: 1.9, fontFeatures: const [FontFeature.tabularFigures()])),
            ),
            Expanded(child: Text(seg.text, style: tt.bodyMedium?.copyWith(height: 1.55, color: current ? cs.onSecondaryContainer : cs.onSurface))),
          ]),
        ),
      ),
    );
  }
}

/// Loads a parsed courseware file's Markdown through daemon.
Future<(CoursewareFile, String)> loadParsedCourseware(Api api, String lectureId, String file) async {
  final list = jsonList(await api.get('/api/lectures/$lectureId/courseware')).map((e) => CoursewareFile((e as Map).cast<String, dynamic>()));
  var f = list.where((x) => x.path == file || x.name == file).firstOrNull;
  if (f == null && file.isNotEmpty) {
    final aliases = jsonList(await api.get('/api/lectures/$lectureId/courseware', {
      'former_path': file,
    }));
    if (aliases.length == 1) {
      f = CoursewareFile((aliases.single as Map).cast<String, dynamic>());
    }
  }
  if (f == null) throw Exception(l10n.coursewareGone);
  if (!f.parsed) throw Exception(l10n.coursewareNotParsed);
  final res = await http.get(Uri.parse(api.fileUrl(f.mdPath)));
  if (res.statusCode != 200) throw Exception(l10n.coursewareReadFailed(res.statusCode));
  return (f, utf8.decode(res.bodyBytes));
}

/// A courseware passage in the supporting pane, opened at [query].
class _ClassroomPage extends StatefulWidget {
  const _ClassroomPage({required this.ref,this.onClose});
  final CoursewareRef ref;
  final VoidCallback? onClose;
  @override
  State<_ClassroomPage> createState()=>_ClassroomPageState();
}
class _ClassroomPageState extends State<_ClassroomPage> {
  late final _page = _load();
  Future<Uint8List> _load() async {
    final ref=widget.ref;
    final result=await context.read<AppState>().api.get('/api/lectures/${Uri.encodeComponent(ref.lectureId)}/courseware/page-preview',{
      'file':ref.file,'page':'${ref.page}','rev':ref.indexRevision!,
    }) as Map;
    return base64Decode(result['png'] as String);
  }
  @override
  Widget build(BuildContext context) {
    final ref=widget.ref;
    return Column(children:[
      _PaneHeader(icon:Icons.picture_as_pdf_outlined,title:l10n.notePDFPage(p.posix.basename(ref.file),ref.page!),onClose:widget.onClose),
      Expanded(child:FutureBuilder(future:_page,builder:(context,snapshot) {
        if(snapshot.hasError)return ErrorState(error:snapshot.error!);
        if(snapshot.data==null)return const CenteredLoader();
        return InteractiveViewer(child:Image.memory(snapshot.data!,fit:BoxFit.contain));
      })),
    ]);
  }
}
class CoursewareSource extends StatefulWidget {
  const CoursewareSource({super.key, required this.lectureId, required this.file, this.query = '', this.verified = false, this.page, this.indexRevision, this.onClose});
  final String lectureId, file, query;
  final bool verified;
  final int? page;
  final String? indexRevision;
  final VoidCallback? onClose;
  @override
  State<CoursewareSource> createState() => _CoursewareSourceState();
}

class _CoursewareSourceState extends State<CoursewareSource> {
  late final _data = _load();
  Future<(CoursewareFile,String)> _load() async {
    final api = context.read<AppState>().api;
    final parsed = await loadParsedCourseware(api,widget.lectureId,widget.file);
    if (widget.verified && widget.query.isNotEmpty && !parsed.$2.contains(widget.query)) {
      throw Exception(l10n.studyCitationUnavailable);
    }
    return parsed;
  }
  final _reader = MdReaderController();

  @override
  void dispose() {
    _reader.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder(
      future: _data,
      builder: (context, snap) {
        final d = snap.data;
        return Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          _PaneHeader(
            icon: Icons.description_outlined,
            title: d == null ? l10n.assetCourseware : widget.page == null ? d.$1.name : l10n.notePDFPage(d.$1.name,widget.page!),
            onClose: widget.onClose,
            trailing: d == null
                ? null
                : IconButton(
                    tooltip: l10n.readerFullscreen,
                    onPressed: () => openCoursewareReader(context, d.$1, lectureId: widget.lectureId, query: widget.query),
                    icon: const Icon(Icons.open_in_full_rounded),
                  ),
          ),
          Expanded(
            child: snap.hasError
                ? ErrorState(error: snap.error!)
                : d == null
                    ? const CenteredLoader()
                    : MarkdownReader(
                        doc: MdDoc(d.$2),
                        controller: _reader,
                        baseDir: p.posix.dirname(d.$1.mdPath),
                        lectureId: widget.lectureId,
                        initialQuery: widget.query,
                        onRef: (r) => openRef(context, r, lectureId: widget.lectureId),
                        padding: const EdgeInsets.fromLTRB(Space.md, 0, Space.md, Space.xl),
                      ),
          ),
        ]);
      },
    );
  }
}

/// Full-window reader for a parsed courseware file, with contents and search.
Future<void> openCoursewareReader(BuildContext context, CoursewareFile file, {required String lectureId, String query = ''}) =>
    Navigator.of(context, rootNavigator: true).push(MaterialPageRoute(builder: (_) => _CoursewareReaderPage(file: file, lectureId: lectureId, query: query)));

class _CoursewareReaderPage extends StatefulWidget {
  const _CoursewareReaderPage({required this.file, required this.lectureId, required this.query});
  final CoursewareFile file;
  final String lectureId, query;
  @override
  State<_CoursewareReaderPage> createState() => _CoursewareReaderPageState();
}

class _CoursewareReaderPageState extends State<_CoursewareReaderPage> {
  late final _data = loadParsedCourseware(context.read<AppState>().api, widget.lectureId, widget.file.path);
  final _reader = MdReaderController();
  bool _search = false;

  @override
  void dispose() {
    _reader.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    return ActivityRegion(kind: 'read', lectureId: widget.lectureId, child: FutureBuilder(
      future: _data,
      builder: (context, snap) {
        final d = snap.data;
        final doc = d == null ? null : MdDoc(d.$2);
        return LayoutBuilder(builder: (context, c) {
          final tocPane = c.maxWidth >= 1040 && (doc?.headings.isNotEmpty ?? false);
          final parsedBy = l10n.coursewareParsedBy(widget.file.pages > 0 ? l10n.coursewarePages(widget.file.pages) : '');
          return Scaffold(
            appBar: AppBar(
              title: Text(widget.file.name, maxLines: 1, overflow: TextOverflow.ellipsis),
              actions: [
                // On a phone the title needs the room; the logo keeps the credit.
                Tooltip(
                  message: parsedBy,
                  child: Padding(
                    padding: const EdgeInsets.only(right: Space.sm),
                    child: Row(children: [
                      ClipRRect(borderRadius: BorderRadius.circular(4), child: Image.asset('assets/brands/mineru.png', width: 18, height: 18)),
                      if (!context.isCompact) ...[
                        const SizedBox(width: 6),
                        Text(parsedBy, style: tt.labelMedium?.copyWith(color: cs.onSurfaceVariant)),
                      ],
                    ]),
                  ),
                ),
                if (doc != null) ...[
                  IconButton(tooltip: l10n.commonFind, isSelected: _search, onPressed: () => setState(() => _search = !_search), icon: const Icon(Icons.search_rounded)),
                  if (!tocPane && doc.headings.isNotEmpty)
                    IconButton(
                      tooltip: l10n.commonContents,
                      onPressed: () => showModalBottomSheet(
                        context: context,
                        showDragHandle: true,
                        builder: (s) => SizedBox(height: MediaQuery.sizeOf(s).height * 0.6, child: ReaderToc(doc: doc, controller: _reader, onPicked: () => Navigator.pop(s))),
                      ),
                      icon: const Icon(Icons.toc_rounded),
                    ),
                ],
                const SizedBox(width: Space.sm),
              ],
              bottom: doc == null
                  ? null
                  : PreferredSize(
                      preferredSize: const Size.fromHeight(2),
                      child: ListenableBuilder(listenable: _reader, builder: (_, _) => LinearProgressIndicator(value: _reader.progress, minHeight: 2, backgroundColor: Colors.transparent)),
                    ),
            ),
            body: snap.hasError
                ? ErrorState(error: snap.error!)
                : doc == null
                    ? const CenteredLoader()
                    : Row(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                        if (tocPane) SizedBox(width: 280, child: ReaderToc(doc: doc, controller: _reader)),
                        Expanded(
                          child: Column(children: [
                            if (_search) ReaderSearchBar(doc: doc, controller: _reader, onClose: () => setState(() => _search = false)),
                            Expanded(
                              child: MarkdownReader(
                                doc: doc,
                                controller: _reader,
                                lectureId: widget.lectureId,
                                baseDir: p.posix.dirname(d!.$1.mdPath),
                                initialQuery: widget.query,
                                onRef: (r) => openRef(context, r, lectureId: widget.lectureId),
                                maxWidth: 820,
                              ),
                            ),
                          ]),
                        ),
                      ]),
          );
        });
      },
    ));
  }
}

/// Opens a reference where no supporting pane exists (chat, player panel):
/// timestamps seek, slides and courseware open in a sheet, lectures in the
/// player and library files in the viewer or the system's app.
void openRef(
  BuildContext context,
  CoursiveRef ref, {
  String lectureId = '',
  void Function(TimeRef)? onSeek,
  void Function(String, int)? onPlay,
}) {
  if (ref is LectureRef) {
    Navigator.of(context, rootNavigator: true).push(MaterialPageRoute(builder: (_) => PlayerPage(lectureId: ref.lectureId)));
    return;
  }
  if (ref is NoteRef) {
    Navigator.of(context, rootNavigator: true).push(MaterialPageRoute(builder: (_) => NotePage(noteId: ref.noteId)));
    return;
  }
  if (ref is CanvasRef) {
    Navigator.of(context, rootNavigator: true).push(MaterialPageRoute(builder: (_) => PlayerPage(lectureId: ref.lectureId, canvasCardId: ref.cardId)));
    return;
  }
  if (ref is FileRef) {
    openLibraryFile(context, ref.path);
    return;
  }
  if (ref is TimeRef && ref.quote == null) {
    if (onSeek != null) {
      onSeek(ref);
    } else {
      seekOrOpen(context, ref.lectureId.isNotEmpty ? ref.lectureId : lectureId, ref.seconds);
    }
    return;
  }
  showModalBottomSheet(
    context: context,
    useRootNavigator: true,
    isScrollControlled: true,
    showDragHandle: true,
    useSafeArea: true,
    constraints: const BoxConstraints(maxWidth: 720),
    builder: (sheet) => FractionallySizedBox(
      heightFactor: 0.85,
      child: SourcePane(
        ref: ref,
        lectureId: lectureId,
        onPlay: onPlay == null ? null : (id, ms) {
          Navigator.pop(sheet);
          onPlay(id, ms);
        },
      ),
    ),
  );
}

/// A note read inside a narrow pane (the player's side panel): contents in
/// a menu, timestamps seek the player in place.
class NoteInlineReader extends StatefulWidget {
  const NoteInlineReader({super.key, required this.note, required this.onBack, required this.onEdit, this.onSeek});
  final Note note;
  final VoidCallback onBack, onEdit;
  final void Function(TimeRef)? onSeek;
  @override
  State<NoteInlineReader> createState() => _NoteInlineReaderState();
}

class _NoteInlineReaderState extends State<NoteInlineReader> {
  late MdDoc _doc = MdDoc(widget.note.body);
  final _reader = MdReaderController();

  @override
  void didUpdateWidget(NoteInlineReader old) {
    super.didUpdateWidget(old);
    if (old.note.body != widget.note.body) _doc = MdDoc(widget.note.body);
  }

  @override
  void dispose() {
    _reader.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final n = widget.note;
    final tt = Theme.of(context).textTheme;
    // Reading beside the player counts like the full note page.
    return ActivityRegion(kind: 'read', lectureId: n.lectureId, child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
      Padding(
        padding: const EdgeInsets.fromLTRB(Space.xs, Space.xs, Space.xs, 0),
        child: Row(children: [
          IconButton(tooltip: l10n.commonBack, onPressed: widget.onBack, icon: const Icon(Icons.arrow_back_rounded)),
          Expanded(child: Text(n.title, maxLines: 1, overflow: TextOverflow.ellipsis, style: tt.titleMedium)),
          if (_doc.headings.isNotEmpty)
            MenuAnchor(
              menuChildren: [
                for (final h in _doc.headings)
                  MenuItemButton(
                    onPressed: () => _reader.jumpToHeading(h),
                    child: Padding(
                      padding: EdgeInsets.only(left: (h.level - 2).clamp(0, 3) * 12.0),
                      child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 260), child: Text(h.title, overflow: TextOverflow.ellipsis)),
                    ),
                  ),
              ],
              builder: (context, m, _) => IconButton(tooltip: l10n.commonContents, onPressed: () => m.isOpen ? m.close() : m.open(), icon: const Icon(Icons.toc_rounded)),
            ),
          IconButton(tooltip: l10n.commonEdit, onPressed: widget.onEdit, icon: const Icon(Icons.edit_outlined)),
          IconButton(
            tooltip: l10n.readerOpen,
            onPressed: () => Navigator.of(context, rootNavigator: true).push(MaterialPageRoute(builder: (_) => NotePage(noteId: n.id))),
            icon: const Icon(Icons.open_in_full_rounded),
          ),
        ]),
      ),
      ListenableBuilder(
        listenable: _reader,
        builder: (_, _) => LinearProgressIndicator(value: _reader.progress, minHeight: 2, backgroundColor: Colors.transparent),
      ),
      Expanded(
        child: MarkdownReader(
          doc: _doc,
          controller: _reader,
          lectureId: n.lectureId,
          baseDir: p.posix.dirname(n.path),
          onRef: (r) => openRef(context, r, lectureId: n.lectureId, onSeek: widget.onSeek),
          padding: const EdgeInsets.fromLTRB(Space.sm, Space.md, Space.sm, Space.xxl),
        ),
      ),
    ]));
  }
}
