part of 'study_canvas.dart';

extension _StudyTransforms on StudyCanvasState {
  PointerEvent _surfaceEvent(PointerEvent e) {
    final box = _inputSurface.currentContext!.findRenderObject() as RenderBox;
    return e.transformed(Matrix4.inverted(box.getTransformTo(null)));
  }

  Offset _surfacePoint(Offset global) =>
      (_inputSurface.currentContext!.findRenderObject() as RenderBox)
          .globalToLocal(global);

  void _finishTransform({bool cancel = false}) {
    final session = _transformSession;
    _transformSession = null;
    _transformPointer = null;
    _transformCorner = null;
    _rotatePivot = null;
    _dragCard = null;
    if (session == null) return;
    if (cancel)
      session.cancel();
    else
      session.commit();
    _selectionBounds = _boundsOfSelection();
    if (_selectionReady) _regionSelection = _selectionBounds;
    _scene.value++;
    _cursor.value++;
    if (mounted) _update(() => _rotationLabel = null);
    if (!cancel) unawaited(flush());
  }

  void _beginGrip(PointerDownEvent e, {int? corner}) {
    if (_transformSession != null || _drawingPointer != null) return;
    final d = document!, bounds = _boundsOfSelection();
    if (bounds == null || !d.canRotateSelection(_selection, _detail.strokeIds))
      return;
    final single = _selection.length == 1 && _detail.strokeIds.isEmpty
        ? d.cards.where((c) => _selection.contains(c.id)).firstOrNull
        : null;
    final corners = single == null
        ? [
            bounds.topLeft,
            bounds.topRight,
            bounds.bottomRight,
            bounds.bottomLeft,
          ]
        : StudyCardTransform(single).corners;
    final local = _surfacePoint(e.position), world = _world(local);
    final pivot = corner == null ? bounds.center : corners[(corner + 2) % 4];
    _transformSession = StudyTransformSession(
      d,
      Set.of(_selection),
      Set.of(_detail.strokeIds),
      pivot: pivot,
    );
    _transformPointer = e.pointer;
    _transformCorner = corner;
    _transformStart = world;
    _transformVector = corner == null ? world - pivot : corners[corner] - pivot;
    _rotateLast = (world - pivot).direction;
    _rotationRaw = 0;
    _rotationBase = single == null ? 0 : StudyCardTransform(single).angle;
    _angleSnap = StudyAngleSnap();
    _rotatePivot = corner == null ? pivot : null;
    _pointers[e.pointer] = local;
    if (e.kind == PointerDeviceKind.touch) _touchPointers.add(e.pointer);
    _gesturing.value = true;
    _saveTimer?.cancel();
    _update(() {});
  }

  void _moveGrip(PointerMoveEvent e) {
    if (_transformPointer != e.pointer || _transformSession == null) {
      _move(_surfaceEvent(e) as PointerMoveEvent);
      return;
    }
    final d = document!, session = _transformSession!;
    final p = _world(_surfacePoint(e.position));
    _pointers[e.pointer] = _surfacePoint(e.position);
    if (_transformCorner != null && session.resizesReadingFrame) {
      session.previewCorner(_transformCorner!, p - _transformStart);
    } else if (_transformCorner != null) {
      final v = p - session.pivot;
      final scale =
          (v.dx * _transformVector.dx + v.dy * _transformVector.dy) /
          math.max(1e-9, _transformVector.distanceSquared);
      session.preview(scale: scale);
    } else {
      final v = p - session.pivot;
      if (v.distance * d.zoom < 4) return;
      final angle = v.direction;
      _rotationRaw += studyAngle(angle - _rotateLast);
      _rotateLast = angle;
      final before = _angleSnap.locked;
      final snapped = _angleSnap.update(
        _rotationBase + _rotationRaw,
        enabled: _angleSnapping && !HardwareKeyboard.instance.isAltPressed,
      );
      session.preview(angle: snapped - _rotationBase);
      if (_angleSnap.locked != null &&
          before != _angleSnap.locked &&
          e.kind != PointerDeviceKind.mouse)
        unawaited(HapticFeedback.selectionClick());
      final degrees = studyAngle(snapped) * 180 / math.pi;
      _rotationLabel =
          '${(degrees.abs() == 180 ? 180 : degrees).toStringAsFixed(1)}°';
    }
    _scene.value++;
    _cursor.value++;
    _update(() {});
  }

  void _endGrip(PointerEvent e) {
    if (_transformPointer == e.pointer)
      _finishTransform(cancel: e is PointerCancelEvent);
    _up(_surfaceEvent(e));
  }

  Widget _transformFrame() {
    final d = document!;
    if (_drawingPointer != null || _quickAsk || _detail.objectId != null)
      return const SizedBox.shrink();
    final bounds = _boundsOfSelection();
    if (bounds == null) return const SizedBox.shrink();
    final single = _selection.length == 1 && _detail.strokeIds.isEmpty
        ? d.cards.where((c) => _selection.contains(c.id)).firstOrNull
        : null;
    final showGrips = d.canRotateSelection(_selection, _detail.strokeIds);
    if (!showGrips && (single?.positionLocked != true || _selectionReady)) {
      return const SizedBox.shrink();
    }
    final corners = single == null
        ? [
            bounds.topLeft,
            bounds.topRight,
            bounds.bottomRight,
            bounds.bottomLeft,
          ]
        : StudyCardTransform(single).corners;
    // Keep the frame in screen space so its grips stay the same size.
    // Project it from the same camera update that moves the cards.
    return Positioned.fill(
      child: AnimatedBuilder(
        animation: _camera,
        builder: (_, _) => _screenTransformFrame(
          d,
          bounds,
          corners,
          showGrips: showGrips,
          showCorners: single?.collapsed != true,
          card: !_selectionReady && _tool == StudyTool.select ? single : null,
        ),
      ),
    );
  }

  Widget _screenTransformFrame(
    StudyDocument d,
    Rect bounds,
    List<Offset> corners, {
    required bool showGrips,
    required bool showCorners,
    required StudyCard? card,
  }) {
    final screenBounds = Rect.fromPoints(
      bounds.topLeft * d.zoom + d.camera,
      bounds.bottomRight * d.zoom + d.camera,
    );
    if (!screenBounds.overlaps(Offset.zero & _viewport)) {
      return const SizedBox.shrink();
    }
    final points = corners.map((p) => p * d.zoom + d.camera).toList();
    final top = (points[0] + points[1]) / 2;
    final centre = bounds.center * d.zoom + d.camera;
    final direction = top - centre;
    final grip =
        top +
        (direction.distance < 1
            ? const Offset(0, -32)
            : direction / direction.distance * 32);
    Widget handle(Offset at, {int? corner}) => Positioned(
      left: at.dx - 22,
      top: at.dy - 22,
      width: 44,
      height: 44,
      child: Listener(
        key: ValueKey(
          corner == null
              ? 'study-rotation-handle'
              : 'study-resize-handle-$corner',
        ),
        behavior: HitTestBehavior.opaque,
        onPointerDown: (e) => _beginGrip(e, corner: corner),
        onPointerMove: _moveGrip,
        onPointerUp: _endGrip,
        onPointerCancel: _endGrip,
        child: MouseRegion(
          cursor: corner == null
              ? SystemMouseCursors.grab
              : SystemMouseCursors.resizeUpLeftDownRight,
          child: Center(
            child: Container(
              width: corner == null ? 28 : 12,
              height: corner == null ? 28 : 12,
              decoration: BoxDecoration(
                color: chrome.paper,
                border: Border.all(color: chrome.accent, width: 2),
                borderRadius: BorderRadius.circular(corner == null ? 20 : 3),
              ),
              child: corner == null
                  ? Icon(Icons.rotate_right, size: 18, color: chrome.accent)
                  : null,
            ),
          ),
        ),
      ),
    );
    return Stack(
      clipBehavior: Clip.none,
      children: [
        Positioned.fill(
          child: IgnorePointer(
            child: CustomPaint(
              painter: _TransformOutline(
                points,
                showGrips ? grip : null,
                chrome.accent,
                snapped: _angleSnap.locked != null && _rotatePivot != null,
              ),
            ),
          ),
        ),
        if (showGrips && showCorners)
          for (var i = 0; i < 4; i++) handle(points[i], corner: i),
        if (showGrips) handle(grip),
        if (_rotationLabel == null &&
            _dragCard == null &&
            _transformSession == null &&
            card != null)
          Positioned.fill(
            child: CustomSingleChildLayout(
              delegate: _FloatingLayout(
                screenBounds,
                _topBarsBottom,
                [
                  ..._selectionObstacles,
                  if (showGrips) Rect.fromCircle(center: grip, radius: 22),
                ],
              ),
              child: _cardSelectionBar(card),
            ),
          ),
        if (_rotationLabel != null)
          Positioned(
            left: grip.dx + 24,
            top: grip.dy - 16,
            child: IgnorePointer(
              child: Material(
                borderRadius: BorderRadius.circular(8),
                color: chrome.accent,
                child: Padding(
                  padding: const EdgeInsets.all(6),
                  child: Text(
                    _rotationLabel!,
                    style: TextStyle(color: chrome.paper),
                  ),
                ),
              ),
            ),
          ),
      ],
    );
  }
}

class _TransformOutline extends CustomPainter {
  _TransformOutline(this.points, this.grip, this.color, {this.snapped = false});
  final List<Offset> points;
  final Offset? grip;
  final Color color;
  final bool snapped;
  @override
  void paint(Canvas canvas, Size size) {
    final pen = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5;
    canvas.drawPath(Path()..addPolygon(points, true), pen);
    final handle = grip;
    if (handle != null)
      canvas.drawLine((points[0] + points[1]) / 2, handle, pen);
    if (snapped && handle != null) {
      final centre = (points[0] + points[2]) / 2;
      canvas.drawLine(
        centre - (handle - centre),
        handle,
        pen..strokeWidth = .7,
      );
    }
  }

  @override
  bool shouldRepaint(_TransformOutline old) => true;
}
