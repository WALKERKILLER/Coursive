part of 'study_canvas.dart';

/// The slot target plugin canvas tools render in (`uiSlotCatalog`).
const _canvasTools = 'canvas.toolbar.tools';

// Plugin objects on the study canvas (docs/specs/0032-ui-extension-protocol.md,
// Canvas plugin objects). They are drawn between the cards and the ink and
// never hit-tested, so a plugin cannot take a stroke or a selection; a tool
// changes them only through a checked batch that is one undo entry.
extension _StudyPlugins on StudyCanvasState {
  /// The enabled plugin ids, or none outside the app shell.
  Set<String> _activePlugins() {
    try {
      return {
        for (final p in context.watch<PluginStore>().active) '${p['id']}',
      };
    } catch (_) {
      return const {};
    }
  }

  /// The canvas tools of enabled plugins, or none outside the app shell.
  List<UiSlotContribution> _canvasToolContributions(BuildContext context) {
    try {
      return context.watch<UiSlotRegistry>().contributions(_canvasTools);
    } catch (_) {
      return const [];
    }
  }

  /// The plugin sheet is offered when a plugin contributes a canvas tool or
  /// the canvas holds plugin objects.
  bool get _pluginsOffered =>
      document!.pluginObjects.isNotEmpty || _hasCanvasTools;

  String _pluginTitle(String id) {
    try {
      return '${context.read<PluginStore>().byId(id)?['title'] ?? id}';
    } catch (_) {
      return id;
    }
  }

  Widget _pluginObject(StudyPluginObject o, Rect stage, bool active) =>
      Positioned(
        key: ValueKey('study-plugin/${o.id}'),
        left: o.rect.left - stage.left,
        top: o.rect.top - stage.top,
        width: o.rect.width,
        height: o.rect.height,
        child: IgnorePointer(
          child: RepaintBoundary(
            child: RotatedBox(
              quarterTurns: o.quarterTurns,
              child: ClipRect(
                child: active
                    ? _StudyPluginCard(
                        object: o,
                        plugin: _pluginTitle(o.pluginId),
                      )
                    : SingleChildScrollView(
                        physics: const NeverScrollableScrollPhysics(),
                        child: CanvasFallback(model: o.fallback),
                      ),
              ),
            ),
          ),
        ),
      );

  /// What a canvas tool may read as `context.*`: this lecture, the revision a
  /// batch is checked against, the centre of the view in canvas units and the
  /// plugin's own objects. Nothing of another plugin is passed.
  Map<String, dynamic> _pluginContext(String pluginId) {
    final d = document!, centre = _world(_viewport.center(Offset.zero));
    return {
      'lecture_id': widget.ctl.lectureId,
      'canvas_revision': d.pluginRevision,
      'canvas_x': centre.dx,
      'canvas_y': centre.dy,
      'canvas_objects': [
        for (final o in d.pluginObjects)
          if (o.pluginId == pluginId) {'id': o.id, 'revision': o.revision},
      ],
    };
  }

  void _plugins() {
    final d = document!;
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheet) => PluginViewPatchScope(
        onPatch: _pluginPatch,
        child: ListenableBuilder(
          listenable: d,
          builder: (sheet, _) {
            final tools = _canvasToolContributions(sheet);
            return SafeArea(
              child: ConstrainedBox(
                constraints: BoxConstraints(
                  maxHeight: MediaQuery.sizeOf(sheet).height * .75,
                ),
                child: ListView(
                  shrinkWrap: true,
                  padding: const EdgeInsets.only(bottom: Space.md),
                  children: [
                    ListTile(title: Text(l10n.studyPlugins)),
                    for (final c in tools)
                      Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: Space.md,
                          vertical: Space.xs,
                        ),
                        child: c.builder(sheet, _pluginContext(c.pluginId)),
                      ),
                    if (tools.isEmpty)
                      ListTile(
                        dense: true,
                        title: Text(l10n.studyPluginsNoTools),
                      ),
                    if (d.pluginObjects.isNotEmpty)
                      ListTile(title: Text(l10n.studyPluginObjects)),
                    for (final o in d.pluginObjects)
                      ListTile(
                        key: ValueKey('study-plugin-entry/${o.id}'),
                        leading: const Icon(Icons.extension_outlined),
                        title: Text(o.title),
                        subtitle: Text(
                          [
                            _pluginTitle(o.pluginId),
                            if (o.source.isNotEmpty) o.source,
                          ].join(' · '),
                        ),
                        onTap: () {
                          Navigator.pop(sheet);
                          _flyTo(
                            _viewport.center(Offset.zero) -
                                o.rect.center * d.zoom,
                            d.zoom,
                          );
                        },
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            IconButton(
                              tooltip: l10n.studyPluginObjectExport,
                              icon: const Icon(Icons.download_rounded),
                              onPressed: () => _exportPluginObject(o),
                            ),
                            IconButton(
                              tooltip: l10n.studyPluginObjectRemove,
                              icon: const Icon(Icons.delete_outline),
                              onPressed: () => d.removePluginObject(o.id),
                            ),
                          ],
                        ),
                      ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  /// Commits a tool's `canvas` batch: checked whole against the current plugin
  /// objects, shown for approval when it overwrites or deletes, and refused
  /// whole when the canvas changed in between.
  Future<void> _pluginPatch(String pluginId, PluginViewPatch patch) async {
    final d = document, raw = patch.patch['canvas'];
    if (d == null || raw == null) return;
    try {
      final batch = d.checkPluginBatch(pluginId, raw);
      if (batch.needsApproval && !await _approvePluginBatch(batch)) return;
      d.commitPluginBatch(batch);
    } on FormatException catch (e) {
      _toast(l10n.studyPluginBatchFailed(e.message));
    }
  }

  Future<bool> _approvePluginBatch(StudyPluginBatch batch) async {
    final approved = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.studyPluginBatchTitle(_pluginTitle(batch.pluginId))),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (batch.label.isNotEmpty) Text(batch.label),
              for (final o in batch.created)
                Text(l10n.studyPluginBatchCreate(o.title)),
              for (final o in batch.updated)
                Text(l10n.studyPluginBatchUpdate(o.title)),
              for (final o in batch.deleted)
                Text(l10n.studyPluginBatchDelete(o.title)),
              const SizedBox(height: Space.sm),
              Text(l10n.studyPluginBatchUndo),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: Text(l10n.pluginProposalReject),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: Text(l10n.studyPluginBatchApply),
          ),
        ],
      ),
    );
    return approved == true;
  }

  Future<void> _exportPluginObject(StudyPluginObject o) async {
    final dest = await getSaveLocation(suggestedName: '${o.title}.json');
    if (dest == null || !mounted) return;
    try {
      await File(dest.path)
          .writeAsString(const JsonEncoder.withIndent('  ').convert(o.json()));
      _toast(l10n.studyPluginObjectExported(o.title));
    } catch (e) {
      _toast(l10n.studyExportError('$e'));
    }
  }
}

/// A plugin object whose plugin is enabled: its title, text, source and owner,
/// drawn by the host from the stored content.
class _StudyPluginCard extends StatelessWidget {
  const _StudyPluginCard({required this.object, required this.plugin});

  final StudyPluginObject object;
  final String plugin;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme, tt = Theme.of(context).textTheme;
    final muted = tt.labelSmall?.copyWith(color: cs.onSurfaceVariant);
    return Container(
      padding: const EdgeInsets.all(Space.md),
      decoration: BoxDecoration(
        color: cs.surfaceContainerLow,
        borderRadius: Corners.all(Corners.lg),
        border: Border.all(color: cs.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.extension_outlined, size: 16, color: cs.primary),
              const SizedBox(width: Space.xs),
              Expanded(
                child: Text(
                  plugin,
                  style: muted,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
          const SizedBox(height: Space.xs),
          Text(
            object.title,
            style: tt.titleSmall,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
          ),
          if (object.text.isNotEmpty) ...[
            const SizedBox(height: Space.xs),
            Expanded(
              child: Text(
                object.text,
                style: tt.bodySmall,
                overflow: TextOverflow.fade,
              ),
            ),
          ] else
            const Spacer(),
          if (object.source.isNotEmpty)
            Text(
              object.source,
              style: muted,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
        ],
      ),
    );
  }
}
