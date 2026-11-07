part of 'study_canvas.dart';

extension _StudySelectionTools on StudyCanvasState {
  String _kindLabel(StudySelectionKind kind) => switch (kind) {
    StudySelectionKind.ink => l10n.studySelectInk,
    StudySelectionKind.text => l10n.studySelectText,
    StudySelectionKind.document => l10n.studySelectDocument,
    StudySelectionKind.video => l10n.studySelectVideo,
    StudySelectionKind.other => l10n.studySelectOther,
  };

  Future<void> _editSelectionFilters() async {
    final chosen = Set.of(_selectionKinds);
    final applied = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, update) => AlertDialog(
          title: Text(l10n.studySelectionFilters),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              for (final kind in StudySelectionKind.values)
                CheckboxListTile(
                  title: Text(_kindLabel(kind)),
                  value: chosen.contains(kind),
                  onChanged: (value) => update(() {
                    value == true ? chosen.add(kind) : chosen.remove(kind);
                  }),
                ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: Text(l10n.commonCancel),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: Text(l10n.commonSave),
            ),
          ],
        ),
      ),
    );
    if (!mounted || applied != true) return;
    _update(() {
      _selectionKinds
        ..clear()
        ..addAll(chosen);
      _selection.clear();
      _detail.clear();
      _selectionReady = false;
      _selectionBounds = _regionSelection = null;
      _cursor.value++;
    });
    final prefs = await SharedPreferences.getInstance();
    await prefs.setStringList(
      'study.selection.kinds',
      chosen.map((k) => k.name).toList(),
    );
  }

  /// Editing actions for a selection, behind the bar's 「…」.
  Widget _selectionActions() {
    final content = _selection.isNotEmpty || _detail.strokeIds.isNotEmpty;
    final ink = _detail.strokeIds.isNotEmpty;
    return PopupMenuButton<String>(
      key: const ValueKey('study-selection-actions'),
      tooltip: l10n.studySelectionMore,
      enabled: !_selectionBusy,
      icon: Icon(_selectionBusy ? Icons.hourglass_top : Icons.more_horiz),
      itemBuilder: (_) => [
        PopupMenuItem(value: 'copy', enabled: content, child: const Text('复制')),
        PopupMenuItem(
          value: 'paste',
          enabled: _copiedBounds != null,
          child: const Text('粘贴'),
        ),
        PopupMenuItem(
          value: 'duplicate',
          enabled: content,
          child: const Text('副本'),
        ),
        const PopupMenuDivider(),
        PopupMenuItem(value: 'rotate', child: Text(l10n.studyRotateSelection)),
        PopupMenuItem(
          value: 'shape',
          enabled: ink,
          child: Text(l10n.studyEditShape),
        ),
        PopupMenuItem(
          value: 'recognize',
          enabled: ink,
          child: Text(l10n.studyRecognizeInk),
        ),
        if (_linkStart != null)
          PopupMenuItem(
            value: 'cancel-link',
            child: Text(l10n.studyLinkCancel),
          ),
        PopupMenuItem(
          value: 'filters',
          child: Text(l10n.studySelectionFilters),
        ),
        const PopupMenuDivider(),
        PopupMenuItem(
          value: 'delete',
          enabled: content,
          child: const Text('删除'),
        ),
        PopupMenuItem(value: 'clear', child: Text(l10n.studySelectionClear)),
      ],
      onSelected: (value) {
        switch (value) {
          case 'copy':
            _copySelection();
          case 'paste':
            _pasteSelection();
          case 'duplicate':
            _copySelection();
            _pasteSelection();
          case 'rotate':
            _selectionJob(_rotateSelected);
          case 'shape':
            _selectionJob(_editSelectedShape);
          case 'recognize':
            _selectionJob(_recognizeSelectedInk);
          case 'cancel-link':
            _update(() {
              _linkStart = null;
              _linkStartBounds = null;
            });
          case 'filters':
            _editSelectionFilters();
          case 'delete':
            _deleteSelection();
          case 'clear':
            _clearSelection();
        }
      },
    );
  }

  Future<void> _selectionJob(Future<void> Function() work) async {
    if (_selectionBusy || document == null) return;
    _update(() => _selectionBusy = true);
    try {
      await work();
    } catch (e) {
      if (mounted) _toast(l10n.studySelectionError('$e'));
    } finally {
      if (mounted) _update(() => _selectionBusy = false);
    }
  }

  void _checkSelectionEdit(StudyDocument d, int edit) {
    if (!mounted || document != d || d.edit != edit)
      throw FormatException(l10n.studySelectionChanged);
    if (!d.editable(d.activeLayer)) throw FormatException(l10n.studyLocked);
    if (d.cards.length >= 1998) throw FormatException(l10n.studyCardLimit);
  }

  Future<List<StudyAnchor>> _selectionSources(
    Map<String, dynamic> query,
  ) async {
    if (!await flush()) throw FormatException(l10n.studySelectionChanged);
    if (!mounted) return [];
    query['revision'] = document!.revision;
    final context = await widget.api.studyContext(widget.ctl.lectureId, query);
    return [
      for (final s in context['sources'] as List? ?? [])
        StudyAnchor.fromJson(s['anchor'] as Map),
    ];
  }

  Future<Uint8List> _regionImage(Rect region) async {
    final d = document!;
    await WidgetsBinding.instance.endOfFrame;
    if (!mounted) throw FormatException(l10n.studySelectionChanged);
    final boundary =
        _capture.currentContext?.findRenderObject() as RenderRepaintBoundary?;
    if (boundary == null) throw FormatException(l10n.studyCaptureOutside);
    final screen = Rect.fromLTWH(
      region.left * d.zoom + d.camera.dx,
      region.top * d.zoom + d.camera.dy,
      region.width * d.zoom,
      region.height * d.zoom,
    );
    if (screen.width < 2 ||
        screen.height < 2 ||
        !(Offset.zero & boundary.size).inflate(.5).contains(screen.topLeft) ||
        !(Offset.zero & boundary.size).inflate(.5).contains(screen.bottomRight))
      throw FormatException(l10n.studyCaptureOutside);
    final ratio = _selectionRasterRatio(boundary.size);
    final full = await boundary.toImage(pixelRatio: ratio);
    try {
      final scale = math.min(
        ratio,
        1600 / math.max(screen.width, screen.height),
      );
      final size = Size(
        (screen.width * scale).roundToDouble(),
        (screen.height * scale).roundToDouble(),
      );
      final recorder = ui.PictureRecorder();
      Canvas(recorder).drawImageRect(
        full,
        Rect.fromLTWH(
          screen.left * ratio,
          screen.top * ratio,
          screen.width * ratio,
          screen.height * ratio,
        ),
        Offset.zero & size,
        Paint()..filterQuality = FilterQuality.medium,
      );
      final picture = recorder.endRecording();
      final cropped = await picture.toImage(
        size.width.toInt(),
        size.height.toInt(),
      );
      try {
        return (await cropped.toByteData(format: ui.ImageByteFormat.png))!
            .buffer
            .asUint8List();
      } finally {
        cropped.dispose();
        picture.dispose();
      }
    } finally {
      full.dispose();
    }
  }

  Future<void> _captureRegionCard() => _selectionJob(() async {
    final d = document!, edit = d.edit, region = _regionSelection;
    if (region == null) throw FormatException(l10n.studySelectionEmpty);
    final query = _selectedQuery();
    final bytes = await _regionImage(region);
    _checkSelectionEdit(d, edit);
    final sources = await _selectionSources(query);
    _checkSelectionEdit(d, edit);
    final imported = await widget.api.importStudyAsset(
      widget.ctl.lectureId,
      'selection-${studyId()}.png',
      Stream.value(bytes),
      bytes.length,
    );
    _checkSelectionEdit(d, edit);
    final card = StudyCard.fromJson(
      (imported['objects'] as List).single as Map,
    );
    card
      ..layer = d.activeLayer
      ..title = l10n.studyCaptureCard
      ..sources = sources;
    card.rect = _freeSpot(card.rect.size);
    d.checkpoint();
    d.cards.add(card);
    d.changed();
    _update(() {
      _captureLasso = false;
      _selectionReady = false;
      _tool = StudyTool.select;
      _selection
        ..clear()
        ..add(card.id);
    });
    await _focusSource(
      StudyAnchor(id: studyId(), objectId: card.id, kind: 'image'),
    );
    if (mounted) await flush();
  });

  Future<void> _linkSelection() async {
    final d = document!, edit = d.edit, bounds = _regionSelection;
    if (bounds == null) throw FormatException(l10n.studySelectionEmpty);
    final sources = await _selectionSources(_selectedQuery());
    _checkSelectionEdit(d, edit);
    if (sources.isEmpty) throw FormatException(l10n.studySelectionEmpty);
    if (_linkStart == null) {
      _linkStart = sources;
      _linkStartBounds = bounds;
      _toast(l10n.studyLinkPending);
      return;
    }
    bool exists(StudyAnchor a) => a.kind == 'ink'
        ? a.strokeIds.every((id) => d.strokes.any((s) => s.id == id))
        : d.cards.any((c) => c.id == a.objectId);
    if (!_linkStart!.every(exists))
      throw FormatException(l10n.studySelectionChanged);
    StudyCard endpoint(Rect at, List<StudyAnchor> targets) => StudyCard(
      id: studyId(),
      type: 'knowledge',
      layer: d.activeLayer,
      rect: Rect.fromLTWH(at.right + 24, at.top, 280, 180),
      title: l10n.studyLinkTitle,
      blocks: [StudyBlock(type: 'paragraph', text: l10n.studyLinkDescription)],
      sources: targets,
    );
    final a = endpoint(_linkStartBounds!, sources),
        b = endpoint(bounds, _linkStart!);
    d.checkpoint();
    d.cards.addAll([a, b]);
    d.connections.add(
      StudyConnection(studyId(), a.id, b.id, l10n.studyLinkTitle),
    );
    d.changed();
    _linkStart = null;
    _linkStartBounds = null;
    await flush();
  }

  Future<void> _rotateSelected() async {
    final d = document!;
    if (!d.canRotateSelection(_selection, _detail.strokeIds))
      throw FormatException(l10n.studyRotateUnsupported);
    final bounds = _boundsOfSelection();
    if (bounds == null) return;
    d.rotateSelection(
      Set.of(_selection),
      Set.of(_detail.strokeIds),
      bounds.center,
    );
    _update(() {
      _selectionBounds = _regionSelection = _boundsOfSelection();
      _detail.region = _regionSelection;
      _detail.objectId = null;
      _cursor.value++;
    });
    await flush();
  }

  Future<void> _editSelectedShape() async {
    final d = document!, edit = d.edit;
    final ink = d.strokes
        .where((s) => _detail.strokeIds.contains(s.id))
        .toList();
    if (_selection.isNotEmpty ||
        ink.isEmpty ||
        ink.map((s) => s.gesture).toSet().length != 1 ||
        ink.any((s) => !d.editable(s.layer)))
      throw FormatException(l10n.studyShapeUnavailable);
    if (d.strokes
        .where((s) => s.gesture == ink.first.gesture)
        .any((s) => !d.editable(s.layer) || !_detail.strokeIds.contains(s.id)))
      throw FormatException(l10n.studyShapeUnavailable);
    final points = <Offset>[];
    for (final s in ink) {
      for (final p in s.points) {
        if (points.isEmpty || points.last != p.p) points.add(p.p);
      }
    }
    if (points.length == 2) points.insert(1, (points.first + points.last) / 2);
    final shape = recognizeStudyShape(points, zoom: d.zoom);
    if (shape == null) throw FormatException(l10n.studyShapeUnavailable);
    final changed = await showDialog<List<Offset>>(
      context: context,
      builder: (_) => _ShapeAnchorDialog(shape: shape),
    );
    if (!mounted || changed == null) return;
    _checkSelectionEdit(d, edit);
    final original = ink.first;
    final replacement = StudyStroke(
      id: original.gesture,
      layer: original.layer,
      gesture: original.gesture,
      anchor: original.anchor,
      page: original.page,
      tool: original.tool,
      brush: original.brush,
      width: original.width,
      color: original.color,
      points: [
        for (final p in changed) InkPoint(p, original.points.first.pressure),
      ],
    );
    d.checkpoint();
    d.strokes.removeWhere((s) => s.gesture == original.gesture);
    final chunks = StudyDocument.chunks(replacement);
    d.strokes.addAll(chunks);
    d.changed();
    _update(() {
      _detail.strokeIds
        ..clear()
        ..addAll(chunks.map((s) => s.id));
      _selectionBounds = _regionSelection = _boundsOfSelection();
      _cursor.value++;
    });
    await flush();
  }

  Future<void> _recognizeSelectedInk() async {
    final d = document!, edit = d.edit;
    final ink = d.strokes
        .where((s) => _detail.strokeIds.contains(s.id) && d.strokeVisible(s))
        .toList();
    if (ink.isEmpty) throw FormatException(l10n.studySelectionEmpty);
    final query = <String, dynamic>{
      'objects': <String>[],
      'include_video': false,
      'selection': {'stroke_ids': ink.map((s) => s.id).toList()},
    };
    final consent = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.studyRecognizeInk),
        content: Text(l10n.studyRecognizeConsent),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text(l10n.studyRecognizeStart),
          ),
        ],
      ),
    );
    if (!mounted || consent != true) return;
    _checkSelectionEdit(d, edit);
    final sources = await _selectionSources(query);
    _checkSelectionEdit(d, edit);
    final bounds = ink
        .map((s) => s.bounds)
        .reduce((a, b) => a.expandToInclude(b))
        .inflate(8);
    Uint8List? bytes;
    for (final edge in [1000.0, 720.0, 480.0]) {
      final scale = math.min(1.0, edge / math.max(bounds.width, bounds.height));
      final recorder = ui.PictureRecorder();
      // A dedicated ink raster includes no unselected page or handwriting.
      final c = Canvas(recorder);
      c.drawColor(Colors.white, BlendMode.src);
      c.scale(scale);
      c.translate(-bounds.left, -bounds.top);
      for (final stroke in ink) {
        paintInk(
          c,
          stroke.points,
          0xff111111,
          stroke.width,
          stroke.tool,
          brush: stroke.brush,
        );
      }
      final picture = recorder.endRecording();
      final image = await picture.toImage(
        math.max(1, (bounds.width * scale).ceil()),
        math.max(1, (bounds.height * scale).ceil()),
      );
      try {
        bytes = (await image.toByteData(format: ui.ImageByteFormat.png))!.buffer
            .asUint8List();
      } finally {
        image.dispose();
        picture.dispose();
      }
      if (bytes.length <= 450000) break;
    }
    if (bytes == null || bytes.length > 450000)
      throw FormatException(l10n.studySelectionImageLimit);
    _checkSelectionEdit(d, edit);
    final result = await showDialog<String>(
      barrierDismissible: false,
      context: context,
      builder: (_) => _InkRecognitionDialog(
        api: widget.api,
        request: {
          'lecture_id': widget.ctl.lectureId,
          'question': l10n.studyRecognizePrompt,
          'selection': query,
          'allow_images': true,
          'visuals': [
            {
              'source_id': 'selected-ink',
              'mime_type': 'image/png',
              'data': base64Encode(bytes!),
            },
          ],
        },
      ),
    );
    if (!mounted || result == null || result.trim().isEmpty) return;
    _checkSelectionEdit(d, edit);
    d.checkpoint();
    d.cards.add(
      StudyCard(
        id: studyId(),
        type: 'text',
        layer: d.activeLayer,
        rect: _freeSpot(const Size(480, 320)),
        title: l10n.studyRecognizeInk,
        text: result,
        sources: sources,
      ),
    );
    d.changed();
    await flush();
  }
}

class _InkRecognitionDialog extends StatefulWidget {
  const _InkRecognitionDialog({required this.api, required this.request});
  final Api api;
  final Map<String, dynamic> request;
  @override
  State<_InkRecognitionDialog> createState() => _InkRecognitionDialogState();
}

class _InkRecognitionDialogState extends State<_InkRecognitionDialog> {
  final text = TextEditingController();
  String? run, error;
  bool busy = true, stopped = false, finished = false;
  Timer? timer;
  @override
  void initState() {
    super.initState();
    _start();
  }

  Future<void> _start() async {
    try {
      final out = await widget.api.startCanvasRun(widget.request);
      run = '${out['id']}';
      if (!mounted || stopped) {
        await widget.api.cancelCanvasRun(run!);
        return;
      }
      await _poll();
    } catch (e) {
      if (mounted)
        setState(() {
          error = '$e';
          busy = false;
        });
    }
  }

  Future<void> _poll() async {
    try {
      final out = await widget.api.canvasRun(run!);
      if (!mounted || stopped) return;
      if (out['state'] == 'done') {
        finished = true;
        setState(() {
          text.text = '${out['answer'] ?? ''}';
          busy = false;
        });
      } else if (out['state'] == 'error' || out['state'] == 'cancelled') {
        finished = true;
        setState(() {
          error = '${out['error'] ?? l10n.studyRecognizeFailed}';
          busy = false;
        });
      } else {
        timer = Timer(const Duration(milliseconds: 700), _poll);
      }
    } catch (e) {
      if (mounted)
        setState(() {
          error = '$e';
          busy = false;
        });
    }
  }

  @override
  void dispose() {
    stopped = true;
    timer?.cancel();
    if (!finished && run != null)
      unawaited(widget.api.cancelCanvasRun(run!).catchError((_) => null));
    text.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text(busy ? l10n.studyRecognizeWorking : l10n.studyRecognizeReview),
    content: SizedBox(
      width: 520,
      child: busy
          ? const LinearProgressIndicator()
          : error != null
          ? Text(error!)
          : TextField(
              controller: text,
              minLines: 5,
              maxLines: 12,
              maxLength: 200000,
            ),
    ),
    actions: [
      TextButton(
        onPressed: () => Navigator.pop(context),
        child: Text(l10n.commonCancel),
      ),
      FilledButton(
        onPressed: busy || error != null
            ? null
            : () => Navigator.pop(context, text.text),
        child: Text(l10n.studyRecognizeInsert),
      ),
    ],
  );
}

class _ShapeAnchorDialog extends StatefulWidget {
  const _ShapeAnchorDialog({required this.shape});
  final StudyShape shape;
  @override
  State<_ShapeAnchorDialog> createState() => _ShapeAnchorDialogState();
}

class _ShapeAnchorDialogState extends State<_ShapeAnchorDialog> {
  late List<Offset> points = List.of(widget.shape.points);
  late final Rect bounds = points
      .map((p) => Rect.fromCircle(center: p, radius: 1))
      .reduce((a, b) => a.expandToInclude(b))
      .inflate(35);
  int? drag;
  List<Offset>? baseline;
  bool get ellipse => const {'circle', 'ellipse'}.contains(widget.shape.kind);
  List<Offset> get anchors {
    if (ellipse) {
      final b = points
          .map((p) => Rect.fromPoints(p, p))
          .reduce((a, b) => a.expandToInclude(b));
      return [b.topLeft, b.bottomRight];
    }
    return points.toSet().toList();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text(l10n.studyEditShape),
    content: SizedBox(
      width: 560,
      height: 360,
      child: Column(
        children: [
          Text(l10n.studyShapeDragHint),
          Expanded(
            child: LayoutBuilder(
              builder: (ctx, constraints) {
                final scale = math.min(
                  constraints.maxWidth / bounds.width,
                  constraints.maxHeight / bounds.height,
                );
                final offset =
                    Offset(
                      (constraints.maxWidth - bounds.width * scale) / 2,
                      (constraints.maxHeight - bounds.height * scale) / 2,
                    ) -
                    bounds.topLeft * scale;
                Offset world(Offset p) => (p - offset) / scale;
                return GestureDetector(
                  key: const ValueKey('study-shape-editor'),
                  onPanStart: (e) {
                    final p = world(e.localPosition), list = anchors;
                    final nearby = [
                      for (var i = 0; i < list.length; i++)
                        if ((list[i] - p).distance * scale < 24) i,
                    ];
                    drag = nearby.firstOrNull;
                    baseline = List.of(points);
                  },
                  onPanUpdate: (e) {
                    if (drag == null) return;
                    final old = anchors[drag!];
                    var target = world(e.localPosition);
                    for (final p in anchors) {
                      if (p == old) continue;
                      if ((p.dx - target.dx).abs() * scale < 6)
                        target = Offset(p.dx, target.dy);
                      if ((p.dy - target.dy).abs() * scale < 6)
                        target = Offset(target.dx, p.dy);
                    }
                    if (!ellipse &&
                        anchors.any(
                          (p) => p != old && (p - target).distance * scale < 4,
                        ))
                      return;
                    setState(() {
                      if (ellipse) {
                        final box = Rect.fromPoints(
                          anchors.first,
                          anchors.last,
                        );
                        final next = Rect.fromPoints(
                          anchors[1 - drag!],
                          target,
                        );
                        if (next.width < 4 / scale || next.height < 4 / scale)
                          return;
                        points = [
                          for (final p in points)
                            Offset(
                              next.left +
                                  (p.dx - box.left) * next.width / box.width,
                              next.top +
                                  (p.dy - box.top) * next.height / box.height,
                            ),
                        ];
                      } else {
                        points = [
                          for (final p in points) p == old ? target : p,
                        ];
                      }
                    });
                  },
                  onPanEnd: (_) {
                    drag = null;
                    baseline = null;
                  },
                  onPanCancel: () {
                    if (baseline != null) setState(() => points = baseline!);
                    drag = null;
                    baseline = null;
                  },
                  child: CustomPaint(
                    size: constraints.biggest,
                    painter: _ShapeAnchorPainter(
                      points,
                      anchors,
                      offset,
                      scale,
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    ),
    actions: [
      TextButton(
        onPressed: () => Navigator.pop(context),
        child: Text(l10n.commonCancel),
      ),
      FilledButton(
        onPressed: () => Navigator.pop(context, points),
        child: Text(l10n.commonSave),
      ),
    ],
  );
}

class _ShapeAnchorPainter extends CustomPainter {
  _ShapeAnchorPainter(this.points, this.anchors, this.offset, this.scale);
  final List<Offset> points, anchors;
  final Offset offset;
  final double scale;
  @override
  void paint(Canvas canvas, Size size) {
    final line = Paint()
      ..color = const Color(0xff42639a)
      ..strokeWidth = 2
      ..style = PaintingStyle.stroke;
    final path = Path()
      ..moveTo(
        (points.first * scale + offset).dx,
        (points.first * scale + offset).dy,
      );
    for (final p in points.skip(1)) {
      final v = p * scale + offset;
      path.lineTo(v.dx, v.dy);
    }
    canvas.drawPath(path, line);
    for (final p in anchors) {
      final v = p * scale + offset;
      final guide = Paint()
        ..color = const Color(0x3342639a)
        ..strokeWidth = 1;
      for (var x = 0.0; x < size.width; x += 10) {
        canvas.drawLine(
          Offset(x, v.dy),
          Offset(math.min(size.width, x + 4), v.dy),
          guide,
        );
      }
      for (var y = 0.0; y < size.height; y += 10) {
        canvas.drawLine(
          Offset(v.dx, y),
          Offset(v.dx, math.min(size.height, y + 4)),
          guide,
        );
      }
      canvas.drawCircle(v, 7, Paint()..color = const Color(0xff42639a));
    }
  }

  @override
  bool shouldRepaint(_ShapeAnchorPainter old) => true;
}
