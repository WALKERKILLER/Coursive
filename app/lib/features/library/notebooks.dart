import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/app_state.dart';
import '../../l10n/l10n.dart';
import '../../theme/prefs.dart';
import '../../widgets/states.dart';
import '../study_canvas/study_canvas.dart';
import '../study_canvas/study_canvas_surface.dart';
import 'note_reader.dart';

// Independent documents share the editor and identity across both entry points.
// Design: docs/decisions/0038-independent-canvas-documents.md.
Future<void> createDocument(BuildContext context, String courseId) async {
  final canvas = await showModalBottomSheet<bool>(
    context: context,
    showDragHandle: true,
    builder: (sheet) => SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            leading: const Icon(Icons.edit_note),
            title: Text(l10n.documentMarkdown),
            onTap: () => Navigator.pop(sheet, false),
          ),
          ListTile(
            leading: const Icon(Icons.gesture),
            title: Text(l10n.documentCanvas),
            onTap: () => Navigator.pop(sheet, true),
          ),
        ],
      ),
    ),
  );
  if (canvas == null || !context.mounted) return;
  if (!canvas) {
    await Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => NotePage(courseId: courseId)),
    );
    return;
  }
  await createBlankCanvas(context, courseId);
}

Future<void> createBlankCanvas(BuildContext context, String courseId) async {
  String title = '';
  final name = await showDialog<String>(
    context: context,
    builder: (dialog) => StatefulBuilder(
      builder: (_, update) => AlertDialog(
        title: Text(l10n.documentCanvasTitle),
        content: TextField(
          autofocus: true,
          maxLength: 100,
          onChanged: (v) => update(() => title = v.trim()),
          onSubmitted: (_) {
            if (title.isNotEmpty) Navigator.pop(dialog, title);
          },
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialog),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: title.isEmpty
                ? null
                : () => Navigator.pop(dialog, title),
            child: Text(l10n.documentCreate),
          ),
        ],
      ),
    ),
  );
  if (name == null || !context.mounted) return;
  try {
    final created = await context.read<AppState>().api.post('/api/notebooks', {
      'title': name,
      'course_id': courseId,
    });
    if (context.mounted)
      await openNotebook(context, created['id'] as String, name);
  } catch (e) {
    if (context.mounted) showSnack(context, '$e');
  }
}

Future<void> openNotebook(BuildContext context, String id, String title) =>
    Navigator.of(context, rootNavigator: true).push(
      MaterialPageRoute(
        builder: (_) => _NotebookPage(id: id, title: title),
      ),
    );

class _NotebookPage extends StatefulWidget {
  const _NotebookPage({required this.id, required this.title});
  final String id, title;
  @override
  State<_NotebookPage> createState() => _NotebookPageState();
}

class _NotebookPageState extends State<_NotebookPage> {
  final canvas = GlobalKey<StudyCanvasState>();
  bool leaving = false;
  Future<void> close() async {
    if (leaving || (canvas.currentState?.returnFromSource() ?? false)) return;
    leaving = true;
    if (await canvas.currentState?.flush() == false) {
      leaving = false;
      return;
    }
    if (mounted) Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) => PopScope(
    canPop: false,
    onPopInvokedWithResult: (didPop, _) {
      if (!didPop) close();
    },
    child: Scaffold(
      body: StudyCanvasSurface(
        prefs: context.watch<UiPrefs>(),
        child: StudyCanvas.notebook(
          pdfLibraryRoot: context.read<AppState>().daemon.libraryRoot,
          key: canvas,
          api: context.read<AppState>().api,
          id: widget.id,
          name: widget.title,
          onBack: close,
        ),
      ),
    ),
  );
}

class CourseDocuments extends StatefulWidget {
  const CourseDocuments({super.key, required this.courseId});
  final String courseId;
  @override
  State<CourseDocuments> createState() => _CourseDocumentsState();
}

class _CourseDocumentsState extends State<CourseDocuments> {
  late Future<List<Map>> documents;
  StreamSubscription? events;
  @override
  void initState() {
    super.initState();
    documents = load();
    events = context.read<AppState>().events.listen((e) {
      if (e['type'] == 'notes_changed' || e['type'] == 'sync_done')
        setState(() => documents = load());
    });
  }

  Future<List<Map>> load() async {
    final api = context.read<AppState>().api;
    final q = {'course_id': widget.courseId};
    final lists = await Future.wait([
      api.get('/api/notes', q),
      api.get('/api/notebooks', q),
    ]);
    return [
      for (final n in lists[0] as List? ?? [])
        if ((n['lecture_id'] ?? '') == '') {...n as Map, 'canvas': false},
      for (final n in lists[1] as List? ?? []) {...n as Map, 'canvas': true},
    ];
  }

  @override
  void dispose() {
    events?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Column(
    children: [
      Row(
        children: [
          Expanded(
            child: Text(
              l10n.courseDocuments,
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ),
          TextButton.icon(
            icon: const Icon(Icons.add),
            label: Text(l10n.documentCreate),
            onPressed: () => createDocument(context, widget.courseId),
          ),
        ],
      ),
      FutureBuilder<List<Map>>(
        future: documents,
        builder: (_, snap) {
          if (snap.hasError)
            return TextButton(
              onPressed: () => setState(() => documents = load()),
              child: Text(l10n.commonRetry),
            );
          return Column(
            children: [
              for (final n in snap.data ?? <Map>[])
                ListTile(
                  dense: true,
                  leading: Icon(
                    n['canvas'] == true ? Icons.gesture : Icons.edit_note,
                  ),
                  title: Text(n['title'] as String),
                  onTap: () => n['canvas'] == true
                      ? openNotebook(context, n['id'], n['title'])
                      : Navigator.push(
                          context,
                          MaterialPageRoute(
                            builder: (_) => NotePage(noteId: n['id']),
                          ),
                        ),
                ),
            ],
          );
        },
      ),
    ],
  );
}
