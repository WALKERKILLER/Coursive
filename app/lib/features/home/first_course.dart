import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../l10n/l10n.dart';
import '../../theme/tokens.dart';
import '../../widgets/organic_shapes.dart';
import '../courses/local_course_dialog.dart';
import '../plugins/plugins_page.dart' show openPlugins;

/// 首页 without any course: one calm panel that starts the first one, either
/// from videos on this device or from a course platform plugin.
class FirstCoursePanel extends StatefulWidget {
  const FirstCoursePanel({super.key});

  @override
  State<FirstCoursePanel> createState() => _FirstCoursePanelState();
}

class _FirstCoursePanelState extends State<FirstCoursePanel>
    with SingleTickerProviderStateMixin {
  late final _enter = AnimationController(
    vsync: this,
    duration: Motion.long4 * 1.5,
  );
  bool _hover = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_enter.value == 0 && !_enter.isAnimating) {
      if (MediaQuery.disableAnimationsOf(context)) {
        _enter.value = 1;
      } else {
        _enter.forward();
      }
    }
  }

  @override
  void dispose() {
    _enter.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    return LayoutBuilder(
      builder: (context, box) {
        final wide = box.maxWidth >= 640;
        final art = _Constellation(
          enter: _enter,
          lifted: _hover,
          size: wide ? 232 : 176,
        );
        final text = Column(
          crossAxisAlignment: wide
              ? CrossAxisAlignment.start
              : CrossAxisAlignment.center,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              l10n.homeFirstCourseTitle,
              style: (wide ? tt.headlineMedium : tt.headlineSmall)?.copyWith(
                fontWeight: FontWeight.w500,
              ),
              textAlign: wide ? TextAlign.start : TextAlign.center,
            ),
            const SizedBox(height: Space.md),
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 440),
              child: Text(
                l10n.homeFirstCourseBody,
                style: tt.bodyLarge?.copyWith(
                  color: cs.onSurfaceVariant,
                  height: 1.6,
                ),
                textAlign: wide ? TextAlign.start : TextAlign.center,
              ),
            ),
            const SizedBox(height: Space.xl),
            Wrap(
              alignment: wide ? WrapAlignment.start : WrapAlignment.center,
              spacing: Space.md,
              runSpacing: Space.md,
              children: [
                FilledButton.icon(
                  onPressed: () => startLocalCourse(context),
                  style: FilledButton.styleFrom(minimumSize: const Size(0, 48)),
                  icon: const Icon(Icons.add_rounded),
                  label: Text(l10n.localCourseCreate),
                ),
                OutlinedButton.icon(
                  onPressed: () => openPlugins(context),
                  style: OutlinedButton.styleFrom(
                    minimumSize: const Size(0, 48),
                  ),
                  icon: const Icon(Icons.school_outlined),
                  label: Text(l10n.homeFirstCourseConnect),
                ),
              ],
            ),
            const SizedBox(height: Space.lg),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.lock_outline_rounded,
                  size: 16,
                  color: cs.onSurfaceVariant,
                ),
                const SizedBox(width: Space.xs),
                Flexible(
                  child: Text(
                    l10n.homeFirstCourseLocal,
                    style: tt.labelMedium?.copyWith(color: cs.onSurfaceVariant),
                  ),
                ),
              ],
            ),
          ],
        );
        return MouseRegion(
          onEnter: (_) => setState(() => _hover = true),
          onExit: (_) => setState(() => _hover = false),
          child: Container(
            clipBehavior: Clip.antiAlias,
            decoration: BoxDecoration(
              borderRadius: Corners.all(Corners.xl),
              // A faint wash of the primary tone behind the shapes.
              gradient: RadialGradient(
                center: wide
                    ? const Alignment(0.72, -0.1)
                    : const Alignment(0, -0.55),
                radius: wide ? 0.9 : 1.1,
                colors: [
                  cs.primaryContainer.withValues(alpha: 0.55),
                  cs.surfaceContainerLow,
                ],
              ),
            ),
            padding: EdgeInsets.all(wide ? Space.xxl + Space.sm : Space.xl),
            child: wide
                ? Row(
                    children: [
                      Expanded(child: text),
                      const SizedBox(width: Space.xxl),
                      ExcludeSemantics(child: art),
                    ],
                  )
                : Column(
                    children: [
                      ExcludeSemantics(child: art),
                      const SizedBox(height: Space.xl),
                      text,
                    ],
                  ),
          ),
        );
      },
    );
  }
}

/// Three organic tiles for what a course holds: the recording, its
/// transcript and the notes. They arrive one after another and lean in on
/// hover.
class _Constellation extends StatelessWidget {
  const _Constellation({
    required this.enter,
    required this.lifted,
    required this.size,
  });
  final Animation<double> enter;
  final bool lifted;
  final double size;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final s = size;
    final tiles = [
      // (shape, fill, ink, icon, centre as a fraction of the box, extent, turn)
      (
        OrganicShape.cookie9,
        cs.primaryContainer,
        cs.onPrimaryContainer,
        Icons.play_arrow_rounded,
        const Offset(0.4, 0.52),
        0.6,
        -0.04,
      ),
      (
        OrganicShape.sunny,
        cs.tertiaryContainer,
        cs.onTertiaryContainer,
        Icons.subtitles_outlined,
        const Offset(0.8, 0.24),
        0.36,
        0.05,
      ),
      (
        OrganicShape.pebble,
        cs.secondaryContainer,
        cs.onSecondaryContainer,
        Icons.edit_note_rounded,
        const Offset(0.78, 0.8),
        0.32,
        -0.06,
      ),
    ];
    final dots = [
      (const Offset(0.1, 0.16), 0.06, cs.primary),
      (const Offset(0.12, 0.88), 0.04, cs.tertiary),
      (const Offset(0.97, 0.53), 0.035, cs.secondary),
    ];
    Widget arrive(int i, Widget child) {
      final a = CurvedAnimation(
        parent: enter,
        curve: Interval(
          i * 0.14,
          math.min(1, 0.55 + i * 0.14),
          curve: Motion.emphasizedDecelerate,
        ),
      );
      return FadeTransition(
        opacity: a,
        child: ScaleTransition(
          scale: Tween(begin: 0.6, end: 1.0).animate(a),
          child: child,
        ),
      );
    }

    return SizedBox.square(
      dimension: s,
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          for (final (i, (at, r, color)) in dots.indexed)
            Positioned(
              left: at.dx * s - r * s / 2,
              top: at.dy * s - r * s / 2,
              child: arrive(
                i + 1,
                Container(
                  width: r * s,
                  height: r * s,
                  decoration: BoxDecoration(
                    color: color.withValues(alpha: 0.5),
                    shape: BoxShape.circle,
                  ),
                ),
              ),
            ),
          for (final (i, (shape, fill, ink, icon, at, extent, turn))
              in tiles.indexed)
            Positioned(
              left: (at.dx - extent / 2) * s,
              top: (at.dy - extent / 2) * s,
              child: arrive(
                i,
                AnimatedRotation(
                  turns: lifted ? turn : 0,
                  duration: Motion.long2,
                  curve: Motion.emphasized,
                  child: CustomPaint(
                    painter: _TilePainter(shape, fill),
                    child: SizedBox.square(
                      dimension: extent * s,
                      child: Icon(icon, size: extent * s * 0.42, color: ink),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _TilePainter extends CustomPainter {
  _TilePainter(this.shape, this.color);
  final OrganicShape shape;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) => canvas.drawPath(
    shapePath(shapeRadii(shape), Offset.zero & size),
    Paint()..color = color,
  );

  @override
  bool shouldRepaint(_TilePainter o) => o.shape != shape || o.color != color;
}
