import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/models.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import '../../widgets/adaptive_sheet.dart';
import '../../widgets/pickers.dart';
import '../../widgets/states.dart';
import '../lms/lms_page.dart';
import '../plugins/plugins_page.dart' show PluginGlyph;
import 'sign_in_sheet.dart';

/// 课程 → 来源: installed source plugins, each source's sign-in state,
/// installing a plugin from a folder and removing one.
/// Contract: docs/specs/0024-source-provider-plugins.md
class SourcesPage extends StatelessWidget {
  const SourcesPage({super.key});

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();
    final tt = Theme.of(context).textTheme;
    final cs = Theme.of(context).colorScheme;
    final pad = context.isCompact ? Space.lg : Space.xl;
    return Scaffold(
      appBar: AppBar(title: Text(l10n.sourcesTitle)),
      body: RefreshIndicator(
        onRefresh: app.loadSources,
        child: app.plugins.isEmpty
            ? LayoutBuilder(
                builder: (context, c) => SingleChildScrollView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  child: ConstrainedBox(
                    constraints: BoxConstraints(minHeight: c.maxHeight),
                    child: EmptyState(
                      icon: Icons.extension_outlined,
                      title: l10n.sourcesEmptyTitle,
                      body: l10n.sourcesEmptyBody,
                      action: FilledButton.icon(
                        onPressed: () => installSourcePlugin(context),
                        icon: const Icon(Icons.create_new_folder_outlined),
                        label: Text(l10n.sourcesInstall),
                      ),
                    ),
                  ),
                ),
              )
            : ListView(
                padding: EdgeInsets.fromLTRB(pad, Space.sm, pad, Space.huge),
                children: [
                  ContentWidth(
                    max: 840,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          l10n.sourcesIntro,
                          style: tt.bodyMedium?.copyWith(
                            color: cs.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: Space.xl),
                        for (final p in app.plugins) ...[
                          _PluginCard(plugin: p),
                          const SizedBox(height: Space.lg),
                        ],
                        Align(
                          alignment: AlignmentDirectional.centerStart,
                          child: OutlinedButton.icon(
                            onPressed: () => installSourcePlugin(context),
                            icon: const Icon(Icons.create_new_folder_outlined),
                            label: Text(l10n.sourcesInstall),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
      ),
    );
  }
}

/// Installs a source plugin from a folder after the user reviews what it
/// may reach, then offers to sign its first source in.
Future<void> installSourcePlugin(BuildContext context, {ApiCall? api}) async {
  final app = context.read<AppState>();
  final call = api ?? app.api.call;
  final dir = await pickDirectory(
    context,
    title: l10n.sourcesChooseFolder,
    confirm: l10n.commonChoose,
  );
  if (dir == null || !context.mounted) return;
  Map<String, dynamic> preview;
  try {
    preview = ((await call('POST', '/api/sources/preview', {'path': dir})) as Map)
        .cast<String, dynamic>();
  } catch (e) {
    if (context.mounted) showSnack(context, l10n.sourcesCannotRead('$e'));
    return;
  }
  if (!context.mounted) return;
  await _consentAndInstall(
    context,
    SourcePlugin(preview),
    () => call('POST', '/api/sources/install', {'path': dir}),
  );
}

/// Installs source plugin [id] from the plugin index: the daemon downloads
/// and verifies it, the user reviews what it may reach, then it is moved
/// into place. [onStaged] runs once the download is verified or failed.
/// Returns whether it was installed.
Future<bool> installListedSource(
  BuildContext context,
  String id, {
  ApiCall? api,
  VoidCallback? onStaged,
}) async {
  final call = api ?? context.read<AppState>().api.call;
  Map<String, dynamic> staged;
  try {
    staged = ((await call('POST', '/api/sources/market/$id/stage')) as Map)
        .cast<String, dynamic>();
  } catch (e) {
    if (context.mounted) showSnack(context, l10n.sourcesCannotInstall('$e'));
    return false;
  } finally {
    onStaged?.call();
  }
  if (!context.mounted) return false;
  return _consentAndInstall(
    context,
    SourcePlugin((staged['plugin'] as Map).cast<String, dynamic>()),
    () => call('POST', '/api/sources/market/install', {
      'token': staged['token'],
    }),
  );
}

/// Shows what [p] may reach, runs [install] once the user agrees, then
/// offers to sign its first source in.
Future<bool> _consentAndInstall(
  BuildContext context,
  SourcePlugin p,
  Future<void> Function() install,
) async {
  final app = context.read<AppState>();
  final ok = await showAdaptiveSheet<bool>(
    context,
    builder: (c) => _Consent(plugin: p),
  );
  if (ok != true || !context.mounted) return false;
  try {
    await install();
    await app.loadSources();
  } catch (e) {
    if (context.mounted) showSnack(context, l10n.sourcesCannotInstall('$e'));
    return false;
  }
  if (!context.mounted) return true;
  showSnack(context, l10n.sourcesInstalled(p.title));
  final first = app.plugins
      .where((x) => x.id == p.id)
      .expand((x) => x.sources)
      .firstOrNull;
  if (first != null && first.fields.isNotEmpty && !first.signedIn) {
    await signInSource(context, first);
  }
  return true;
}

/// Uninstalls a source plugin after confirmation. Returns whether it was
/// removed.
Future<bool> removeSourcePlugin(BuildContext context, SourcePlugin plugin) async {
  final app = context.read<AppState>();
  final ok = await showDialog<bool>(
    context: context,
    builder: (c) => AlertDialog(
      icon: const Icon(Icons.delete_outline_rounded),
      title: Text(l10n.sourcesRemoveTitle(plugin.title)),
      content: Text(l10n.sourcesRemoveBody),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(c, false),
          child: Text(l10n.commonCancel),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(c, true),
          child: Text(l10n.sourcesRemovePlugin),
        ),
      ],
    ),
  );
  if (ok != true) return false;
  try {
    await app.api.delete('/api/sources/${plugin.id}');
    await app.loadSources();
    await app.loadCourses(quiet: true);
  } catch (e) {
    if (context.mounted) showSnack(context, '$e');
    return false;
  }
  return true;
}

/// Signs a source in and syncs its courses.
Future<void> signInSource(BuildContext context, Source s) async {
  final app = context.read<AppState>();
  if (!await showSignInSheet(context, s) || !context.mounted) return;
  showSnack(context, l10n.sourceSignedIn(s.title));
  await app.loadSources();
  await app.loadCourses(sync: true);
}

/// What a plugin may do, shown before it is installed.
class _Consent extends StatelessWidget {
  const _Consent({required this.plugin});
  final SourcePlugin plugin;

  @override
  Widget build(BuildContext context) {
    final tt = Theme.of(context).textTheme;
    final cs = Theme.of(context).colorScheme;
    return SingleChildScrollView(
      padding: sheetPadding(context),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              PluginGlyph(icon: plugin.icon.isEmpty ? 'school' : plugin.icon, id: plugin.id, size: 48),
              const SizedBox(width: Space.lg),
              Expanded(
                child: Text(l10n.sourcesConsentTitle(plugin.title), style: tt.headlineSmall),
              ),
            ],
          ),
          const SizedBox(height: Space.xs),
          Text(
            l10n.sourcesVersion(plugin.version),
            style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
          ),
          if (plugin.summary.isNotEmpty) ...[
            const SizedBox(height: Space.lg),
            Text(plugin.summary, style: tt.bodyLarge),
          ],
          const SizedBox(height: Space.xl),
          Text(l10n.sourcesConsentHosts, style: tt.titleSmall),
          const SizedBox(height: Space.sm),
          Wrap(
            spacing: Space.sm,
            runSpacing: Space.sm,
            children: [
              for (final h in plugin.hosts)
                Chip(
                  avatar: const Icon(Icons.language_rounded, size: 18),
                  label: Text(h),
                ),
            ],
          ),
          const SizedBox(height: Space.xl),
          Container(
            padding: const EdgeInsets.all(Space.lg),
            decoration: BoxDecoration(
              color: cs.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(Corners.lg),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.shield_outlined, color: cs.onSurfaceVariant),
                const SizedBox(width: Space.md),
                Expanded(
                  child: Text(l10n.sourcesConsentBody, style: tt.bodyMedium),
                ),
              ],
            ),
          ),
          if (!plugin.available) ...[
            const SizedBox(height: Space.lg),
            InlineError(text: l10n.sourcesUnavailable),
          ],
          const SizedBox(height: Space.xl),
          Row(
            children: [
              const Spacer(),
              TextButton(
                onPressed: () => Navigator.pop(context, false),
                child: Text(l10n.commonCancel),
              ),
              const SizedBox(width: Space.sm),
              FilledButton(
                onPressed: plugin.available
                    ? () => Navigator.pop(context, true)
                    : null,
                child: Text(l10n.sourcesInstallPlugin),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _PluginCard extends StatelessWidget {
  const _PluginCard({required this.plugin});
  final SourcePlugin plugin;

  Future<void> _menu(BuildContext context, String v) async {
    final app = context.read<AppState>();
    try {
      switch (v) {
        case 'toggle':
          await app.api.put('/api/sources/${plugin.id}', {
            'enabled': !plugin.enabled,
          });
        case 'remove':
          await removeSourcePlugin(context, plugin);
          return;
      }
      await app.loadSources();
    } catch (e) {
      if (context.mounted) showSnack(context, '$e');
    }
  }

  @override
  Widget build(BuildContext context) {
    final tt = Theme.of(context).textTheme;
    final cs = Theme.of(context).colorScheme;
    return Card.filled(
      margin: EdgeInsets.zero,
      color: cs.surfaceContainerLow,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(Corners.xl),
      ),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(
          Space.xl,
          Space.lg,
          Space.sm,
          Space.lg,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                PluginGlyph(icon: plugin.icon.isEmpty ? 'school' : plugin.icon, id: plugin.id, size: 48),
                const SizedBox(width: Space.lg),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(plugin.title, style: tt.titleLarge),
                      const SizedBox(height: Space.xxs),
                      Text(
                        [
                          l10n.sourcesVersion(plugin.version),
                          if (!plugin.enabled) l10n.sourcesDisabled,
                        ].join(' · '),
                        style: tt.bodySmall?.copyWith(
                          color: cs.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
                PopupMenuButton<String>(
                  tooltip: l10n.commonMore,
                  onSelected: (v) => _menu(context, v),
                  itemBuilder: (_) => [
                    PopupMenuItem(
                      value: 'toggle',
                      child: Text(
                        plugin.enabled ? l10n.commonDisable : l10n.commonEnable,
                      ),
                    ),
                    PopupMenuItem(
                      value: 'remove',
                      child: Text(l10n.sourcesRemovePlugin),
                    ),
                  ],
                ),
              ],
            ),
            if (plugin.summary.isNotEmpty) ...[
              const SizedBox(height: Space.sm),
              Padding(
                padding: const EdgeInsetsDirectional.only(end: Space.lg),
                child: Text(
                  plugin.summary,
                  style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
                ),
              ),
            ],
            if (plugin.enabled)
              for (final s in plugin.sources) ...[
                const SizedBox(height: Space.md),
                _SourceRow(source: s),
              ],
          ],
        ),
      ),
    );
  }
}

class _SourceRow extends StatelessWidget {
  const _SourceRow({required this.source});
  final Source source;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final s = source;
    final failed = s.process == 'error';
    final (icon, color, status) = switch (s) {
      _ when failed => (
        Icons.error_outline_rounded,
        cs.error,
        l10n.sourceFailed(s.error),
      ),
      _ when s.expired => (
        Icons.lock_clock_outlined,
        cs.error,
        l10n.sourceExpired,
      ),
      _ when s.signedIn => (
        Icons.check_circle_rounded,
        cs.primary,
        s.account.isEmpty ? l10n.sourceSignedInPlain : l10n.sourceSignedInAs(s.account),
      ),
      _ => (
        Icons.radio_button_unchecked_rounded,
        cs.onSurfaceVariant,
        l10n.sourceSignedOut,
      ),
    };
    final app = context.read<AppState>();
    final Widget action;
    if (failed) {
      action = FilledButton.tonal(
        onPressed: () async {
          try {
            await app.api.post('/api/sources/${s.id}/restart');
          } catch (e) {
            if (context.mounted) showSnack(context, '$e');
          }
          await app.loadSources();
        },
        child: Text(l10n.sourceRestart),
      );
    } else if (s.signedIn) {
      action = TextButton(
        onPressed: () async {
          try {
            await app.api.post('/api/sources/${s.id}/sign-out');
            await app.loadSources();
          } catch (e) {
            if (context.mounted) showSnack(context, '$e');
          }
        },
        child: Text(l10n.sourceSignOut),
      );
    } else {
      action = FilledButton.tonal(
        onPressed: s.fields.isEmpty ? null : () => signInSource(context, s),
        child: Text(s.expired ? l10n.loginSignInAgain : l10n.loginSignIn),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ListTile(
          contentPadding: const EdgeInsetsDirectional.only(end: Space.sm),
          leading: Icon(icon, color: color),
          title: Text(s.title),
          subtitle: Text(
            status,
            style: failed || s.expired ? TextStyle(color: cs.error) : null,
          ),
          trailing: action,
        ),
        if (s.signedIn && s.hasLms)
          Padding(
            padding: const EdgeInsetsDirectional.only(start: 56),
            child: Align(
              alignment: AlignmentDirectional.centerStart,
              child: TextButton.icon(
                onPressed: () => Navigator.of(context).push(
                  MaterialPageRoute(builder: (_) => const LmsPage()),
                ),
                icon: const Icon(Icons.assignment_outlined),
                label: Text(l10n.lmsTitle),
              ),
            ),
          ),
      ],
    );
  }
}
