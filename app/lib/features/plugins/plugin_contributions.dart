import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/plugin_navigation.dart';
import '../../core/plugins.dart';
import '../../core/ui_slots.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import 'declarative_renderer.dart';
import 'plugin_actions.dart';

/// The slot targets this host renders and how contributions combine there. It
/// matches `SLOT_TARGETS` in agent/src/plugins.mjs, which refuses any other
/// target at install (docs/specs/0032-ui-extension-protocol.md).
const uiSlotCatalog = {
  'course.detail.sections': UiSlotMode.list,
  'canvas.toolbar.tools': UiSlotMode.list,
};

/// A host surface that takes view patches from the plugin views inside it, such
/// as the study canvas committing a `canvas` batch. A view outside one ignores
/// its view patches.
class PluginViewPatchScope extends InheritedWidget {
  const PluginViewPatchScope({
    super.key,
    required this.onPatch,
    required super.child,
  });

  final Future<void> Function(String pluginId, PluginViewPatch patch) onPatch;

  static PluginViewPatchScope? maybeOf(BuildContext context) =>
      context.getInheritedWidgetOfExactType<PluginViewPatchScope>();

  @override
  bool updateShouldNotify(PluginViewPatchScope oldWidget) =>
      onPatch != oldWidget.onPatch;
}

/// Feeds the pages and slots of active plugins from [store] into the host
/// registries on every change of the plugin list, so disabling or removing a
/// plugin revokes them. A plugin whose route or keyed slot collides with one
/// already taken is left out whole and named in [skipped]; the others stay.
class PluginContributionBinder {
  PluginContributionBinder({
    required this.store,
    required this.pages,
    required this.slots,
  }) {
    store.addListener(sync);
    sync();
  }

  final PluginStore store;
  final PluginNavigationRegistry pages;
  final UiSlotRegistry slots;

  /// Plugins left out at the last sync, with the colliding route or slot key.
  final skipped = <String, String>{};

  void sync() {
    final nextPages = <PluginPageContribution>[];
    final nextSlots = <UiSlotContribution>[];
    final routes = <String>{};
    final keys = <String>{};
    skipped.clear();
    for (final plugin in store.active) {
      final id = '${plugin['id']}';
      final ownPages = [
        for (final e in _entries(plugin, 'pages'))
          if (e.raw['document'] is Map) PluginPageContribution.fromEntry(e),
      ];
      final ownSlots = [
        for (final e in _entries(plugin, 'slots')) ?slotContribution(e),
      ];
      final ownRoutes = {for (final p in ownPages) p.routeId};
      final ownKeys = {
        for (final s in ownSlots)
          if (s.mode == UiSlotMode.keyed) '${s.target}/${s.slotKey}',
      };
      final clash = [
        ...ownRoutes.where(routes.contains),
        ...ownKeys.where(keys.contains),
      ];
      if (clash.isNotEmpty) {
        skipped[id] = clash.first;
        continue;
      }
      routes.addAll(ownRoutes);
      keys.addAll(ownKeys);
      nextPages.addAll(ownPages);
      nextSlots.addAll(ownSlots);
    }
    pages.replaceAll(nextPages);
    slots.replaceAll(nextSlots);
  }

  void dispose() => store.removeListener(sync);

  static Iterable<PluginEntry> _entries(
    Map<String, dynamic> plugin,
    String kind,
  ) sync* {
    for (final e in jsonList((plugin['contributes'] as Map?)?[kind])) {
      if (e is Map) yield PluginEntry(plugin, kind, e);
    }
  }
}

/// A slot entry the host can render: a declarative document on a catalog
/// target, in the target's mode. Anything else is left out.
UiSlotContribution? slotContribution(PluginEntry e) {
  final mode = uiSlotCatalog[e.target];
  final document = e.raw['document'];
  if (mode == null || mode.name != e.mode || document is! Map) return null;
  return UiSlotContribution(
    target: e.target,
    id: e.id,
    pluginId: e.pluginId,
    mode: mode,
    order: e.order,
    priority: e.priority,
    key: e.raw['key'] == null ? null : e.slotKey,
    builder: (context, slotContext) => PluginDeclarativeView(
      key: ValueKey('${e.pluginId}/${e.id}'),
      pluginId: e.pluginId,
      viewId: e.id,
      document: document.cast<String, dynamic>(),
      hostContext: slotContext,
    ),
  );
}

/// The `declarative` page renderer: the page document wired to the action
/// bridge.
Widget declarativePageBuilder(
  BuildContext context,
  PluginPageContribution page,
) {
  final document = page.view['document'];
  if (document is! Map) {
    return DeclarativeErrorView(message: 'missing document');
  }
  return SingleChildScrollView(
    padding: const EdgeInsets.all(Space.md),
    child: PluginDeclarativeView(
      pluginId: page.pluginId,
      viewId: page.id,
      document: document.cast<String, dynamic>(),
    ),
  );
}

/// A plugin's declarative document bound to its view state in the daemon. It
/// reads the state and revision when it mounts, sends each button through the
/// action bridge, applies a state patch, opens a page of the same plugin for a
/// navigation result, hands a view patch to the enclosing [PluginViewPatchScope]
/// and shows a notification or a refusal as a snackbar. The host context of a
/// slot (such as `course_id`) is readable as `context.*`.
class PluginDeclarativeView extends StatefulWidget {
  const PluginDeclarativeView({
    super.key,
    required this.pluginId,
    required this.viewId,
    required this.document,
    this.hostContext = const {},
    this.call,
  });

  final String pluginId, viewId;
  final Map<String, dynamic> document;
  final Map<String, dynamic> hostContext;

  /// The daemon call; null uses the app's API.
  final ApiCall? call;

  @override
  State<PluginDeclarativeView> createState() => _PluginDeclarativeViewState();
}

class _PluginDeclarativeViewState extends State<PluginDeclarativeView> {
  late final PluginActionClient _client;
  Map<String, dynamic> _state = const {};

  @override
  void initState() {
    super.initState();
    _client = PluginActionClient(
      call: widget.call ?? context.read<AppState>().api.call,
      onStatePatch: (patch) {
        if (mounted) setState(() => _state = patch.state);
      },
      onNavigation: _open,
      onViewPatch: (patch) {
        if (mounted) {
          PluginViewPatchScope.maybeOf(context)
              ?.onPatch(widget.pluginId, patch);
        }
      },
      onNotification: (n) => _say(n.text),
      onProposal: _confirm,
    );
    _reload();
  }

  Future<void> _reload() async {
    try {
      final view = await _client.readState(widget.pluginId, widget.viewId);
      if (mounted) setState(() => _state = view.state);
    } catch (_) {
      // The document still renders; its bindings stay empty until an action.
    }
  }

  Future<void> _act(String action, Map<String, dynamic> args) async {
    try {
      final outcome = await _client.invoke(
        PluginActionRequest(
          pluginId: widget.pluginId,
          actionId: action,
          viewId: widget.viewId,
          args: args,
          contextRevision: _client.revisionOf(widget.pluginId, widget.viewId),
        ),
      );
      final failure = outcome.failure;
      if (failure == null) return;
      if (failure.code == 'revision_conflict') await _reload();
      _say(l10n.pluginActionFailed(_title(action), failure.message));
    } catch (e) {
      _say(l10n.pluginActionFailed(_title(action), '$e'));
    }
  }

  /// Asks before a plugin writes to the library; nothing is written unless
  /// the user approves.
  Future<void> _confirm(PluginProposal proposal) async {
    final plugin = context.read<PluginStore>().byId(widget.pluginId);
    final approved = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.pluginProposalTitle(proposal.summary)),
        content: Text(
          l10n.pluginProposalBody('${plugin?['title'] ?? widget.pluginId}'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: Text(l10n.pluginProposalReject),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(l10n.pluginProposalApply),
          ),
        ],
      ),
    );
    try {
      final outcome = approved == true
          ? await _client.apply(proposal)
          : await _client.reject(proposal);
      if (outcome.failure case final failure?) {
        _say(l10n.pluginActionFailed(proposal.summary, failure.message));
      }
    } catch (e) {
      _say(l10n.pluginActionFailed(proposal.summary, '$e'));
    }
  }

  void _open(PluginNavigation navigation) {
    final registry = context.read<PluginNavigationRegistry>();
    final page =
        registry.byRoute(navigation.route) ??
        registry.byRoute('plugin:${widget.pluginId}:${navigation.route}');
    if (page == null) {
      _say(
        l10n.pluginRouteMissing(
          navigation.title.isEmpty ? navigation.route : navigation.title,
        ),
      );
      return;
    }
    final route = MaterialPageRoute<void>(
      builder: (_) => PluginPageOutlet(page: page),
    );
    final navigator = Navigator.of(context, rootNavigator: true);
    navigation.replace
        ? navigator.pushReplacement(route)
        : navigator.push(route);
  }

  String _title(String action) {
    final plugin = context.read<PluginStore>().byId(widget.pluginId);
    for (final a in jsonList((plugin?['contributes'] as Map?)?['actions'])) {
      if (a is Map && a['id'] == action) return '${a['title']}';
    }
    return action;
  }

  void _say(String text) {
    if (!mounted || text.isEmpty) return;
    ScaffoldMessenger.maybeOf(context)
        ?.showSnackBar(SnackBar(content: Text(text)));
  }

  @override
  Widget build(BuildContext context) => DeclarativeRenderer(
    document: widget.document,
    state: {..._state, 'context': widget.hostContext},
    onAction: _act,
  );
}
