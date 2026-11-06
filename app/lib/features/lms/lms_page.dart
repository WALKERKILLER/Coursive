import 'dart:async';

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/format.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import '../../widgets/states.dart';
import 'lms_course.dart';

/// 作业与通知: each signed-in learning platform, its sync state, and which
/// library course each platform course belongs to.
/// Contract: docs/specs/0024-source-provider-plugins.md
class LmsPage extends StatefulWidget {
  const LmsPage({super.key, this.api, this.events, this.libraryCourses});

  /// For tests; default to the app's daemon connection and state.
  final ApiCall? api;
  final Stream<Map<String, dynamic>>? events;
  final List<({String id, String title})>? libraryCourses;

  @override
  State<LmsPage> createState() => _LmsPageState();
}

class _LmsPageState extends State<LmsPage> {
  List<Map<String, dynamic>>? _sources;
  Object? _error;
  StreamSubscription? _sub;

  ApiCall get _call => widget.api ?? context.read<AppState>().api.call;

  @override
  void initState() {
    super.initState();
    _load();
    _sub = (widget.events ?? context.read<AppState>().events).listen((e) {
      if (e['type'] == 'lms_changed') _load();
    });
  }

  @override
  void dispose() {
    _sub?.cancel();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final r = lmsMap(await _call('GET', '/api/lms'));
      if (mounted) {
        setState(() {
          _sources = lmsMaps(r['sources']);
          _error = null;
        });
      }
    } catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  Future<void> _sync() async {
    try {
      await _call('POST', '/api/lms/sync');
    } catch (e) {
      if (mounted) showSnack(context, '$e');
    }
  }

  Future<void> _link(
    String source,
    String course, {
    String library = '',
    bool auto = false,
  }) async {
    try {
      final r = lmsMap(
        await _call('PUT', '/api/lms/link', {
          'source': source,
          'course': course,
          'library_course': library,
          'auto': auto,
        }),
      );
      if (mounted) setState(() => _sources = lmsMaps(r['sources']));
    } catch (e) {
      if (mounted) showSnack(context, '$e');
    }
  }

  List<({String id, String title})> _library() =>
      widget.libraryCourses ??
      [
        for (final c in context.read<AppState?>()?.courses ?? const [])
          (id: c.id, title: c.title),
      ];

  @override
  Widget build(BuildContext context) {
    final ss = _sources;
    final pad = context.isCompact ? Space.lg : Space.xl;
    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.lmsTitle),
        actions: [
          if (ss != null && ss.any((s) => s['signed_in'] == true))
            IconButton(
              tooltip: l10n.commonSyncNow,
              onPressed: ss.any((s) => s['syncing'] == true) ? null : _sync,
              icon: const Icon(Icons.sync_rounded),
            ),
        ],
      ),
      body: ss == null
          ? (_error != null
                ? ErrorState(error: _error!, onRetry: _load)
                : const CenteredLoader())
          : ss.isEmpty
          ? EmptyState(
              icon: Icons.assignment_outlined,
              title: l10n.lmsEmptyTitle,
              body: l10n.lmsEmptyBody,
            )
          : ListView(
              padding: EdgeInsets.fromLTRB(pad, Space.sm, pad, Space.huge),
              children: [
                for (final s in ss)
                  ContentWidth(
                    max: 840,
                    child: Padding(
                      padding: const EdgeInsets.only(bottom: Space.xxl),
                      child: _SourceSection(
                        source: s,
                        library: _library(),
                        onLink: _link,
                      ),
                    ),
                  ),
              ],
            ),
    );
  }
}

typedef _LinkFn =
    Future<void> Function(
      String source,
      String course, {
      String library,
      bool auto,
    });

class _SourceSection extends StatelessWidget {
  const _SourceSection({
    required this.source,
    required this.library,
    required this.onLink,
  });
  final Map<String, dynamic> source;
  final List<({String id, String title})> library;
  final _LinkFn onLink;

  @override
  Widget build(BuildContext context) {
    final tt = Theme.of(context).textTheme;
    final cs = Theme.of(context).colorScheme;
    final syncing = source['syncing'] == true;
    final synced = (source['synced_at'] as num?)?.toInt() ?? 0;
    final err = '${source['error'] ?? ''}';
    final courses = lmsMaps(source['courses']);
    final open = courses.fold<int>(
      0,
      (n, c) => n + ((c['open'] as num?)?.toInt() ?? 0),
    );
    final status = [
      if (source['signed_in'] != true) l10n.lmsSignedOut,
      if (syncing)
        l10n.commonSyncing
      else if (synced > 0)
        l10n.commonLastSynced(
          fmtRelative(DateTime.fromMillisecondsSinceEpoch(synced)),
        )
      else
        l10n.lmsNotSynced,
      l10n.lmsSummary(courses.length, open),
    ].join(' · ');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('${source['title']}', style: tt.titleLarge),
        const SizedBox(height: Space.xs),
        Text(status, style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant)),
        AnimatedSize(
          duration: Motion.medium2,
          curve: Motion.emphasized,
          child: syncing
              ? const Padding(
                  padding: EdgeInsets.only(top: Space.sm),
                  child: LinearProgressIndicator(),
                )
              : const SizedBox(width: double.infinity),
        ),
        if (err.isNotEmpty) ...[
          const SizedBox(height: Space.md),
          Text(l10n.lmsSyncFailed(err), style: tt.bodyMedium?.copyWith(color: cs.error)),
        ],
        const SizedBox(height: Space.lg),
        Text(l10n.lmsCourseLinks, style: tt.titleSmall),
        const SizedBox(height: Space.xs),
        Text(
          l10n.lmsCourseLinksBody,
          style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
        ),
        const SizedBox(height: Space.md),
        if (courses.isEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: Space.xl),
            child: Text(
              syncing ? l10n.lmsLoadingCourses : l10n.lmsNoCourses,
              textAlign: TextAlign.center,
              style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
            ),
          )
        else
          Material(
            color: cs.surfaceContainerLow,
            borderRadius: BorderRadius.circular(Corners.lg),
            clipBehavior: Clip.antiAlias,
            child: Column(
              children: [
                for (final c in courses)
                  _CourseRow(
                    source: '${source['source']}',
                    course: c,
                    library: library,
                    onLink: onLink,
                  ),
              ],
            ),
          ),
      ],
    );
  }
}

class _CourseRow extends StatelessWidget {
  const _CourseRow({
    required this.source,
    required this.course,
    required this.library,
    required this.onLink,
  });
  final String source;
  final Map<String, dynamic> course;
  final List<({String id, String title})> library;
  final _LinkFn onLink;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final id = '${course['id']}';
    final name = '${course['title']}';
    final linked = '${course['library_course'] ?? ''}';
    final manual = course['manual'] == true;
    final title =
        library.where((l) => l.id == linked).map((l) => l.title).firstOrNull ??
        '';
    final label = linked.isEmpty
        ? (manual ? l10n.lmsNoLink : l10n.lmsAutoNoMatch)
        : (manual ? title : l10n.lmsAutoMatch(title));
    final open = (course['open'] as num?)?.toInt() ?? 0;
    final term = '${course['term'] ?? ''}';
    final meta = Text(
      [
        if (term.isNotEmpty) term,
        open > 0 ? l10n.assignmentOpenCount(open) : l10n.lmsNoOpenAssignments,
      ].join(' · '),
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
    );
    // On a phone the link sits under the name, which keeps the row's width.
    final compact = context.isCompact;
    final link = MenuAnchor(
      builder: (context, ctl, _) => ActionChip(
        avatar: Icon(
          linked.isEmpty ? Icons.link_off_rounded : Icons.link_rounded,
          size: 18,
        ),
        label: ConstrainedBox(
          constraints: BoxConstraints(maxWidth: compact ? 220 : 180),
          child: Text(label, overflow: TextOverflow.ellipsis),
        ),
        tooltip: l10n.lmsLinkCourse,
        onPressed: () => ctl.isOpen ? ctl.close() : ctl.open(),
      ),
      menuChildren: [
        MenuItemButton(
          leadingIcon: const Icon(Icons.auto_awesome_outlined),
          onPressed: () => onLink(source, id, auto: true),
          child: Text(l10n.lmsAutoLink),
        ),
        MenuItemButton(
          leadingIcon: const Icon(Icons.link_off_rounded),
          onPressed: () => onLink(source, id),
          child: Text(l10n.lmsNoLink),
        ),
        if (library.isNotEmpty) const Divider(),
        for (final l in library)
          MenuItemButton(
            leadingIcon: Icon(
              l.id == linked && manual
                  ? Icons.check_rounded
                  : Icons.class_outlined,
            ),
            onPressed: () => onLink(source, id, library: l.id),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 280),
              child: Text(l.title, overflow: TextOverflow.ellipsis),
            ),
          ),
      ],
    );
    return ListTile(
      contentPadding: EdgeInsets.fromLTRB(
        Space.lg,
        Space.xs,
        compact ? Space.lg : Space.sm,
        Space.xs,
      ),
      leading: CircleAvatar(
        backgroundColor: cs.tertiaryContainer,
        child: Text(
          name.characters.firstOrNull ?? '?',
          style: TextStyle(color: cs.onTertiaryContainer),
        ),
      ),
      title: Text(
        name,
        maxLines: compact ? 2 : 1,
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: compact
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [meta, const SizedBox(height: Space.xs), link],
            )
          : meta,
      onTap: () => showLmsCourseSheet(context, source, id, name),
      trailing: compact ? null : link,
    );
  }
}
