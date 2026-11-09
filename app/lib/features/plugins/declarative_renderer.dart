import 'package:flutter/material.dart';

import '../../core/plugin_navigation.dart';
import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';

/// A versioned, host-owned renderer for plugin data trees. Plugins submit
/// JSON; the host maps the whitelisted node types to native widgets and never
/// evaluates plugin expressions or Dart code.
class DeclarativeRenderer extends StatefulWidget {
  const DeclarativeRenderer({
    super.key,
    required this.document,
    this.state = const {},
    this.onAction,
    this.maxDepth = 24,
    this.maxNodes = 2000,
    this.maxText = 20000,
  });

  final Map<String, dynamic> document;
  final Map<String, dynamic> state;
  final void Function(String action, Map<String, dynamic> args)? onAction;
  final int maxDepth, maxNodes, maxText;

  @override
  State<DeclarativeRenderer> createState() => _DeclarativeRendererState();
}

class _DeclarativeRendererState extends State<DeclarativeRenderer> {
  final _values = <String, dynamic>{};
  final _controllers = <String, TextEditingController>{};

  @override
  void dispose() {
    for (final c in _controllers.values) {
      c.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    try {
      final version = widget.document['schemaVersion'];
      if (version != null && version != 1) {
        throw const DeclarativeError('unsupported schemaVersion');
      }
      final body = widget.document['body'];
      if (body is! Map) throw const DeclarativeError('missing body');
      final count = _countNodes(body);
      if (count > widget.maxNodes) {
        throw DeclarativeError('too many nodes: $count');
      }
      final renderer = _NodeRenderer(
        state: {...widget.state, ..._values},
        maxDepth: widget.maxDepth,
        maxText: widget.maxText,
        onAction: widget.onAction,
        controllers: _controllers,
        values: _values,
        onChanged: () => setState(() {}),
      );
      return renderer.build(context, body.cast<String, dynamic>(), 0);
    } on DeclarativeError catch (e) {
      return DeclarativeErrorView(message: e.message);
    }
  }

  int _countNodes(Object? node, [int depth = 0]) {
    if (depth > widget.maxDepth) throw DeclarativeError('document is too deep');
    if (node is List) {
      var n = 0;
      for (final x in node) {
        n += _countNodes(x, depth + 1);
      }
      return n;
    }
    if (node is Map) {
      var n = 1;
      for (final key in const ['children', 'items', 'tabs']) {
        n += _countNodes(node[key], depth + 1);
      }
      return n;
    }
    return 0;
  }
}

class DeclarativeError implements Exception {
  const DeclarativeError(this.message);
  final String message;
}

class DeclarativeErrorView extends StatelessWidget {
  const DeclarativeErrorView({super.key, required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(Space.lg),
      decoration: BoxDecoration(
        color: cs.errorContainer,
        borderRadius: BorderRadius.circular(Corners.lg),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline_rounded, color: cs.onErrorContainer),
          const SizedBox(width: Space.sm),
          Expanded(
            child: Text(
              l10n.pluginViewUnsupported,
              style: TextStyle(color: cs.onErrorContainer),
            ),
          ),
        ],
      ),
    );
  }
}

class _NodeRenderer {
  _NodeRenderer({
    required this.state,
    required this.maxDepth,
    required this.maxText,
    required this.onAction,
    required this.controllers,
    required this.values,
    required this.onChanged,
  });

  final Map<String, dynamic> state;
  final int maxDepth, maxText;
  final void Function(String action, Map<String, dynamic> args)? onAction;
  final Map<String, TextEditingController> controllers;
  final Map<String, dynamic> values;
  final VoidCallback onChanged;

  Widget build(BuildContext context, Map<String, dynamic> node, int depth) {
    if (depth > maxDepth) throw const DeclarativeError('document is too deep');
    final type = '${node['type'] ?? ''}';
    final children = _children(node);
    switch (type) {
      case 'column':
        return Column(
          crossAxisAlignment: _cross(node['align']),
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final child in children) build(context, child, depth + 1),
          ],
        );
      case 'row':
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final child in children)
              Flexible(child: build(context, child, depth + 1)),
          ],
        );
      case 'card':
        return Card(
          margin: const EdgeInsets.symmetric(vertical: Space.xs),
          child: Padding(
            padding: const EdgeInsets.all(Space.lg),
            child: children.isEmpty
                ? const SizedBox.shrink()
                : build(context, children.first, depth + 1),
          ),
        );
      case 'text':
        return _text(context, node);
      case 'button':
        return _button(context, node);
      case 'input':
        return _input(context, node);
      case 'list':
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final child in children) build(context, child, depth + 1),
          ],
        );
      case 'table':
        return _table(context, node, depth);
      case 'timeline':
        return _timeline(context, node, depth);
      case 'tabs':
        return _tabs(context, node, depth);
      case 'spacer':
        return const SizedBox(height: Space.md);
      case 'divider':
        return const Divider();
      case 'chart':
      case 'math':
      case 'mediaRef':
      case 'sourceRef':
      case 'canvasGraph':
        return _unsupported(context, type);
      default:
        return _unsupported(context, type.isEmpty ? 'unknown' : type);
    }
  }

  List<Map<String, dynamic>> _children(Map<String, dynamic> node) {
    final raw = node['children'] ?? node['items'];
    if (raw is! List) return const [];
    return [
      for (final child in raw)
        if (child is Map) child.cast<String, dynamic>(),
    ];
  }

  Widget _text(BuildContext context, Map<String, dynamic> node) {
    final value = '${_resolve(node['text'] ?? node['ref'] ?? '')}';
    if (value.length > maxText) {
      throw DeclarativeError('text is too long: ${value.length}');
    }
    final tt = Theme.of(context).textTheme;
    final style = switch ('${node['variant'] ?? 'body'}') {
      'title' => tt.titleLarge,
      'subtitle' => tt.titleSmall,
      'label' => tt.labelLarge,
      _ => tt.bodyMedium,
    };
    return Text(value, style: style, textAlign: _align(node['align']));
  }

  Widget _button(BuildContext context, Map<String, dynamic> node) {
    final label = '${_resolve(node['text'] ?? '')}';
    final action = '${node['action'] ?? node['onPress']?['action'] ?? ''}';
    final rawArgs =
        node['args'] ?? node['onPress']?['args'] ?? const <String, dynamic>{};
    void press() {
      if (action.isEmpty) return;
      final args = _resolve(rawArgs);
      onAction?.call(
        action,
        args is Map ? args.cast<String, dynamic>() : const {},
      );
    }

    final child = Text(label);
    final style = '${node['variant'] ?? 'filled'}';
    return switch (style) {
      'tonal' => FilledButton.tonal(onPressed: press, child: child),
      'text' => TextButton(onPressed: press, child: child),
      _ => FilledButton(onPressed: press, child: child),
    };
  }

  Widget _input(BuildContext context, Map<String, dynamic> node) {
    final id = '${node['id'] ?? node['ref'] ?? 'input'}';
    final controller = controllers.putIfAbsent(
      id,
      () => TextEditingController(text: '${_resolve(node['value'] ?? '')}'),
    );
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: Space.xs),
      child: TextField(
        controller: controller,
        decoration: InputDecoration(
          labelText: '${_resolve(node['label'] ?? '')}',
          hintText: '${_resolve(node['hint'] ?? '')}',
        ),
        onChanged: (value) {
          values[id] = value;
          onChanged();
        },
      ),
    );
  }

  Widget _table(BuildContext context, Map<String, dynamic> node, int depth) {
    final columns = [
      for (final c in (node['columns'] as List?) ?? const []) '${_resolve(c)}',
    ];
    final rows = (node['rows'] as List?) ?? const [];
    if (columns.isEmpty || rows.isEmpty) return const SizedBox.shrink();
    return Table(
      border: TableBorder.all(
        color: Theme.of(context).colorScheme.outlineVariant,
      ),
      children: [
        TableRow(
          children: [
            for (final c in columns)
              Padding(
                padding: const EdgeInsets.all(Space.sm),
                child: Text(c, style: Theme.of(context).textTheme.labelLarge),
              ),
          ],
        ),
        for (final row in rows)
          if (row is List)
            TableRow(
              children: [
                for (final cell in row)
                  Padding(
                    padding: const EdgeInsets.all(Space.sm),
                    child: Text('${_resolve(cell)}'),
                  ),
              ],
            ),
      ],
    );
  }

  Widget _timeline(BuildContext context, Map<String, dynamic> node, int depth) {
    final items = (node['items'] as List?) ?? const [];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        for (final item in items)
          if (item is Map)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: Space.xs),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 56,
                    child: Text(
                      '${_resolve(item['time'] ?? '')}',
                      style: Theme.of(context).textTheme.labelSmall,
                    ),
                  ),
                  Expanded(child: Text('${_resolve(item['text'] ?? '')}')),
                ],
              ),
            ),
      ],
    );
  }

  Widget _tabs(BuildContext context, Map<String, dynamic> node, int depth) {
    final tabs = (node['tabs'] as List?) ?? const [];
    if (tabs.isEmpty) return const SizedBox.shrink();
    final labels = [
      for (final tab in tabs)
        if (tab is Map) '${_resolve(tab['label'] ?? '')}',
    ];
    final views = [
      for (final tab in tabs)
        if (tab is Map) tab,
    ];
    return DefaultTabController(
      length: tabs.length,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TabBar(tabs: [for (final label in labels) Tab(text: label)]),
          SizedBox(
            height: 240,
            child: TabBarView(
              children: [
                for (final tab in views)
                  SingleChildScrollView(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        for (final child
                            in ((tab['children'] as List?) ?? const []))
                          if (child is Map)
                            build(
                              context,
                              child.cast<String, dynamic>(),
                              depth + 1,
                            ),
                      ],
                    ),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _unsupported(BuildContext context, String type) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.symmetric(vertical: Space.xs),
      padding: const EdgeInsets.all(Space.md),
      decoration: BoxDecoration(
        color: cs.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(Corners.md),
      ),
      child: Row(
        children: [
          Icon(Icons.widgets_outlined, size: 18, color: cs.onSurfaceVariant),
          const SizedBox(width: Space.sm),
          Expanded(
            child: Text(
              l10n.pluginViewUnknown(type),
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ),
        ],
      ),
    );
  }

  CrossAxisAlignment _cross(Object? value) => switch ('$value') {
    'start' => CrossAxisAlignment.start,
    'end' => CrossAxisAlignment.end,
    _ => CrossAxisAlignment.stretch,
  };

  TextAlign _align(Object? value) => switch ('$value') {
    'center' => TextAlign.center,
    'end' => TextAlign.end,
    _ => TextAlign.start,
  };

  Object? _resolve(Object? value) {
    if (value is String) {
      final match = RegExp(r'^\$\{([^}]+)\}$').firstMatch(value);
      if (match != null) return _readPath(match.group(1)!);
      return value;
    }
    if (value is List) return [for (final x in value) _resolve(x)];
    if (value is Map) {
      if (value['ref'] is String) return _readPath('${value['ref']}');
      return {for (final e in value.entries) '${e.key}': _resolve(e.value)};
    }
    return value;
  }

  Object? _readPath(String path) {
    Object? current = state;
    for (final part in path.split('.')) {
      if (current is Map) {
        current = current[part];
      } else if (current is List) {
        final index = int.tryParse(part);
        current = index != null && index >= 0 && index < current.length
            ? current[index]
            : null;
      } else {
        return null;
      }
    }
    return current;
  }
}

/// Registers the declarative renderer as a page builder for plugin pages.
Widget declarativePageBuilder(
  BuildContext context,
  PluginPageContribution page,
) {
  final document = page.view['document'] is Map
      ? (page.view['document'] as Map).cast<String, dynamic>()
      : page.view;
  return DeclarativeRenderer(
    document: document,
    onAction: (action, args) {
      // The action bridge is wired by the page host; a page without a bridge
      // still renders and its buttons stay inert instead of crashing.
    },
  );
}
