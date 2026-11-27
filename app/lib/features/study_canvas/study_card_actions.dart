part of 'study_canvas.dart';

extension _StudyCardActions on StudyCanvasState {
  Widget _cardSelectionBar(StudyCard card) {
    final d = document!;
    final busy = _selectionBusy || _importing;
    final destination = !busy && d.editable(d.activeLayer);
    final editable = !busy && d.canTransformCard(card);
    final removable =
        !busy && d.canRemoveCard(card);
    final duplicable = !busy && card.duplicable;
    final actions = [
      if (editable &&
          destination &&
          card.type == 'image' &&
          card.path.isNotEmpty)
        _BarAction(
          icon: Icons.crop_rounded,
          tooltip: l10n.studyCropRectangle,
          onPressed: () =>
              _selectionJob(() => _cropCardImage(card, free: false)),
        ),
      if (editable &&
          destination &&
          card.type == 'image' &&
          card.path.isNotEmpty)
        _BarAction(
          icon: Icons.gesture_rounded,
          tooltip: l10n.studyCropFreehand,
          onPressed: () =>
              _selectionJob(() => _cropCardImage(card, free: true)),
        ),
      if (duplicable && destination)
        _BarAction(
          icon: Icons.control_point_duplicate_outlined,
          tooltip: l10n.studyDuplicateCard,
          onPressed: _duplicateSelection,
        ),
      if (!busy && d.editable(card.layer))
        _BarAction(
          icon: card.positionLocked
              ? Icons.lock_open_outlined
              : Icons.lock_outline,
          tooltip: card.positionLocked
              ? l10n.studyUnlockPosition
              : l10n.studyLockPosition,
          onPressed: () {
            d.checkpoint();
            card.positionLocked = !card.positionLocked;
            d.changed();
          },
        ),
      if (removable)
        _BarAction(
          icon: Icons.delete_outline_rounded,
          tooltip: l10n.commonDelete,
          color: chrome.danger,
          onPressed: _deleteSelection,
        ),
      PopupMenuButton<VoidCallback>(
        key: const ValueKey('study-card-selection-more'),
        icon: const Icon(Icons.more_horiz, size: 18),
        tooltip: l10n.studySelectionMore,
        style: _actionBarButtonStyle(context),
        onSelected: (action) => action(),
        itemBuilder: (_) => [
          PopupMenuItem(
            value: _clearSelection,
            child: Text(l10n.studySelectionClear),
          ),
        ],
      ),
    ];
    return KeyedSubtree(
      key: _cardActionsKey,
      child: _floatingActionBar(
        key: const ValueKey('study-card-selection-bar'),
        actions: actions,
      ),
    );
  }

  // Crop original image pixels independently of the card's canvas transform.
  // A new asset preserves the source image and ink that references its frame.
  Future<void> _cropCardImage(StudyCard card, {required bool free}) async {
    final d = document!, edit = d.edit;
    _checkSelectionEdit(d, edit);
    final bytes = await widget.api.studyImageBytes(card.path);
    _checkSelectionEdit(d, edit);
    final buffer = await ui.ImmutableBuffer.fromUint8List(bytes);
    final ui.Image image;
    try {
      final descriptor = await ui.ImageDescriptor.encoded(buffer);
      try {
        final maxSide = math.max(descriptor.width, descriptor.height);
        final scale = math.min(1.0, 4096 / maxSide);
        final codec = await descriptor.instantiateCodec(
          targetWidth: math.max(1, (descriptor.width * scale).round()),
          targetHeight: math.max(1, (descriptor.height * scale).round()),
        );
        try {
          image = (await codec.getNextFrame()).image;
        } finally {
          codec.dispose();
        }
      } finally {
        descriptor.dispose();
      }
    } finally {
      buffer.dispose();
    }
    try {
      _checkSelectionEdit(d, edit);
      final points = await showDialog<List<Offset>>(
        context: context,
        builder: (_) => _CardCropDialog(image: image, free: free),
      );
      if (points == null || !mounted) return;
      _checkSelectionEdit(d, edit);
      final cropped = await _renderCardCrop(image, points, free: free);
      _checkSelectionEdit(d, edit);
      final filename = 'crop-${studyId()}.png';
      final out = widget.standalone
          ? await widget.api.upload(
              '/api/notebooks/${widget.notebookId}/assets',
              {'name': filename},
              Stream.value(cropped),
              cropped.length,
            )
          : await widget.api.importStudyAsset(
              widget.ctl.lectureId,
              filename,
              Stream.value(cropped),
              cropped.length,
            );
      _checkSelectionEdit(d, edit);
      final copy = StudyCard.fromJson((out['objects'] as List).single as Map);
      copy
        ..title = card.title
        ..layer = d.activeLayer
        ..rotation = StudyCardTransform(card).angle
        ..rect = _placeNear(card.visualBounds, copy.rect.size);
      d.checkpoint();
      d.cards.add(copy);
      _selection
        ..clear()
        ..add(copy.id);
      d.changed();
    } finally {
      image.dispose();
    }
  }
}

Path _cropPath(List<Offset> points, Size size, bool free) {
  final scaled = [
    for (final p in points) Offset(p.dx * size.width, p.dy * size.height),
  ];
  return free
      ? (Path()..addPolygon(scaled, true))
      : (Path()..addRect(Rect.fromPoints(scaled.first, scaled.last)));
}

Future<Uint8List> _renderCardCrop(
  ui.Image image,
  List<Offset> region, {
  required bool free,
}) async {
  final size = Size(image.width.toDouble(), image.height.toDouble());
  final path = _cropPath(region, size, free);
  final bounds = path.getBounds();
  final left = bounds.left.floor(), top = bounds.top.floor();
  final width = bounds.right.ceil() - left, height = bounds.bottom.ceil() - top;
  final recorder = ui.PictureRecorder();
  Canvas(recorder)
    ..translate(-left.toDouble(), -top.toDouble())
    ..clipPath(path)
    ..drawImage(image, Offset.zero, Paint());
  final picture = recorder.endRecording();
  try {
    final result = await picture.toImage(width, height);
    try {
      final bytes = await result.toByteData(format: ui.ImageByteFormat.png);
      return bytes!.buffer.asUint8List();
    } finally {
      result.dispose();
    }
  } finally {
    picture.dispose();
  }
}

class _CardCropDialog extends StatefulWidget {
  const _CardCropDialog({required this.image, required this.free});
  final ui.Image image;
  final bool free;

  @override
  State<_CardCropDialog> createState() => _CardCropDialogState();
}

class _CardCropDialogState extends State<_CardCropDialog> {
  List<Offset> _points = [];
  bool _drawing = false;

  Offset _unit(Offset point, Size size) => Offset(
    (point.dx / size.width).clamp(0.0, 1.0),
    (point.dy / size.height).clamp(0.0, 1.0),
  );

  @override
  Widget build(BuildContext context) {
    final img = widget.image, free = widget.free;
    final viewport = MediaQuery.sizeOf(context);
    final size = applyBoxFit(
      BoxFit.contain,
      Size(img.width.toDouble(), img.height.toDouble()),
      Size(
        math.min(640, viewport.width * .7),
        math.min(540, viewport.height * .6),
      ),
    ).destination;
    final path = _points.length < (free ? 3 : 2)
        ? null
        : _cropPath(_points, size, free);
    final bounds = path?.getBounds();
    return AlertDialog(
      title: Text(free ? l10n.studyCropFreehand : l10n.studyCropRectangle),
      content: SizedBox.fromSize(
        size: size,
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          dragStartBehavior: DragStartBehavior.down,
          onPanStart: (event) => setState(() {
            _points = [_unit(event.localPosition, size)];
            _drawing = true;
          }),
          onPanUpdate: (event) => setState(() {
            final p = _unit(event.localPosition, size);
            if (!free) {
              _points = [_points.first, p];
            } else if ((_points.last - p).distance > .002 &&
                _points.length < 2000) {
              _points.add(p);
            }
          }),
          onPanEnd: (_) => setState(() => _drawing = false),
          onPanCancel: () => setState(() {
            _drawing = false;
            _points = [];
          }),
          child: CustomPaint(
            key: const ValueKey('study-card-crop-image'),
            foregroundPainter: path == null ? null : _CardCropOverlay(path),
            child: RawImage(image: img, fit: BoxFit.fill),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: Text(l10n.commonCancel),
        ),
        FilledButton(
          onPressed:
              !_drawing &&
                  bounds != null &&
                  bounds.width >= 6 &&
                  bounds.height >= 6
              ? () => Navigator.pop(context, _points)
              : null,
          child: Text(l10n.studyCropKeepCopy),
        ),
      ],
    );
  }
}

class _CardCropOverlay extends CustomPainter {
  const _CardCropOverlay(this.path);
  final Path path;

  @override
  void paint(Canvas canvas, Size size) {
    final shade = Path()..fillType = PathFillType.evenOdd;
    shade
      ..addRect(Offset.zero & size)
      ..addPath(path, Offset.zero);
    canvas.drawPath(shade, Paint()..color = Colors.black45);
    canvas.drawPath(
      path,
      Paint()
        ..color = Colors.white
        ..style = PaintingStyle.stroke
        ..strokeWidth = 1.5,
    );
  }

  @override
  bool shouldRepaint(_CardCropOverlay old) => old.path != path;
}
