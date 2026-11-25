import 'dart:math' as math;
import 'dart:ui';

import 'package:coursive/core/ink_curve.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'sparse circular samples trace the arc, keep tips and preserve corners',
    () {
      const radius = 110.0;
      final circle = [
        for (var i = 0; i <= 12; i++)
          Offset(
            math.cos(i * math.pi / 6) * radius,
            math.sin(i * math.pi / 6) * radius,
          ),
      ];
      final curve = InkCurve();
      var interior = 0;
      for (var i = 1; i < circle.length; i++) {
        final samples = <Offset>[];
        curve.append(circle[i - 1], circle[i], 2, (p, t) => samples.add(p));
        expect(samples.last, circle[i]);
        if (i == 1) continue;
        for (final p in samples.take(samples.length - 1)) {
          expect((p.distance - radius).abs(), lessThan(1.1));
          interior++;
        }
      }
      expect(interior, greaterThan(20));
      curve.reset();
      final corners = <Offset>[];
      curve.append(
        Offset.zero,
        const Offset(30, 0),
        1,
        (p, _) => corners.add(p),
      );
      curve.append(
        const Offset(30, 0),
        const Offset(30, 30),
        1,
        (p, _) => corners.add(p),
      );
      expect(corners, [const Offset(30, 0), const Offset(30, 30)]);
    },
  );
}
