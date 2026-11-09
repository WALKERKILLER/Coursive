part of 'study_canvas.dart';

// Render zoomed-out page text above logical-pixel resolution, while bounding
// the full viewport raster to 12 megapixels (about 48 MB of pixel storage).
double _selectionRasterRatio(Size viewport) => math.min(
  4.0,
  math.sqrt(12000000 / math.max(1, viewport.width * viewport.height)),
);

Rect _normalizeRegion(Rect r, Rect parent) => Rect.fromLTWH(
  ((r.left - parent.left) / parent.width).clamp(0, 1),
  ((r.top - parent.top) / parent.height).clamp(0, 1),
  (r.width / parent.width).clamp(.000001, 1),
  (r.height / parent.height).clamp(.000001, 1),
);

Rect _sourceBounds(StudyCard card) {
  if (card.type != 'pdfdoc') return card.rect;
  final area = Rect.fromLTWH(
    card.rect.left,
    card.rect.top + 36,
    card.rect.width,
    math.max(1, card.rect.height - 72),
  );
  var width = area.width, height = width / card.pageAspect;
  if (height > area.height) {
    height = area.height;
    width = height * card.pageAspect;
  }
  return Rect.fromCenter(center: area.center, width: width, height: height);
}

extension _StudyAgentUI on StudyCanvasState {
  Future<void> _editKnowledge(StudyCard original) async {
    final card = original.copy();
    final title = TextEditingController(text: card.title);
    final controls = <TextEditingController>[];
    final saved = await showDialog<bool>(
      context: context,
      builder: (c) => AlertDialog(
        title: const Text('编辑知识卡片'),
        content: SizedBox(
          width: 650,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(
                  controller: title,
                  maxLength: 240,
                  decoration: const InputDecoration(labelText: '标题'),
                ),
                for (final b in card.blocks)
                  Padding(
                    padding: const EdgeInsets.only(top: 12),
                    child: Builder(
                      builder: (_) {
                        final value = b.type == 'table'
                            ? [
                                b.columns,
                                ...b.rows,
                              ].map((r) => r.join('\t')).join('\n')
                            : b.type == 'list' || b.type == 'steps'
                            ? b.items.join('\n')
                            : b.text;
                        final ctl = TextEditingController(text: value);
                        controls.add(ctl);
                        return Column(
                          children: [
                            TextField(
                              controller: ctl,
                              minLines: 2,
                              maxLines: 9,
                              maxLength: 12000,
                              decoration: InputDecoration(
                                labelText: b.type == 'table'
                                    ? '表格：首行为列名，列用 Tab 分隔'
                                    : b.type == 'list' || b.type == 'steps'
                                    ? '每行一项'
                                    : b.type,
                              ),
                              onChanged: (v) {
                                if (b.type == 'table') {
                                  final rows = v
                                      .split('\n')
                                      .map((s) => s.split('\t'))
                                      .toList();
                                  b.columns = rows.first;
                                  b.rows = rows.skip(1).toList();
                                } else if (b.type == 'list' ||
                                    b.type == 'steps') {
                                  b.items = v.split('\n');
                                } else {
                                  b.text = v;
                                }
                              },
                            ),
                            if (b.type == 'exercise') ...[
                              TextFormField(
                                initialValue: b.hint,
                                decoration: const InputDecoration(
                                  labelText: '提示',
                                ),
                                onChanged: (v) => b.hint = v,
                              ),
                              TextFormField(
                                initialValue: b.answer,
                                minLines: 2,
                                maxLines: 5,
                                decoration: const InputDecoration(
                                  labelText: '答案与解析',
                                ),
                                onChanged: (v) => b.answer = v,
                              ),
                            ],
                          ],
                        );
                      },
                    ),
                  ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(c),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(c, true),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    Future.delayed(const Duration(milliseconds: 400), () {
      title.dispose();
      for (final ctl in controls) ctl.dispose();
    });
    if (!mounted || saved != true) return;
    final d = document!;
    if (!d.editable(original.layer)) {
      _toast('该图层不可编辑');
      return;
    }
    if (card.blocks.any(
      (b) =>
          b.type == 'table' &&
          (b.columns.isEmpty ||
              b.columns.length > 8 ||
              b.rows.any((r) => r.length != b.columns.length)),
    )) {
      _toast('表格各行的列数须相同');
      return;
    }
    d.checkpoint();
    original
      ..title = title.text
      ..blocks = card.blocks;
    d.changed();
  }

  Future<void> _connectionSettings() async {
    await showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (c) => StatefulBuilder(
        builder: (c, refresh) => SizedBox(
          height: 420,
          child: Column(
            children: [
              const ListTile(
                title: Text('知识关联'),
                subtitle: Text('关系随对象移动；更改与删除都可撤销。'),
              ),
              Expanded(
                child: ListView.builder(
                  itemCount: document!.connections.length,
                  itemBuilder: (c, i) {
                    final link = document!.connections[i];
                    final a = document!.cards
                            .where((v) => v.id == link.from)
                            .firstOrNull,
                        b = document!.cards
                            .where((v) => v.id == link.to)
                            .firstOrNull;
                    final editable =
                        a != null &&
                        b != null &&
                        document!.editable(a.layer) &&
                        document!.editable(b.layer);
                    return ListTile(
                      title: Text(
                        '${a?.title ?? link.from} → ${b?.title ?? link.to}',
                      ),
                      subtitle: Text(link.label),
                      onTap: editable
                          ? () async {
                              final input = TextEditingController(
                                text: link.label,
                              );
                              final label = await showDialog<String>(
                                context: c,
                                builder: (d) => AlertDialog(
                                  title: const Text('关系说明'),
                                  content: TextField(
                                    controller: input,
                                    maxLength: 240,
                                  ),
                                  actions: [
                                    TextButton(
                                      onPressed: () => Navigator.pop(d),
                                      child: const Text('取消'),
                                    ),
                                    FilledButton(
                                      onPressed: () =>
                                          Navigator.pop(d, input.text),
                                      child: const Text('保存'),
                                    ),
                                  ],
                                ),
                              );
                              Future.delayed(
                                const Duration(milliseconds: 400),
                                input.dispose,
                              );
                              if (!mounted || label == null) return;
                              document!.checkpoint();
                              link.label = label;
                              document!.changed();
                              if (c.mounted) refresh(() {});
                            }
                          : null,
                      trailing: IconButton(
                        tooltip: '移除关联',
                        onPressed: editable
                            ? () {
                                document!.checkpoint();
                                document!.connections.remove(link);
                                document!.changed();
                                refresh(() {});
                              }
                            : null,
                        icon: const Icon(Icons.link_off),
                      ),
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ConnectionPainter extends CustomPainter {
  _ConnectionPainter(this.document, this.stage, Listenable repaint, this.chrome)
    : super(repaint: repaint);
  final StudyDocument document;
  final Rect stage;
  final StudyChrome chrome;
  @override
  void paint(Canvas canvas, Size size) {
    final d = document,
        rects = {
          for (final c in document.cards)
            if (d.visible(c.layer)) c.id: c.rect,
          for (final o in document.pluginObjects) o.id: o.rect,
        };
    canvas.save();
    canvas.translate(-stage.left, -stage.top);
    final viewport = stage;
    Offset edge(Rect r, Offset toward) {
      final delta = toward - r.center;
      final scale = math.min(
        delta.dx.abs() < .001 ? double.infinity : r.width / 2 / delta.dx.abs(),
        delta.dy.abs() < .001 ? double.infinity : r.height / 2 / delta.dy.abs(),
      );
      return scale.isFinite ? r.center + delta * scale : r.center;
    }

    for (final link in d.connections) {
      final a = rects[link.from], b = rects[link.to];
      if (a == null || b == null) continue;
      if (!Rect.fromPoints(a.center, b.center).inflate(60).overlaps(viewport))
        continue;
      final start = edge(a, b.center),
          end = edge(b, a.center),
          mid = (start + end) / 2;
      final path = Path()
        ..moveTo(start.dx, start.dy)
        ..cubicTo(mid.dx, start.dy, mid.dx, end.dy, end.dx, end.dy);
      canvas.drawPath(
        path,
        Paint()
          ..color = chrome.accent.withValues(alpha: .65)
          ..style = PaintingStyle.stroke
          ..strokeWidth = 2 / d.zoom,
      );
      canvas.drawCircle(end, 4 / d.zoom, Paint()..color = chrome.accent);
      final label = TextPainter(
        text: TextSpan(
          text: link.label,
          style: TextStyle(
            color: chrome.onAccentSoft,
            fontSize: 12 / d.zoom,
            backgroundColor: chrome.accentSoft,
          ),
        ),
        textDirection: TextDirection.ltr,
        maxLines: 2,
      )..layout(maxWidth: 180 / d.zoom);
      label.paint(canvas, mid - Offset(label.width / 2, label.height / 2));
      label.dispose();
    }
    canvas.restore();
  }

  @override
  bool shouldRepaint(_ConnectionPainter old) => true;
  @override
  bool shouldRebuildSemantics(_ConnectionPainter old) => false;
}

class _KnowledgeBody extends StatelessWidget {
  const _KnowledgeBody({
    required this.card,
    required this.onSource,
    required this.onBeginAnswer,
    required this.onAnswer,
    required this.editable,
  });
  final StudyCard card;
  final ValueChanged<StudyAnchor> onSource;
  final VoidCallback onBeginAnswer;
  final void Function(int, String) onAnswer;
  final bool editable;
  @override
  Widget build(BuildContext context) => Material(
    type: MaterialType.transparency,
    child: SingleChildScrollView(
      padding: const EdgeInsets.all(18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          for (final b in card.blocks)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: switch (b.type) {
                'heading' => SelectableText(
                  b.text,
                  style: TextStyle(
                    fontSize: 19,
                    fontWeight: FontWeight.bold,
                    color: StudyChrome.of(context).ink,
                  ),
                ),
                'list' || 'steps' => Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    for (var i = 0; i < b.items.length; i++)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 6),
                        child: SelectableText(
                          '${b.type == 'steps' ? '${i + 1}.' : '•'} ${b.items[i]}',
                        ),
                      ),
                  ],
                ),
                'table' => SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: DataTable(
                    columns: [
                      for (final c in b.columns) DataColumn(label: Text(c)),
                    ],
                    rows: [
                      for (final r in b.rows)
                        DataRow(
                          cells: [
                            for (final v in r) DataCell(SelectableText(v)),
                          ],
                        ),
                    ],
                  ),
                ),
                'code' => Container(
                  width: double.infinity,
                  padding: const EdgeInsets.all(12),
                  color: StudyChrome.of(context).surfaceSoft,
                  child: SelectableText(
                    '${b.language}\n${b.text}',
                    style: const TextStyle(
                      fontFamily: 'monospace',
                      fontSize: 13,
                    ),
                  ),
                ),
                'math' => SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: Math.tex(
                    b.text,
                    onError: (_, _) => SelectableText(b.text),
                  ),
                ),
                'exercise' => Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SelectableText(
                      b.text,
                      style: const TextStyle(fontWeight: FontWeight.w600),
                    ),
                    _StudyAnswer(
                      key: ValueKey(
                        '${card.id}/answer/${card.blocks.indexOf(b)}',
                      ),
                      value: card.responses['${card.blocks.indexOf(b)}'] ?? '',
                      enabled: editable,
                      onBegin: onBeginAnswer,
                      onChanged: (v) => onAnswer(card.blocks.indexOf(b), v),
                    ),
                    if (b.hint.isNotEmpty)
                      ExpansionTile(
                        title: const Text('提示'),
                        children: [
                          Padding(
                            padding: const EdgeInsets.all(8),
                            child: SelectableText(b.hint),
                          ),
                        ],
                      ),
                    if (b.answer.isNotEmpty)
                      ExpansionTile(
                        title: const Text('答案与解析'),
                        children: [
                          Padding(
                            padding: const EdgeInsets.all(8),
                            child: SelectableText(b.answer),
                          ),
                        ],
                      ),
                  ],
                ),
                _ => SelectableText(b.text),
              },
            ),
          if (card.sources.isNotEmpty) ...[
            const Divider(),
            Text('引用来源', style: Theme.of(context).textTheme.labelMedium),
          ],
          for (final a in card.sources)
            TextButton.icon(
              style: TextButton.styleFrom(alignment: Alignment.centerLeft),
              onPressed: () => onSource(a),
              icon: const Icon(Icons.link, size: 15),
              label: Text(
                a.kind == 'video'
                    ? '视频 · ${fmtClock(a.timeStartMs)}–${fmtClock(a.timeEndMs)}'
                    : a.kind == 'ink'
                    ? '手写选区 · ${a.strokeIds.length} 段'
                    : '${a.kind.toUpperCase()}${a.page > 0 ? ' · 第 ${a.page} 页' : ''} · ${a.quote.substring(0, math.min(50, a.quote.length))}',
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ),
        ],
      ),
    ),
  );
}

class _StudyAnswer extends StatefulWidget {
  const _StudyAnswer({
    super.key,
    required this.value,
    required this.enabled,
    required this.onBegin,
    required this.onChanged,
  });
  final String value;
  final bool enabled;
  final VoidCallback onBegin;
  final ValueChanged<String> onChanged;
  @override
  State<_StudyAnswer> createState() => _StudyAnswerState();
}

class _StudyAnswerState extends State<_StudyAnswer> {
  late final controller = TextEditingController(text: widget.value);
  final focus = FocusNode();
  @override
  void initState() {
    super.initState();
    focus.addListener(_focusChanged);
  }

  void _focusChanged() {
    if (focus.hasFocus) widget.onBegin();
  }

  @override
  void didUpdateWidget(_StudyAnswer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (controller.text != widget.value) {
      controller.value = TextEditingValue(
        text: widget.value,
        selection: TextSelection.collapsed(offset: widget.value.length),
      );
    }
  }

  @override
  void dispose() {
    focus.dispose();
    controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => TextField(
    controller: controller,
    focusNode: focus,
    enabled: widget.enabled,
    minLines: 2,
    maxLines: 6,
    maxLength: 4000,
    decoration: const InputDecoration(labelText: '我的作答', helperText: '随工作区保存'),
    onChanged: widget.onChanged,
  );
}
