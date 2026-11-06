import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import '../plugins/plugins_page.dart' show PluginGlyph;
import 'sources_page.dart';

/// 插件 → 发现 → 来源插件: the source plugins in the plugin index, each with
/// install, sign-in and uninstall. The daemon owns these installs, so the
/// section works without the assistant.
/// Contract: docs/specs/0025-plugin-marketplace.md
class SourceMarketSection extends StatefulWidget {
  const SourceMarketSection({super.key, this.query = '', this.category = ''});

  /// The 发现 search and category filter.
  final String query, category;

  @override
  State<SourceMarketSection> createState() => _SourceMarketSectionState();
}

class _SourceMarketSectionState extends State<SourceMarketSection> {
  List<Map<String, dynamic>>? _listings;
  // Why the daemon shows the last verified index, or none: '', 'unverified'
  // or 'stale'.
  String _notice = '';
  Object? _error;
  bool _loading = false;

  Future<void> _load() async {
    if (_loading) return;
    _loading = true;
    try {
      final r = (await context.read<AppState>().api.get(
        '/api/sources/market',
      ) as Map).cast<String, dynamic>();
      if (!mounted) return;
      setState(() {
        _listings = [
          for (final p in jsonList(r['plugins']))
            (p as Map).cast<String, dynamic>(),
        ];
        _notice = '${r['notice'] ?? ''}';
        _error = null;
      });
    } catch (e) {
      if (mounted) setState(() => _error = e);
    } finally {
      _loading = false;
    }
  }

  bool _matches(Map<String, dynamic> p) {
    final q = widget.query.trim().toLowerCase();
    return q.isEmpty ||
        [
          'title',
          'summary',
          'id',
          'author',
        ].any((k) => '${p[k] ?? ''}'.toLowerCase().contains(q));
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    // The daemon may still be starting; load once it answers.
    if (!context.select<AppState, bool>((a) => a.connected)) {
      return const SizedBox.shrink();
    }
    if (_listings == null && _error == null && !_loading) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _load();
      });
    }
    if (widget.category.isNotEmpty && widget.category != 'source') {
      return const SizedBox.shrink();
    }
    final all = _listings;
    final shown =
        all?.where(_matches).toList() ?? const <Map<String, dynamic>>[];
    if (all != null && all.isNotEmpty && shown.isEmpty) {
      return const SizedBox.shrink();
    }
    final notice = switch (_notice) {
      _ when _error != null || all == null => null,
      'unverified' when all.isEmpty => l10n.sourcesMarketUnverifiedEmpty,
      'unverified' => l10n.sourcesMarketUnverified,
      'stale' => l10n.sourcesMarketStale,
      _ => null,
    };
    final Widget body;
    if (_error != null) {
      body = _Note(
        icon: Icons.cloud_off_rounded,
        text: l10n.sourcesMarketFailed(
          _error is ApiError ? (_error as ApiError).message : '$_error',
        ),
        action: TextButton(onPressed: _load, child: Text(l10n.commonRetry)),
        error: true,
      );
    } else if (all == null) {
      body = const Padding(
        padding: EdgeInsets.all(Space.lg),
        child: Center(child: CircularProgressIndicator()),
      );
    } else if (all.isEmpty && notice != null) {
      body = const SizedBox.shrink();
    } else if (all.isEmpty) {
      body = _Note(
        icon: Icons.inventory_2_outlined,
        text: l10n.sourcesMarketEmpty,
        action: TextButton(
          onPressed: () => installSourcePlugin(context),
          child: Text(l10n.sourcesInstall),
        ),
      );
    } else {
      body = LayoutBuilder(
        builder: (context, c) {
          final cols = (c.maxWidth / 300).floor().clamp(1, 4);
          final w = (c.maxWidth - (cols - 1) * Space.md) / cols;
          return Wrap(
            spacing: Space.md,
            runSpacing: Space.md,
            children: [
              for (final p in shown)
                SizedBox(
                  width: w,
                  child: _SourceCard(listing: p),
                ),
            ],
          );
        },
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(Space.xs, Space.xl, 0, Space.md),
          child: Row(
            children: [
              Icon(Icons.school_outlined, size: 20, color: cs.primary),
              const SizedBox(width: Space.sm),
              Text(l10n.sourcesMarketTitle, style: tt.titleMedium),
              if (shown.isNotEmpty) ...[
                const SizedBox(width: Space.sm),
                Text(
                  '${shown.length}',
                  style: tt.titleMedium?.copyWith(color: cs.onSurfaceVariant),
                ),
              ],
            ],
          ),
        ),
        if (notice != null) ...[
          _Note(
            icon: Icons.gpp_maybe_outlined,
            text: notice,
            action: TextButton(onPressed: _load, child: Text(l10n.commonRetry)),
          ),
          const SizedBox(height: Space.md),
        ],
        body,
      ],
    );
  }
}

/// One listed source plugin: 安装 when absent; once installed, its sign-in
/// state with 登录 and 卸载.
class _SourceCard extends StatefulWidget {
  const _SourceCard({required this.listing});
  final Map<String, dynamic> listing;

  @override
  State<_SourceCard> createState() => _SourceCardState();
}

class _SourceCardState extends State<_SourceCard> {
  bool _busy = false;

  Future<void> _run(Future<void> Function() f) async {
    setState(() => _busy = true);
    try {
      await f();
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Busy covers only the download and its verification; consent and
  /// sign-in are sheets of their own.
  Future<void> _install(String id) async {
    setState(() => _busy = true);
    await installListedSource(
      context,
      id,
      onStaged: () {
        if (mounted) setState(() => _busy = false);
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final p = widget.listing;
    final id = '${p['id']}';
    final version = '${p['version'] ?? ''}';
    final official = p['official'] == true;
    final installed = context
        .watch<AppState>()
        .plugins
        .where((x) => x.id == id)
        .firstOrNull;
    final source = installed?.sources.firstOrNull;
    final listed = '${p['icon'] ?? ''}';
    final icon = installed?.icon.isNotEmpty == true
        ? installed!.icon
        : (listed.isEmpty ? 'school' : listed);
    final update =
        installed != null &&
        p['available'] == true &&
        installed.version != version &&
        version.isNotEmpty;
    const compact = ButtonStyle(visualDensity: VisualDensity.compact);

    final String status;
    final Color statusColor;
    final List<Widget> actions;
    if (installed == null) {
      status = p['available'] == true
          ? l10n.sourcesVersion(version)
          : l10n.sourcesMarketUnsupported;
      statusColor = cs.onSurfaceVariant;
      actions = [
        FilledButton.tonalIcon(
          style: compact,
          onPressed: p['available'] == true && !_busy
              ? () => _install(id)
              : null,
          icon: _busy
              ? const SizedBox.square(
                  dimension: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.download_rounded, size: 18),
          label: Text(_busy ? l10n.sourcesMarketVerifying : l10n.commonInstall),
        ),
      ];
    } else {
      final signedIn = source?.signedIn ?? false;
      final expired = source?.expired ?? false;
      status = switch (source) {
        _ when !installed.enabled => l10n.sourcesDisabled,
        final s? when s.signedIn =>
          s.account.isEmpty
              ? l10n.sourceSignedInPlain
              : l10n.sourceSignedInAs(s.account),
        final s? when s.expired => l10n.sourceExpired,
        _ => l10n.sourceSignedOut,
      };
      statusColor = signedIn
          ? cs.primary
          : (expired ? cs.error : cs.onSurfaceVariant);
      actions = [
        TextButton(
          style: compact,
          onPressed: _busy
              ? null
              : () => _run(() => removeSourcePlugin(context, installed)),
          child: Text(l10n.commonUninstall),
        ),
        if (update)
          FilledButton(
            style: compact,
            onPressed: _busy ? null : () => _install(id),
            child: Text(l10n.sourcesMarketUpdate(version)),
          )
        else if (source != null &&
            installed.enabled &&
            !signedIn &&
            source.fields.isNotEmpty)
          FilledButton.tonal(
            style: compact,
            onPressed: _busy
                ? null
                : () => _run(() => signInSource(context, source)),
            child: Text(expired ? l10n.loginSignInAgain : l10n.loginSignIn),
          ),
      ];
    }

    return Material(
      color: cs.surfaceContainerLow,
      shape: RoundedRectangleBorder(borderRadius: Corners.all(Corners.xl)),
      clipBehavior: Clip.antiAlias,
      child: Padding(
        padding: const EdgeInsets.all(Space.lg),
        child: SizedBox(
          height: 148,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  PluginGlyph(icon: icon, id: id, size: 48),
                  const SizedBox(width: Space.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '${p['title']}',
                          style: tt.titleMedium,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        const SizedBox(height: Space.xxs),
                        Row(
                          children: [
                            if (official) ...[
                              Icon(
                                Icons.verified_rounded,
                                size: 14,
                                color: cs.primary,
                              ),
                              const SizedBox(width: Space.xxs),
                            ],
                            Flexible(
                              child: Text(
                                official
                                    ? l10n.pluginsOfficial
                                    : '${p['author'] ?? ''}',
                                style: tt.labelMedium?.copyWith(
                                  color: cs.onSurfaceVariant,
                                ),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: Space.md),
              Expanded(
                child: Text(
                  '${p['summary']}',
                  style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              Row(
                children: [
                  if (installed != null) ...[
                    Icon(
                      source?.signedIn ?? false
                          ? Icons.check_circle_rounded
                          : Icons.radio_button_unchecked_rounded,
                      size: 16,
                      color: statusColor,
                    ),
                    const SizedBox(width: Space.xs),
                  ],
                  Expanded(
                    child: Text(
                      status,
                      style: tt.labelLarge?.copyWith(color: statusColor),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  for (final a in actions) ...[
                    const SizedBox(width: Space.xs),
                    a,
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Note extends StatelessWidget {
  const _Note({
    required this.icon,
    required this.text,
    required this.action,
    this.error = false,
  });
  final IconData icon;
  final String text;
  final Widget action;
  final bool error;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final (bg, fg) = error
        ? (cs.errorContainer, cs.onErrorContainer)
        : (cs.surfaceContainer, cs.onSurfaceVariant);
    return Container(
      padding: const EdgeInsets.fromLTRB(
        Space.lg,
        Space.sm,
        Space.sm,
        Space.sm,
      ),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: Corners.all(Corners.lg),
      ),
      child: Row(
        children: [
          Icon(icon, size: 20, color: fg),
          const SizedBox(width: Space.md),
          Expanded(
            child: Text(text, style: tt.bodyMedium?.copyWith(color: fg)),
          ),
          const SizedBox(width: Space.sm),
          action,
        ],
      ),
    );
  }
}
