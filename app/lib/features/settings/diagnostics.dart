import 'dart:io';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';

import '../../core/app_state.dart';
import '../../core/daemon_service.dart';
import '../../core/open_path.dart';
import '../../theme/tokens.dart';
import '../../widgets/states.dart';
import '../assistant/runtime_setup.dart';
import 'models_page.dart';
import 'settings_page.dart';
import '../setup/media_tools_help.dart';
import '../setup/workspace_setup_page.dart';
import '../../l10n/l10n.dart';

/// 设置 → 诊断: whether the background service, transcription and the
/// assistant work, where things are, the logs, and a restart.
class Diagnostics extends StatefulWidget {
  const Diagnostics({super.key});
  @override
  State<Diagnostics> createState() => _DiagnosticsState();
}

enum _Health { ok, attention, pending }

class _Check {
  const _Check(this.icon, this.title, this.detail, this.health, {this.action});
  final IconData icon;
  final String title, detail;
  final _Health health;
  final Widget? action;
}

class _DiagnosticsState extends State<Diagnostics> {
  Map<String, dynamic>? _runtime;
  List<String> _agentLog = const [];
  Map? _tools, _models, _asr;
  bool _loading = false;
  String? _pdfBackend;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final app = context.read<AppState>();
    if (!app.connected || _loading) return;
    setState(() {
      _loading = true;
      _runtime = null;
      _tools = _models = _asr = null;
      _pdfBackend = null;
    });
    // One unavailable service must not hide the other preparation checks.
    Future<Map?> read(String path) async {
      try {
        return await app.api.get(path).timeout(const Duration(seconds: 12))
            as Map;
      } catch (_) {
        return null;
      }
    }

    final results = await Future.wait([
      read('/api/health'),
      read('/api/agent/runtime'),
      read('/api/agent/status'),
      read('/agent/models'),
      read('/api/asr'),
    ]);
    if (!mounted) return;
    setState(() {
      _tools = results[0]?['media_tools'] as Map?;
      _pdfBackend = results[0]?['pdf_backend'] as String?;
      _runtime = results[1]?.cast<String, dynamic>();
      _agentLog = [
        for (final l in (results[2]?['logs'] as List?) ?? const []) '$l',
      ];
      _models = results[3];
      _asr = results[4];
      _loading = false;
    });
    await app.refreshHealth();
  }

  static String get _platform => kIsWeb
      ? 'Web'
      : switch (Platform.operatingSystem) {
          'linux' => 'Linux',
          'windows' => 'Windows',
          'macos' => 'macOS',
          'android' => 'Android',
          final os => os,
        };

  List<_Check> _checks(AppState app, DaemonService daemon) {
    final h = app.health;
    final agent = app.agent;
    final node = (_runtime?['node'] as Map?)?.cast<String, dynamic>();
    final ready = daemon.state == DaemonState.ready;
    final agentState = '${agent['state']}';
    final agentError = '${agent['error'] ?? ''}'.trim();
    return [
      _Check(
        Icons.dns_outlined,
        l10n.diagDaemon,
        switch (daemon.state) {
          DaemonState.ready => l10n.diagDaemonDetail(
            h['version'] ?? '—',
            daemon.port,
          ),
          DaemonState.failed => daemon.error ?? l10n.diagNotStarted,
          _ => l10n.commonStarting,
        },
        ready
            ? _Health.ok
            : (daemon.state == DaemonState.failed
                  ? _Health.attention
                  : _Health.pending),
      ),
      if (daemon.workspace != null)
        _Check(
          Icons.terminal_rounded,
          l10n.diagWorkspace,
          'Ubuntu 24.04 · proot',
          _Health.ok,
        ),
      _mediaCheck(daemon, pdf: false),
      _mediaCheck(daemon, pdf: true),
      _Check(
        Icons.auto_awesome_outlined,
        l10n.setupModelTitle,
        _models == null
            ? l10n.setupUnknown
            : (_models!['selected'] == null
                  ? l10n.setupModelMissing
                  : '${l10n.setupModelConfigured}\n${(_models!['selected']['vision'] == true || _models!['vision_ready'] == true) ? l10n.setupVisionConfigured : l10n.setupVisionMissing}'),
        _models == null
            ? _Health.pending
            : (_models!['selected'] != null ? _Health.ok : _Health.attention),
        action: FilledButton.tonal(
          onPressed: () async {
            await Navigator.of(context)
                .push(MaterialPageRoute(builder: (_) => const ModelsPage()));
            if (mounted) _load();
          },
          child: Text(l10n.setupConfigure),
        ),
      ),
      _Check(
        Icons.record_voice_over_outlined,
        l10n.setupAsrTitle,
        _asr == null
            ? l10n.setupUnknown
            : (_asrEnabled ? l10n.setupAsrConfigured : l10n.setupAsrMissing),
        _asr == null
            ? _Health.pending
            : (_asrEnabled ? _Health.ok : _Health.attention),
        action: FilledButton.tonal(
          onPressed: () => SettingsNav.open('transfer'),
          child: Text(l10n.setupConfigure),
        ),
      ),
      _Check(
        Icons.smart_toy_outlined,
        l10n.commonAssistant,
        switch (agentState) {
          'ready' =>
            node?['version'] != null
                ? l10n.diagAgentRunningNode(node!['version'])
                : l10n.commonRunning,
          'starting' => l10n.commonStarting,
          _ => agentError.isNotEmpty ? agentError : l10n.diagAgentNotRunning,
        },
        !ready || agentState == 'starting'
            ? _Health.pending
            : (agentState == 'ready' ? _Health.ok : _Health.attention),
        action: ready && agentState != 'ready' && agentState != 'starting'
            ? FilledButton.tonal(
                onPressed: () async {
                  await showRuntimeSetup(context);
                  if (mounted) _load();
                },
                child: Text(l10n.diagSetupRuntime),
              )
            : null,
      ),
    ];
  }

  bool get _asrEnabled => (_asr?['channels'] as List? ?? const [])
      .whereType<Map>()
      .any((c) => c['enabled'] == true);

  _Check _mediaCheck(DaemonService daemon, {required bool pdf}) {
    if (pdf && _pdfBackend == 'unsupported') {
      return _Check(
        Icons.picture_as_pdf_outlined,
        l10n.setupPdfTitle,
        l10n.setupPdfUnsupported,
        _Health.attention,
      );
    }
    final nativePdf = pdf && _pdfBackend == 'pdfkit';
    final names = pdf
        ? (nativePdf ? ['study_pdf'] : ['pdfinfo', 'pdftoppm', 'pdftotext'])
        : ['ffmpeg', 'ffprobe'];
    final known =
        (!pdf || _pdfBackend != null) &&
        names.every((name) => _tools?[name] is bool);
    final missing = names.where((name) => _tools?[name] != true).toList();
    return _Check(
      pdf ? Icons.picture_as_pdf_outlined : Icons.graphic_eq_rounded,
      pdf ? l10n.setupPdfTitle : l10n.setupMediaTitle,
      !known
          ? l10n.setupUnknown
          : missing.isEmpty
          ? l10n.setupToolsPresent
          : nativePdf
          ? l10n.setupPdfHelperMissing
          : l10n.setupToolsMissing(missing.join(', ')),
      !known
          ? _Health.pending
          : missing.isEmpty
          ? _Health.ok
          : _Health.attention,
      action: known && missing.isEmpty
          ? null
          : daemon.workspace != null
          ? (pdf
                ? WorkspacePDFButton(
                    workspace: daemon.workspace!,
                    onDone: _load,
                  )
                : WorkspaceFFmpegButton(
                    workspace: daemon.workspace!,
                    onDone: _load,
                  ))
          : TextButton(
              onPressed: () => showMediaToolsHelp(context, pdf: pdf),
              child: Text(l10n.setupInstallHelp),
            ),
    );
  }

  String _report(AppState app, DaemonService daemon, List<_Check> checks) => [
    'Coursive ${app.health['version'] ?? ''} · $_platform',
    for (final c in checks) l10n.diagReportLine(c.title, c.detail),
    if ('${app.health['library'] ?? ''}'.isNotEmpty)
      l10n.diagReportLibrary(app.health['library']),
  ].join('\n');

  @override
  Widget build(BuildContext context) {
    final app = context.watch<AppState>();
    final daemon = context.watch<DaemonService>();
    final checks = _checks(app, daemon);
    final library = '${app.health['library'] ?? ''}';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: Space.xl),
        Text(l10n.setupIntro),
        Align(
          alignment: Alignment.centerRight,
          child: TextButton.icon(
            onPressed: _loading ? null : _load,
            icon: const Icon(Icons.refresh),
            label: Text(l10n.setupRecheck),
          ),
        ),
        _Summary(
          checks: checks,
          loading: _loading,
          subtitle:
              'Coursive ${app.health['version'] ?? ''} · $_platform${daemon.workspace != null ? l10n.diagWorkspaceSuffix : ''}',
          onCopy: () {
            Clipboard.setData(
              ClipboardData(text: _report(app, daemon, checks)),
            );
            showSnack(context, l10n.diagCopied);
          },
        ),
        _Heading(l10n.diagStatus),
        _Segmented(children: [for (final c in checks) _CheckTile(c)]),
        if (library.isNotEmpty) ...[
          _Heading(l10n.commonLocation),
          _Segmented(
            children: [_PathTile(label: l10n.commonLibrary, path: library)],
          ),
        ],
        _Heading(l10n.diagLogs),
        _Segmented(
          children: [
            _LogTile(
              icon: Icons.receipt_long_outlined,
              title: l10n.diagDaemonLog,
              lines: daemon.log,
            ),
            _LogTile(
              icon: Icons.smart_toy_outlined,
              title: l10n.diagAgentLog,
              lines: _agentLog,
              onOpen: _load,
            ),
          ],
        ),
        _Heading(l10n.diagMaintenance),
        _Segmented(
          children: [
            _Tile(
              leading: Icons.restart_alt_rounded,
              title: l10n.diagRestart,
              subtitle: l10n.diagRestartHint,
              trailing: OutlinedButton(
                onPressed: daemon.restart,
                child: Text(l10n.diagRestartAction),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

/// The overall verdict, in words rather than a dashboard.
class _Summary extends StatelessWidget {
  const _Summary({
    required this.checks,
    required this.loading,
    required this.subtitle,
    required this.onCopy,
  });
  final List<_Check> checks;
  final bool loading;
  final String subtitle;
  final VoidCallback onCopy;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final issues = checks.where((c) => c.health == _Health.attention).length;
    final pending = checks.any((c) => c.health == _Health.pending);
    final (title, icon, bg, fg) = issues > 0
        ? (
            l10n.diagIssues(issues),
            Icons.priority_high_rounded,
            cs.errorContainer,
            cs.onErrorContainer,
          )
        : pending
        ? (
            loading ? l10n.commonChecking : l10n.setupUnknown,
            Icons.more_horiz_rounded,
            cs.surfaceContainerHighest,
            cs.onSurfaceVariant,
          )
        : (
            l10n.diagAllGood,
            Icons.check_rounded,
            cs.primaryContainer,
            cs.onPrimaryContainer,
          );
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: Space.xs),
      child: Row(
        children: [
          AnimatedContainer(
            duration: Motion.medium2,
            curve: Motion.emphasized,
            width: 56,
            height: 56,
            decoration: BoxDecoration(
              color: bg,
              borderRadius: BorderRadius.circular(Corners.lgIncreased),
            ),
            child: Icon(icon, color: fg, size: 28),
          ),
          const SizedBox(width: Space.lg),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: tt.headlineSmall),
                const SizedBox(height: Space.xxs),
                Text(
                  subtitle,
                  style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
                ),
              ],
            ),
          ),
          const SizedBox(width: Space.sm),
          IconButton.filledTonal(
            tooltip: l10n.diagCopy,
            onPressed: onCopy,
            icon: const Icon(Icons.content_copy_rounded, size: 20),
          ),
        ],
      ),
    );
  }
}

class _Heading extends StatelessWidget {
  const _Heading(this.text);
  final String text;
  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        Space.lg,
        Space.xl,
        Space.lg,
        Space.sm,
      ),
      child: Text(
        text,
        style: Theme.of(context).textTheme.labelLarge
            ?.copyWith(color: cs.onSurfaceVariant),
      ),
    );
  }
}

/// M3 Expressive segmented list: one tile per item, 2dp apart, with the
/// group's large corners only at its ends.
class _Segmented extends StatelessWidget {
  const _Segmented({required this.children});
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    const outer = Radius.circular(Corners.lgIncreased),
        inner = Radius.circular(Corners.xs);
    return Column(
      children: [
        for (final (i, c) in children.indexed) ...[
          if (i > 0) const SizedBox(height: Space.xxs),
          Material(
            color: cs.surfaceContainer,
            clipBehavior: Clip.antiAlias,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.vertical(
                top: i == 0 ? outer : inner,
                bottom: i == children.length - 1 ? outer : inner,
              ),
            ),
            child: c,
          ),
        ],
      ],
    );
  }
}

class _Tile extends StatelessWidget {
  const _Tile({
    required this.leading,
    required this.title,
    this.subtitle,
    this.trailing,
    this.below,
    this.onTap,
    this.leadingColor,
  });
  final IconData leading;
  final String title;
  final String? subtitle;
  final Widget? trailing, below;
  final VoidCallback? onTap;
  final Color? leadingColor;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: Space.lg,
          vertical: Space.md,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: cs.surfaceContainerHighest,
                    borderRadius: BorderRadius.circular(Corners.md),
                  ),
                  child: Icon(
                    leading,
                    size: 20,
                    color: leadingColor ?? cs.onSurfaceVariant,
                  ),
                ),
                const SizedBox(width: Space.lg),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(title, style: tt.titleSmall),
                      if (subtitle != null && subtitle!.isNotEmpty) ...[
                        const SizedBox(height: Space.xxs),
                        Text(
                          subtitle!,
                          style: tt.bodySmall?.copyWith(
                            color: cs.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                if (trailing != null) ...[
                  const SizedBox(width: Space.md),
                  trailing!,
                ],
              ],
            ),
            if (below != null)
              Padding(
                padding: const EdgeInsets.only(
                  left: 40 + Space.lg,
                  top: Space.sm,
                ),
                child: below!,
              ),
          ],
        ),
      ),
    );
  }
}

class _CheckTile extends StatelessWidget {
  const _CheckTile(this.c);
  final _Check c;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return _Tile(
      leading: c.icon,
      leadingColor: c.health == _Health.attention ? cs.error : null,
      title: c.title,
      subtitle: c.detail,
      trailing: _StatusPill(c.health),
      below: c.action == null
          ? null
          : Align(
              alignment: AlignmentDirectional.centerStart,
              child: c.action!,
            ),
    );
  }
}

class _StatusPill extends StatelessWidget {
  const _StatusPill(this.health);
  final _Health health;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final (label, dot) = switch (health) {
      _Health.ok => (l10n.diagOk, cs.primary),
      _Health.attention => (l10n.commonNeedsAttention, cs.error),
      _Health.pending => (l10n.diagChecking, cs.outline),
    };
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 8,
          height: 8,
          decoration: BoxDecoration(color: dot, shape: BoxShape.circle),
        ),
        const SizedBox(width: Space.sm),
        Text(
          label,
          style: Theme.of(context).textTheme.labelMedium?.copyWith(
            color: health == _Health.attention ? cs.error : cs.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _PathTile extends StatelessWidget {
  const _PathTile({required this.label, required this.path});
  final String label, path;

  @override
  Widget build(BuildContext context) {
    return _Tile(
      leading: Icons.folder_outlined,
      title: label,
      subtitle: path,
      onTap: canOpenFolders ? () => openPath(path) : null,
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          IconButton(
            tooltip: l10n.commonCopyPath,
            icon: const Icon(Icons.content_copy_outlined, size: 20),
            onPressed: () {
              Clipboard.setData(ClipboardData(text: path));
              showSnack(context, l10n.commonPathCopied);
            },
          ),
          if (canOpenFolders)
            IconButton(
              tooltip: l10n.commonOpenFolder,
              icon: const Icon(Icons.open_in_new_rounded, size: 20),
              onPressed: () => openPath(path),
            ),
        ],
      ),
    );
  }
}

/// A log's last lines as a preview; opens the whole log.
class _LogTile extends StatelessWidget {
  const _LogTile({
    required this.icon,
    required this.title,
    required this.lines,
    this.onOpen,
  });
  final IconData icon;
  final String title;
  final List<String> lines;
  final VoidCallback? onOpen;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tail = lines.length > 3 ? lines.sublist(lines.length - 3) : lines;
    return _Tile(
      leading: icon,
      title: title,
      subtitle: lines.isEmpty
          ? l10n.diagLogEmpty
          : l10n.diagLogLines(lines.length),
      trailing: Icon(Icons.chevron_right_rounded, color: cs.onSurfaceVariant),
      onTap: () {
        onOpen?.call();
        showLogViewer(context, title, lines);
      },
      below: tail.isEmpty
          ? null
          : Container(
              padding: const EdgeInsets.symmetric(
                horizontal: Space.md,
                vertical: Space.sm,
              ),
              decoration: BoxDecoration(
                color: cs.surfaceContainerLowest,
                borderRadius: BorderRadius.circular(Corners.sm),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  for (final l in tail)
                    Text(
                      l,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: _mono(context, l),
                    ),
                ],
              ),
            ),
    );
  }
}

bool _isError(String l) => RegExp(
  r'\b(error|failed|panic|fatal)\b|错误|失败',
  caseSensitive: false,
).hasMatch(l);

TextStyle _mono(BuildContext context, String line) {
  final cs = Theme.of(context).colorScheme;
  return TextStyle(
    fontFamily: 'monospace',
    fontSize: 12,
    height: 1.5,
    color: _isError(line) ? cs.error : cs.onSurfaceVariant,
  );
}

/// The whole log, newest at the bottom: full screen on phones, a large
/// dialog elsewhere.
Future<void> showLogViewer(
  BuildContext context,
  String title,
  List<String> lines,
) {
  Widget body(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(
            Space.xl,
            Space.lg,
            Space.sm,
            Space.sm,
          ),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title, style: tt.titleLarge),
                    Text(
                      lines.isEmpty
                          ? l10n.diagLogEmpty
                          : l10n.diagLogRecent(lines.length),
                      style: tt.bodySmall?.copyWith(color: cs.onSurfaceVariant),
                    ),
                  ],
                ),
              ),
              IconButton(
                tooltip: l10n.commonCopyAll,
                icon: const Icon(Icons.content_copy_rounded),
                onPressed: lines.isEmpty
                    ? null
                    : () {
                        Clipboard.setData(
                          ClipboardData(text: lines.join('\n')),
                        );
                        showSnack(context, l10n.diagLogCopied);
                      },
              ),
              IconButton(
                tooltip: l10n.commonClose,
                icon: const Icon(Icons.close_rounded),
                onPressed: () => Navigator.pop(context),
              ),
            ],
          ),
        ),
        Expanded(
          child: Container(
            margin: const EdgeInsets.fromLTRB(Space.md, 0, Space.md, Space.md),
            decoration: BoxDecoration(
              color: cs.surfaceContainerLowest,
              borderRadius: BorderRadius.circular(Corners.lg),
            ),
            clipBehavior: Clip.antiAlias,
            child: lines.isEmpty
                ? Center(
                    child: Text(
                      l10n.diagLogNothing,
                      style: tt.bodyMedium?.copyWith(
                        color: cs.onSurfaceVariant,
                      ),
                    ),
                  )
                : SelectionArea(
                    child: ListView.builder(
                      reverse: true,
                      padding: const EdgeInsets.all(Space.lg),
                      itemCount: lines.length,
                      itemBuilder: (context, i) {
                        final l = lines[lines.length - 1 - i];
                        return Text(l, style: _mono(context, l));
                      },
                    ),
                  ),
          ),
        ),
      ],
    );
  }

  if (context.isCompact) {
    return showDialog(
      context: context,
      builder: (context) =>
          Dialog.fullscreen(child: SafeArea(child: body(context))),
    );
  }
  return showDialog(
    context: context,
    builder: (context) => Dialog(
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(Corners.xl),
      ),
      clipBehavior: Clip.antiAlias,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 760, maxHeight: 600),
        child: body(context),
      ),
    ),
  );
}
