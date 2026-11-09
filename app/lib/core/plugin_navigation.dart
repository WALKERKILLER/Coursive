import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../features/plugins/plugin_views.dart' show PluginView;
import '../l10n/l10n.dart';
import '../widgets/states.dart';
import '../theme/tokens.dart';
import 'plugins.dart';

/// One page a plugin contributes. The route id is stable across plugin
/// upgrades; [id] is only unique inside the plugin.
@immutable
class PluginPageContribution {
  const PluginPageContribution({
    required this.id,
    required this.pluginId,
    required this.title,
    this.icon = 'extension',
    this.renderer = 'declarative',
    this.route,
    this.order = 100,
    this.requiredServices = const [],
    this.requiredPermissions = const [],
    this.availableOn = const [],
    this.minHostVersion = '',
    this.fallbackRoute = '',
    this.view = const {},
  });

  factory PluginPageContribution.fromEntry(PluginEntry entry) =>
      PluginPageContribution(
        id: entry.id,
        pluginId: entry.pluginId,
        title: entry.title,
        icon: entry.icon,
        renderer: entry.renderer,
        route: entry.route,
        order: entry.order,
        requiredServices: entry.requiredServices,
        requiredPermissions: entry.requiredPermissions,
        availableOn: entry.availableOn,
        minHostVersion: entry.minHostVersion,
        fallbackRoute: entry.fallbackRoute,
        view: entry.raw,
      );

  final String id, pluginId, title, icon, renderer;
  final String? route;
  final int order;
  final List<String> requiredServices, requiredPermissions, availableOn;
  final String minHostVersion;
  final String fallbackRoute;

  /// The contribution payload a renderer may use, for example a declarative
  /// page document.
  final Map<String, dynamic> view;

  String get routeId => route ?? 'plugin:$pluginId:$id';
  String get key => '$pluginId/$id';

  /// A contribution is available when the host satisfies its declared version
  /// and platform constraints. Service and permission checks stay in the
  /// daemon; this is only the UI-side gate.
  bool availableOnThisHost({String? hostVersion, String? platform}) {
    if (availableOn.isNotEmpty &&
        platform != null &&
        !availableOn.contains(platform)) {
      return false;
    }
    if (minHostVersion.isNotEmpty &&
        hostVersion != null &&
        _compareVersions(hostVersion, minHostVersion) < 0) {
      return false;
    }
    return true;
  }
}

int _compareVersions(String a, String b) {
  final x = a.split('.').map(int.tryParse).toList();
  final y = b.split('.').map(int.tryParse).toList();
  for (var i = 0; i < 3; i++) {
    final d = (i < x.length ? x[i] ?? 0 : 0) - (i < y.length ? y[i] ?? 0 : 0);
    if (d != 0) return d;
  }
  return 0;
}

/// Pages registered by active plugins. Registration is reversible: every
/// [register] returns the callback that removes exactly that contribution.
class PluginNavigationRegistry extends ChangeNotifier {
  final Map<String, PluginPageContribution> _pages = {};

  List<PluginPageContribution> get pages {
    final out = _pages.values.toList()
      ..sort((a, b) {
        final byOrder = a.order.compareTo(b.order);
        if (byOrder != 0) return byOrder;
        final byPlugin = a.pluginId.compareTo(b.pluginId);
        return byPlugin != 0 ? byPlugin : a.id.compareTo(b.id);
      });
    return List.unmodifiable(out);
  }

  PluginPageContribution? byRoute(String route) => _pages[route];

  VoidCallback register(PluginPageContribution page) {
    if (_pages.containsKey(page.routeId)) {
      throw ArgumentError('duplicate plugin route: ${page.routeId}');
    }
    _pages[page.routeId] = page;
    notifyListeners();
    return () {
      if (identical(_pages[page.routeId], page)) {
        _pages.remove(page.routeId);
        notifyListeners();
      }
    };
  }

  /// Replaces every page in one transaction. A duplicate route rejects the
  /// whole batch and leaves the previous set intact.
  void replaceAll(Iterable<PluginPageContribution> pages) {
    final next = <String, PluginPageContribution>{};
    for (final page in pages) {
      if (next.containsKey(page.routeId)) {
        throw ArgumentError('duplicate plugin route: ${page.routeId}');
      }
      next[page.routeId] = page;
    }
    if (mapEquals(next, _pages)) return;
    _pages
      ..clear()
      ..addAll(next);
    notifyListeners();
  }

  void removePlugin(String pluginId) {
    final before = _pages.length;
    _pages.removeWhere((_, page) => page.pluginId == pluginId);
    if (_pages.length != before) notifyListeners();
  }
}

typedef PluginPageBuilder = Widget Function(
  BuildContext context,
  PluginPageContribution page,
);

/// Maps a page's declared renderer to the host implementation. Unknown
/// renderers fall back to [PluginFallbackPage] instead of a blank screen.
class PluginPageBuilderRegistry {
  PluginPageBuilderRegistry([Map<String, PluginPageBuilder>? builders])
    : _builders = {...?builders};

  final Map<String, PluginPageBuilder> _builders;

  void register(String renderer, PluginPageBuilder builder) {
    _builders[renderer] = builder;
  }

  Widget build(BuildContext context, PluginPageContribution page) {
    final builder = _builders[page.renderer];
    if (builder == null) {
      return PluginFallbackPage(
        page: page,
        message: l10n.pluginViewUnsupported,
      );
    }
    try {
      return builder(context, page);
    } catch (_) {
      return PluginFallbackPage(
        page: page,
        message: l10n.pluginViewUnsupported,
      );
    }
  }
}

/// The host-owned frame for a plugin page. The page content comes from the
/// renderer registry; the app keeps the route, app bar and fallback.
class PluginPageOutlet extends StatelessWidget {
  const PluginPageOutlet({
    super.key,
    required this.page,
    this.builders,
    this.showAppBar = true,
  });

  final PluginPageContribution page;
  final PluginPageBuilderRegistry? builders;
  final bool showAppBar;

  @override
  Widget build(BuildContext context) {
    final registry = builders ?? context.read<PluginPageBuilderRegistry>();
    final body = registry.build(context, page);
    if (!showAppBar) return body;
    return Scaffold(
      appBar: AppBar(title: Text(page.title)),
      body: body,
    );
  }
}

/// A safe page for a missing renderer, an unavailable plugin or a failed
/// contribution. It keeps the plugin title and never deletes user data.
class PluginFallbackPage extends StatelessWidget {
  const PluginFallbackPage({super.key, required this.page, this.message});

  final PluginPageContribution page;
  final String? message;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(page.title)),
      body: EmptyState(
        icon: Icons.extension_off_outlined,
        title: page.title,
        body: message ?? l10n.pluginViewUnsupported,
      ),
    );
  }
}

/// Lists the pages contributed by active plugins and opens them through the
/// host outlet. This is the mobile extension center and the desktop launcher
/// until the rail learns to pin plugin pages.
class PluginPageLauncher extends StatelessWidget {
  const PluginPageLauncher({super.key, required this.store});

  final PluginStore store;

  @override
  Widget build(BuildContext context) {
    final pages = [
      for (final entry in store.entries('pages'))
        PluginPageContribution.fromEntry(entry),
    ];
    if (pages.isEmpty) return const SizedBox.shrink();
    final tt = Theme.of(context).textTheme;
    final cs = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(Space.md, Space.md, 0, Space.sm),
          child: Text(
            l10n.pluginPages,
            style: tt.labelLarge?.copyWith(color: cs.onSurfaceVariant),
          ),
        ),
        for (final page in pages)
          ListTile(
            contentPadding: const EdgeInsets.symmetric(horizontal: Space.md),
            leading: Icon(pluginIcon(page.icon), color: cs.primary),
            title: Text(page.title),
            subtitle: Text(
              page.pluginId,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            trailing: const Icon(Icons.chevron_right_rounded),
            onTap: () => Navigator.of(context, rootNavigator: true).push(
              MaterialPageRoute(builder: (_) => PluginPageOutlet(page: page)),
            ),
          ),
      ],
    );
  }
}

/// A declarative page renderer that maps a contribution's `view` payload to
/// the existing native view catalog. It is the first page renderer; richer
/// documents use [DeclarativeRenderer] in the plugin features layer.
Widget declarativePluginPage(
  BuildContext context,
  PluginPageContribution page,
) {
  return PluginView(
    view: {
      'kind': '${page.view['view'] ?? page.view['kind'] ?? 'page'}',
      'data': page.view['data'] ?? page.view,
    },
  );
}
