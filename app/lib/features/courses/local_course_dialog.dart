import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/format.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import '../../widgets/adaptive_sheet.dart';
import 'course_page.dart';

/// The largest video the daemon imports.
const _maxVideo = 4 << 30;

/// Creates a course on this device, or with [courseId] adds videos to one.
/// Returns the course id once the course exists, even if an import failed or
/// the window was closed afterwards.
Future<String?> openLocalCourseDialog(
  BuildContext context,
  Api api, {
  String? courseId,
}) => showAdaptiveSheet<String>(
  context,
  draggable: false,
  builder: (_) => LocalCourseDialog(api: api, courseId: courseId),
);

/// Creates a course on this device and opens it. [onCreated] runs once the
/// course exists, before it opens.
Future<void> startLocalCourse(
  BuildContext context, {
  ValueChanged<String>? onCreated,
}) async {
  final app = context.read<AppState>();
  final id = await openLocalCourseDialog(context, app.api);
  if (!context.mounted || id == null) return;
  onCreated?.call(id);
  await app.loadCourses();
  if (!context.mounted) return;
  await Navigator.of(context)
      .push(MaterialPageRoute(builder: (_) => CoursePage(courseId: id)));
}

class LocalCourseDialog extends StatefulWidget {
  const LocalCourseDialog({
    super.key,
    required this.api,
    this.courseId,
    this.pickFiles,
  });
  final Api api;
  final String? courseId;
  final Future<List<XFile>> Function()? pickFiles;
  @override
  State<LocalCourseDialog> createState() => _LocalCourseDialogState();
}

class _LocalCourseDialogState extends State<LocalCourseDialog> {
  final _title = TextEditingController();
  final _teacher = TextEditingController();
  final _form = GlobalKey<FormState>();
  List<(XFile, int)> _files = [];
  late String? _course = widget.courseId;
  bool _busy = false;
  int _done = 0;
  String? _error;

  bool get _creating => widget.courseId == null;
  bool get _tooLarge => _files.any((f) => f.$2 > _maxVideo);

  @override
  void dispose() {
    _title.dispose();
    _teacher.dispose();
    super.dispose();
  }

  Future<void> _pick() async {
    try {
      final files =
          await (widget.pickFiles?.call() ??
              openFiles(
                acceptedTypeGroups: [
                  const XTypeGroup(
                    label: 'Video',
                    extensions: ['mp4', 'mov', 'm4v', 'mkv', 'webm'],
                  ),
                ],
              ));
      if (!mounted || files.isEmpty) return;
      final sized = [for (final f in files) (f, await f.length())]
        ..sort((a, b) => a.$1.name.compareTo(b.$1.name));
      if (!mounted) return;
      setState(() {
        _files = sized;
        _done = 0;
        _error = null;
      });
    } catch (e) {
      if (mounted) setState(() => _error = l10n.localVideosPickFailed('$e'));
    }
  }

  Future<void> _save() async {
    if (_busy || _tooLarge || !_form.currentState!.validate()) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      if (_course == null) {
        try {
          final course = await widget.api.createLocalCourse(
            _title.text.trim(),
            teacher: _teacher.text.trim(),
          );
          _course = '${course['id']}';
        } catch (e) {
          if (mounted)
            setState(() => _error = l10n.localCourseCreateFailed('$e'));
          return;
        }
        if (!mounted) return;
      }
      while (_done < _files.length) {
        final (file, length) = _files[_done];
        await widget.api.importLocalVideo(
          _course!,
          file.name,
          file.openRead(),
          length,
        );
        if (!mounted) return;
        setState(() => _done++);
      }
      if (mounted) Navigator.pop(context, _course);
    } catch (e) {
      if (mounted) {
        setState(
          () =>
              _error = l10n.localVideoImportFailed(_files[_done].$1.name, '$e'),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  String get _submitLabel {
    if (_error != null) {
      return _course != null && _done < _files.length
          ? l10n.localVideosResume
          : l10n.commonRetry;
    }
    if (_course == null) {
      return _files.isEmpty
          ? l10n.localCourseSubmit
          : l10n.localCourseSubmitWithVideos(_files.length);
    }
    return l10n.localVideosSubmit(_files.length);
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final locked = _busy || _course != null;
    final muted = tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant);
    // Esc, back and the barrier close the window unless videos are uploading,
    // and still report a course that already exists.
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop && !_busy) Navigator.pop(context, _course);
      },
      child: SingleChildScrollView(
        padding: sheetPadding(context, handle: false),
        child: Form(
          key: _form,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                _creating ? l10n.localCourseCreate : l10n.localVideosAdd,
                style: tt.headlineSmall,
              ),
              const SizedBox(height: Space.sm),
              Text(
                _creating
                    ? l10n.localCourseCreateBody
                    : l10n.localVideosSectionHint,
                style: muted,
              ),
              const SizedBox(height: Space.xl),
              if (_creating) ...[
                TextFormField(
                  controller: _title,
                  autofocus: true,
                  enabled: !locked,
                  maxLength: 200,
                  textInputAction: TextInputAction.next,
                  decoration: InputDecoration(
                    labelText: l10n.localCourseName,
                    hintText: l10n.localCourseNameHint,
                    border: const OutlineInputBorder(),
                  ),
                  validator: (v) => v == null || v.trim().isEmpty
                      ? l10n.localCourseNameRequired
                      : null,
                ),
                const SizedBox(height: Space.sm),
                TextFormField(
                  controller: _teacher,
                  enabled: !locked,
                  maxLength: 100,
                  decoration: InputDecoration(
                    labelText: l10n.localCourseTeacher,
                    border: const OutlineInputBorder(),
                  ),
                ),
                const SizedBox(height: Space.lg),
                Text(l10n.localVideosSection, style: tt.titleSmall),
                const SizedBox(height: Space.xs),
                Text(
                  l10n.localVideosSectionHint,
                  style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
                ),
                const SizedBox(height: Space.md),
              ],
              if (_files.isEmpty)
                Align(
                  alignment: AlignmentDirectional.centerStart,
                  child: OutlinedButton.icon(
                    onPressed: _busy ? null : _pick,
                    icon: const Icon(Icons.video_library_outlined),
                    label: Text(l10n.localVideosChoose),
                  ),
                )
              else ...[
                _FileList(
                  files: _files,
                  done: _done,
                  busy: _busy,
                  onRemove: _busy || _done > 0
                      ? null
                      : (i) => setState(() {
                          _files = [..._files]..removeAt(i);
                          _error = null;
                        }),
                ),
                const SizedBox(height: Space.sm),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        l10n.localVideosSummary(
                          _files.length,
                          fmtBytes(_files.fold<int>(0, (s, f) => s + f.$2)),
                        ),
                        style: tt.bodySmall?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      ),
                    ),
                    if (_done == 0)
                      TextButton(
                        onPressed: _busy ? null : _pick,
                        child: Text(l10n.localVideosRechoose),
                      ),
                  ],
                ),
              ],
              if (_busy && _files.isNotEmpty) ...[
                const SizedBox(height: Space.md),
                LinearProgressIndicator(value: _done / _files.length),
                const SizedBox(height: Space.sm),
                Text(
                  l10n.localVideosProgress(_done, _files.length),
                  style: muted,
                ),
              ],
              if (_error != null) ...[
                const SizedBox(height: Space.lg),
                InlineError(text: _error!),
                if (_course != null && _done < _files.length) ...[
                  const SizedBox(height: Space.sm),
                  Text(
                    l10n.localVideosPartialSaved,
                    style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
                  ),
                ],
              ],
              const SizedBox(height: Space.xl),
              Wrap(
                alignment: WrapAlignment.end,
                spacing: Space.sm,
                runSpacing: Space.sm,
                children: [
                  TextButton(
                    onPressed: _busy
                        ? null
                        : () => Navigator.pop(context, _course),
                    child: Text(
                      _creating && _course != null
                          ? l10n.localVideosLater
                          : l10n.commonCancel,
                    ),
                  ),
                  FilledButton(
                    onPressed:
                        _busy ||
                            _tooLarge ||
                            (_course != null && _files.isEmpty)
                        ? null
                        : _save,
                    child: Text(_submitLabel),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The chosen videos in import order, each marked with its state.
class _FileList extends StatelessWidget {
  const _FileList({
    required this.files,
    required this.done,
    required this.busy,
    required this.onRemove,
  });
  final List<(XFile, int)> files;
  final int done;
  final bool busy;
  final void Function(int)? onRemove;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    return Container(
      constraints: const BoxConstraints(maxHeight: 248),
      decoration: BoxDecoration(
        border: Border.all(color: cs.outlineVariant),
        borderRadius: BorderRadius.circular(Corners.md),
      ),
      child: ListView.builder(
        shrinkWrap: true,
        padding: const EdgeInsets.symmetric(vertical: Space.xs),
        itemCount: files.length,
        itemBuilder: (context, i) {
          final (file, size) = files[i];
          final big = size > _maxVideo;
          final Widget lead;
          if (i < done) {
            lead = Icon(Icons.check_circle_rounded, color: cs.primary);
          } else if (busy && i == done) {
            lead = const SizedBox.square(
              dimension: 20,
              child: CircularProgressIndicator(strokeWidth: 2.5),
            );
          } else if (big) {
            lead = Icon(Icons.error_outline_rounded, color: cs.error);
          } else {
            lead = Text(
              '${i + 1}',
              style: tt.labelLarge?.copyWith(color: cs.onSurfaceVariant),
            );
          }
          return ListTile(
            dense: true,
            leading: SizedBox.square(dimension: 24, child: Center(child: lead)),
            title: Text(
              file.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            subtitle: Text(
              big ? l10n.localVideoLimit : fmtBytes(size),
              style: big ? TextStyle(color: cs.error) : null,
            ),
            trailing: onRemove == null
                ? null
                : IconButton(
                    tooltip: l10n.commonRemove,
                    icon: const Icon(Icons.close_rounded),
                    onPressed: () => onRemove!(i),
                  ),
          );
        },
      ),
    );
  }
}
