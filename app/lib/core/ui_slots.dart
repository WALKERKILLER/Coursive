import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

/// How multiple contributions to one slot combine.
enum UiSlotMode { list, single, keyed, chain, hoc }

/// One host-approved extension point contribution. The builder receives only
/// the outlet context, never the app state, a database handle or credentials.
@immutable
class UiSlotContribution {
  const UiSlotContribution({
    required this.target,
    required this.id,
    required this.pluginId,
    required this.builder,
    this.mode = UiSlotMode.list,
    this.order = 100,
    this.priority = 0,
    this.scope,
    this.key,
  });

  final String target, id, pluginId;
  final UiSlotMode mode;
  final int order, priority;
  final String? scope;
  final String? key;
  final Widget Function(BuildContext context, Map<String, dynamic> context_)
  builder;

  String get registrationKey => '$pluginId/$id';
  String get slotKey => key ?? id;
}

/// Registry for typed UI slots. Registration is reversible and the whole set
/// can be replaced atomically when a plugin activates or upgrades.
class UiSlotRegistry extends ChangeNotifier {
  final Map<String, Map<String, UiSlotContribution>> _byTarget = {};

  List<UiSlotContribution> contributions(String target, {String? scope}) {
    final all =
        _byTarget[target]?.values.toList() ?? const <UiSlotContribution>[];
    final out =
        all
            .where((c) => scope == null || c.scope == null || c.scope == scope)
            .toList()
          ..sort((a, b) {
            final byOrder = a.order.compareTo(b.order);
            if (byOrder != 0) return byOrder;
            final byPriority = b.priority.compareTo(a.priority);
            if (byPriority != 0) return byPriority;
            final byPlugin = a.pluginId.compareTo(b.pluginId);
            return byPlugin != 0 ? byPlugin : a.id.compareTo(b.id);
          });
    return List.unmodifiable(out);
  }

  VoidCallback register(UiSlotContribution contribution) {
    final target = _byTarget.putIfAbsent(contribution.target, () => {});
    if (target.containsKey(contribution.registrationKey)) {
      throw ArgumentError(
        'duplicate slot contribution: ${contribution.registrationKey}',
      );
    }
    target[contribution.registrationKey] = contribution;
    notifyListeners();
    return () {
      if (identical(
        _byTarget[contribution.target]?[contribution.registrationKey],
        contribution,
      )) {
        _byTarget[contribution.target]?.remove(contribution.registrationKey);
        if (_byTarget[contribution.target]?.isEmpty ?? false) {
          _byTarget.remove(contribution.target);
        }
        notifyListeners();
      }
    };
  }

  /// Replaces every contribution in one transaction. Duplicate registration
  /// keys or duplicate keyed slot keys reject the whole batch.
  void replaceAll(Iterable<UiSlotContribution> contributions) {
    final next = <String, Map<String, UiSlotContribution>>{};
    final keyed = <String, Set<String>>{};
    for (final c in contributions) {
      final target = next.putIfAbsent(c.target, () => {});
      if (target.containsKey(c.registrationKey)) {
        throw ArgumentError(
          'duplicate slot contribution: ${c.registrationKey}',
        );
      }
      if (c.mode == UiSlotMode.keyed) {
        final keys = keyed.putIfAbsent(c.target, () => {});
        if (!keys.add(c.slotKey)) {
          throw ArgumentError(
            'duplicate keyed slot key: ${c.target}/${c.slotKey}',
          );
        }
      }
      target[c.registrationKey] = c;
    }
    _byTarget
      ..clear()
      ..addAll(next);
    notifyListeners();
  }

  void removePlugin(String pluginId) {
    var changed = false;
    for (final target in _byTarget.values) {
      final before = target.length;
      target.removeWhere((_, c) => c.pluginId == pluginId);
      changed = changed || target.length != before;
    }
    _byTarget.removeWhere((_, target) => target.isEmpty);
    if (changed) notifyListeners();
  }
}

/// Renders one slot target with the host's mode semantics. An empty slot
/// renders [fallback] (or nothing), so adding a slot never changes an
/// existing page when no plugin contributes to it.
class UiSlotOutlet extends StatelessWidget {
  const UiSlotOutlet({
    super.key,
    required this.target,
    this.context_ = const {},
    this.scope,
    this.slotKey,
    this.fallback,
    this.listBuilder,
  });

  final String target;
  final Map<String, dynamic> context_;
  final String? scope;
  final String? slotKey;
  final Widget? fallback;
  final Widget Function(
    BuildContext context,
    List<UiSlotContribution> contributions,
  )?
  listBuilder;

  @override
  Widget build(BuildContext context) {
    final registry = _maybeRegistry(context);
    if (registry == null) return fallback ?? const SizedBox.shrink();
    final contributions = registry.contributions(target, scope: scope);
    if (contributions.isEmpty) return fallback ?? const SizedBox.shrink();

    final mode = contributions.first.mode;
    if (listBuilder != null) return listBuilder!(context, contributions);
    switch (mode) {
      case UiSlotMode.list:
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final c in contributions)
              _SlotHost(
                contribution: c,
                context_: context_,
                fallback: fallback,
              ),
          ],
        );
      case UiSlotMode.single:
      case UiSlotMode.hoc:
        return _SlotHost(
          contribution: contributions.first,
          context_: context_,
          fallback: fallback,
        );
      case UiSlotMode.keyed:
        UiSlotContribution? match;
        for (final c in contributions) {
          if (c.slotKey == slotKey) {
            match = c;
            break;
          }
        }
        return match == null
            ? (fallback ?? const SizedBox.shrink())
            : _SlotHost(
                contribution: match,
                context_: context_,
                fallback: fallback,
              );
      case UiSlotMode.chain:
        return _ChainSlot(
          contributions: contributions,
          context_: context_,
          fallback: fallback,
        );
    }
  }
}

class _SlotHost extends StatelessWidget {
  const _SlotHost({
    required this.contribution,
    required this.context_,
    this.fallback,
  });

  final UiSlotContribution contribution;
  final Map<String, dynamic> context_;
  final Widget? fallback;

  @override
  Widget build(BuildContext context) {
    try {
      return contribution.builder(
        context,
        Map<String, dynamic>.unmodifiable(context_),
      );
    } catch (_) {
      return fallback ?? const SizedBox.shrink();
    }
  }
}

class _ChainSlot extends StatelessWidget {
  const _ChainSlot({
    required this.contributions,
    required this.context_,
    this.fallback,
  });

  final List<UiSlotContribution> contributions;
  final Map<String, dynamic> context_;
  final Widget? fallback;

  @override
  Widget build(BuildContext context) {
    for (final c in contributions) {
      try {
        return c.builder(context, Map<String, dynamic>.unmodifiable(context_));
      } catch (_) {
        continue;
      }
    }
    return fallback ?? const SizedBox.shrink();
  }
}

UiSlotRegistry? _maybeRegistry(BuildContext context) {
  try {
    return context.watch<UiSlotRegistry>();
  } catch (_) {
    return null;
  }
}
