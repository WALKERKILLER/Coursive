// The light that marks what the assistant is reading: a band of colour that
// travels around the edge of the material, after the edge light Apple
// Intelligence uses, and a shimmer through the words that name it.
import 'dart:math' as math;

import 'package:flutter/material.dart';

/// Hues of the reading light, cool to warm and back to cool.
const readingHues = [
  Color(0xff5f8bff),
  Color(0xffa66cff),
  Color(0xffff6fae),
  Color(0xffff9757),
  Color(0xffffcc66),
  Color(0xff45d0c0),
];

/// A sweep of [readingHues] around [rect] turned by [t]. The hues go round
/// twice, so `t` from 0 to 1 turns half a circle and loops without a seam.
Shader readingShader(Rect rect, double t, {double opacity = 1}) =>
    SweepGradient(
      colors: [
        for (final c in [...readingHues, ...readingHues, readingHues.first])
          c.withValues(alpha: opacity),
      ],
      transform: GradientRotation(t * math.pi),
    ).createShader(rect);

/// Paints the reading light along [r]: a wide soft halo, a glow and a
/// crisp edge, all turning with [t].
void paintReadingLight(
  Canvas canvas,
  RRect r,
  double t, {
  double halo = 14,
  double strength = 1,
}) {
  void stroke(double width, double blur, double opacity) => canvas.drawRRect(
    r,
    Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = width
      ..shader = readingShader(r.outerRect, t, opacity: opacity * strength)
      ..maskFilter = blur > 0 ? MaskFilter.blur(BlurStyle.normal, blur) : null,
  );
  stroke(halo, halo * 1.2, .38);
  stroke(halo * .4, halo * .4, .7);
  stroke(1.6, 0, 1);
}

/// Repeats [period] while [active] and motion is allowed; a still frame
/// otherwise.
mixin _ReadingClock<T extends StatefulWidget>
    on State<T>, SingleTickerProviderStateMixin<T> {
  Duration get period;
  bool get active;
  late final clock = AnimationController(vsync: this, duration: period);

  void syncClock() {
    final still = MediaQuery.maybeDisableAnimationsOf(context) ?? false;
    if (active && !still) {
      if (!clock.isAnimating) clock.repeat();
    } else {
      clock.stop();
      clock.value = 0;
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    syncClock();
  }

  @override
  void didUpdateWidget(T old) {
    super.didUpdateWidget(old);
    syncClock();
  }

  @override
  void dispose() {
    clock.dispose();
    super.dispose();
  }
}

/// Wraps [child] in the reading light while [active].
class ReadingGlow extends StatefulWidget {
  const ReadingGlow({
    super.key,
    required this.active,
    required this.child,
    this.radius = 999,
    this.halo = 6,
  });

  final bool active;
  final Widget child;
  final double radius, halo;

  @override
  State<ReadingGlow> createState() => _ReadingGlowState();
}

class _ReadingGlowState extends State<ReadingGlow>
    with SingleTickerProviderStateMixin, _ReadingClock {
  @override
  Duration get period => const Duration(milliseconds: 1800);
  @override
  bool get active => widget.active;

  @override
  Widget build(BuildContext context) => CustomPaint(
    foregroundPainter: widget.active
        ? _GlowPainter(clock, widget.radius, widget.halo)
        : null,
    child: widget.child,
  );
}

class _GlowPainter extends CustomPainter {
  _GlowPainter(this.t, this.radius, this.halo) : super(repaint: t);
  final Animation<double> t;
  final double radius, halo;

  @override
  void paint(Canvas canvas, Size size) {
    final rect = Offset.zero & size;
    paintReadingLight(
      canvas,
      RRect.fromRectAndRadius(
        rect,
        Radius.circular(math.min(radius, size.shortestSide / 2)),
      ),
      t.value,
      halo: halo,
    );
  }

  @override
  bool shouldRepaint(_GlowPainter o) => o.radius != radius || o.halo != halo;
}

/// A light that passes through [child]'s glyphs while [active]; [color] is
/// the text's own colour between passes.
class ReadingShimmer extends StatefulWidget {
  const ReadingShimmer({
    super.key,
    required this.active,
    required this.color,
    required this.child,
  });

  final bool active;
  final Color color;
  final Widget child;

  @override
  State<ReadingShimmer> createState() => _ReadingShimmerState();
}

class _ReadingShimmerState extends State<ReadingShimmer>
    with SingleTickerProviderStateMixin, _ReadingClock {
  @override
  Duration get period => const Duration(milliseconds: 2200);
  @override
  bool get active => widget.active;

  @override
  Widget build(BuildContext context) {
    if (!widget.active) return widget.child;
    return AnimatedBuilder(
      animation: clock,
      builder: (context, child) => ShaderMask(
        blendMode: BlendMode.srcIn,
        shaderCallback: (b) => LinearGradient(
          colors: [
            widget.color,
            widget.color,
            readingHues[0],
            readingHues[1],
            readingHues[2],
            widget.color,
            widget.color,
          ],
          stops: const [0, .3, .42, .5, .58, .7, 1],
          transform: _Slide(clock.value),
        ).createShader(b),
        child: child,
      ),
      child: widget.child,
    );
  }
}

/// Moves a gradient from one side past the other as [t] goes 0 → 1.
class _Slide extends GradientTransform {
  const _Slide(this.t);
  final double t;

  @override
  Matrix4 transform(Rect bounds, {TextDirection? textDirection}) =>
      Matrix4.translationValues(bounds.width * (t * 2 - 1), 0, 0);
}

/// A small turning orb: the assistant is at work.
class ReadingOrb extends StatefulWidget {
  const ReadingOrb({super.key, this.size = 10});
  final double size;

  @override
  State<ReadingOrb> createState() => _ReadingOrbState();
}

class _ReadingOrbState extends State<ReadingOrb>
    with SingleTickerProviderStateMixin, _ReadingClock {
  @override
  Duration get period => const Duration(milliseconds: 1600);
  @override
  bool get active => true;

  @override
  Widget build(BuildContext context) =>
      CustomPaint(size: Size.square(widget.size), painter: _OrbPainter(clock));
}

class _OrbPainter extends CustomPainter {
  _OrbPainter(this.t) : super(repaint: t);
  final Animation<double> t;

  @override
  void paint(Canvas canvas, Size size) {
    final rect = Offset.zero & size;
    final c = rect.center, r = size.shortestSide / 2;
    canvas.drawCircle(
      c,
      r,
      Paint()
        ..shader = readingShader(rect, t.value, opacity: .55)
        ..maskFilter = MaskFilter.blur(BlurStyle.normal, r * .8),
    );
    canvas.drawCircle(c, r, Paint()..shader = readingShader(rect, t.value));
  }

  @override
  bool shouldRepaint(_OrbPainter o) => false;
}
