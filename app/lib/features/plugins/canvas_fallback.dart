import 'package:flutter/material.dart';

import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';

/// The fallback view model for a canvas object whose plugin is missing.
///
/// The daemon builds it from the stored object (`pluginfabric.FallbackFor`), and
/// the canvas shows it instead of the plugin's own renderer. Uninstalling a
/// plugin deletes neither the object nor its references, so the title, the
/// source and the text stay readable and the data stays exportable
/// (docs/specs/0032-ui-extension-protocol.md, Canvas plugin objects).
class CanvasFallbackModel {
  const CanvasFallbackModel({
    this.id = '',
    this.type = '',
    this.pluginId = '',
    this.title = '',
    this.source = '',
    this.summary = '',
    this.schemaVersion = 0,
    this.revision = 0,
    this.data = const {},
  });

  /// Reads a fallback model from daemon JSON. A field of the wrong type is
  /// dropped instead of thrown, so a damaged object still shows what it can.
  factory CanvasFallbackModel.fromJson(Object? raw) {
    final json = raw is Map ? raw.cast<Object?, Object?>() : const <Object?, Object?>{};
    return CanvasFallbackModel(
      id: _string(json['id']),
      type: _string(json['type']),
      pluginId: _string(json['pluginId']),
      title: _string(json['title']),
      source: _string(json['source']),
      summary: _string(json['summary']),
      schemaVersion: _int(json['schemaVersion']),
      revision: _int(json['revision']),
      data: _data(json['data']),
    );
  }

  final String id, type, pluginId, title, source, summary;
  final int schemaVersion, revision;
  final Map<String, dynamic> data;

  /// Nothing readable is left: the object carried no identity and no content.
  bool get isEmpty => id.isEmpty && title.isEmpty && source.isEmpty && summary.isEmpty && data.isEmpty;

  /// Everything the user can still keep, including the object identity.
  Map<String, dynamic> exportData() => {
    'id': id,
    'type': type,
    'pluginId': pluginId,
    'schemaVersion': schemaVersion,
    'revision': revision,
    'title': title,
    'source': source,
    'summary': summary,
    'data': data,
  };
}

String _string(Object? value) => value is String ? value : '';

int _int(Object? value) => value is num && value.isFinite ? value.toInt() : 0;

Map<String, dynamic> _data(Object? value) =>
    value is Map ? value.map((key, entry) => MapEntry('$key', entry)) : const {};

/// Draws a canvas object whose plugin is missing: the title, where it came
/// from, a text summary and, when the plugin left structured data, an export of
/// it. It never needs the plugin, a `BuildContext` of the canvas or any state
/// beyond [model].
class CanvasFallback extends StatelessWidget {
  const CanvasFallback({super.key, required this.model, this.onExport});

  final CanvasFallbackModel model;

  /// Receives the exportable data. Without it the export action is hidden.
  final ValueChanged<Map<String, dynamic>>? onExport;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final muted = tt.labelSmall?.copyWith(color: cs.onSurfaceVariant);

    if (model.isEmpty) {
      return _frame(
        context,
        Row(
          children: [
            Icon(Icons.help_outline_rounded, size: 18, color: cs.onSurfaceVariant),
            const SizedBox(width: Space.sm),
            Expanded(child: Text(l10n.pluginCanvasFallbackUnreadable, style: muted)),
          ],
        ),
      );
    }

    final title = model.title.isEmpty ? l10n.pluginCanvasFallbackUntitled : model.title;
    return _frame(
      context,
      Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(Icons.extension_off_rounded, size: 18, color: cs.onSurfaceVariant),
              const SizedBox(width: Space.sm),
              Expanded(child: Text(l10n.pluginCanvasFallbackMissing, style: muted)),
            ],
          ),
          const SizedBox(height: Space.sm),
          Text(title, style: tt.titleSmall),
          if (model.source.isNotEmpty) ...[
            const SizedBox(height: Space.xs),
            Row(
              children: [
                Icon(Icons.link_rounded, size: 16, color: cs.onSurfaceVariant),
                const SizedBox(width: Space.xs),
                Expanded(child: Text(model.source, style: muted)),
              ],
            ),
          ],
          const SizedBox(height: Space.sm),
          Text(
            model.summary.isEmpty ? l10n.pluginCanvasFallbackEmpty : model.summary,
            style: tt.bodySmall?.copyWith(color: model.summary.isEmpty ? cs.onSurfaceVariant : null),
            maxLines: 4,
            overflow: TextOverflow.ellipsis,
          ),
          if (onExport != null && model.data.isNotEmpty)
            Align(
              alignment: Alignment.centerRight,
              child: TextButton.icon(
                onPressed: () => onExport!(model.exportData()),
                icon: const Icon(Icons.download_rounded, size: 18),
                label: Text(l10n.pluginCanvasFallbackExport),
              ),
            ),
        ],
      ),
    );
  }

  Widget _frame(BuildContext context, Widget child) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(Space.lg),
      decoration: BoxDecoration(
        color: cs.surfaceContainerHigh,
        borderRadius: Corners.all(Corners.lg),
        border: Border.all(color: cs.outlineVariant),
      ),
      child: child,
    );
  }
}
