import 'package:flutter/gestures.dart';
import 'package:flutter/widgets.dart';

const canvasCardHoldDelay = Duration(milliseconds: 600);

/// Marks picture contacts for the canvas's shared pan, pinch and hold router.
/// Playback controls stay outside this surface and retain their own gestures.
class VideoCardHoldScope extends InheritedWidget {
  const VideoCardHoldScope({
    super.key,
    required this.onSurfaceDown,
    required this.onSurfaceEnd,
    this.onControlDown,
    this.writing = false,
    required super.child,
  });
  final bool writing;
  final ValueChanged<PointerDownEvent> onSurfaceDown;
  final ValueChanged<PointerDownEvent>? onControlDown;
  final ValueChanged<PointerEvent> onSurfaceEnd;

  static VideoCardHoldScope? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<VideoCardHoldScope>();

  @override
  bool updateShouldNotify(VideoCardHoldScope oldWidget) =>
      writing != oldWidget.writing || onControlDown != oldWidget.onControlDown || onSurfaceDown != oldWidget.onSurfaceDown ||
      onSurfaceEnd != oldWidget.onSurfaceEnd;
}

class VideoCardControls extends StatelessWidget {
  const VideoCardControls({super.key, required this.child});
  final Widget child;
  @override
  Widget build(BuildContext context) {
    final scope = VideoCardHoldScope.maybeOf(context);
    if (scope == null) return child;
    return Listener(onPointerDown: scope.onControlDown, onPointerUp: scope.onSurfaceEnd,
        onPointerCancel: scope.onSurfaceEnd, child: child);
  }
}

/// Wrap the picture below controls, or reserve a known bottom control lane.
class VideoCardHoldRegion extends StatelessWidget {
  const VideoCardHoldRegion({
    super.key,
    required this.child,
    this.bottomControlsHeight = 0,
  });
  final Widget child;
  final double bottomControlsHeight;

  @override
  Widget build(BuildContext context) {
    final scope = VideoCardHoldScope.maybeOf(context);
    if (scope == null) return child;
    bool picture(PointerEvent e) {
      final box = context.findRenderObject() as RenderBox?;
      return box != null &&
          box.globalToLocal(e.position).dy <
              box.size.height - bottomControlsHeight;
    }

    return Listener(
      behavior: HitTestBehavior.opaque,
      onPointerDown: (e) {
        if (picture(e)) {
          scope.onSurfaceDown(e);
        } else {
          scope.onControlDown?.call(e);
        }
      },
      onPointerUp: scope.onSurfaceEnd,
      onPointerCancel: scope.onSurfaceEnd,
      // Cancel picture taps on navigation/holds. The canvas Listener owns the
      // actual movement, including mixed video/PDF/blank two-finger gestures.
      child: RawGestureDetector(
        behavior: HitTestBehavior.opaque,
        gestures: {
          _PicturePan: GestureRecognizerFactoryWithHandlers<_PicturePan>(
            () => _PicturePan(picture),
            (r) => r
              ..allows = picture
              ..onStart = (_) {}
              ..onUpdate = (_) {},
          ),
          _PictureHold: GestureRecognizerFactoryWithHandlers<_PictureHold>(
            () => _PictureHold(picture),
            (r) => r
              ..allows = picture
              ..onLongPressStart = (_) {},
          ),
        },
        child: child,
      ),
    );
  }
}

class _PicturePan extends PanGestureRecognizer {
  _PicturePan(this.allows)
    : super(
        supportedDevices: {
          PointerDeviceKind.touch,
          PointerDeviceKind.mouse,
          PointerDeviceKind.stylus,
        },
      );
  bool Function(PointerEvent) allows;
  @override
  bool isPointerAllowed(PointerEvent e) =>
      allows(e) && super.isPointerAllowed(e);
}

class _PictureHold extends LongPressGestureRecognizer {
  _PictureHold(this.allows)
    : super(
        duration: canvasCardHoldDelay,
        supportedDevices: {
          PointerDeviceKind.touch,
          PointerDeviceKind.mouse,
          PointerDeviceKind.stylus,
        },
      );
  bool Function(PointerEvent) allows;
  @override
  bool isPointerAllowed(PointerDownEvent e) =>
      allows(e) && super.isPointerAllowed(e);
}
