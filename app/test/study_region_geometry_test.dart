import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/study_selection_geometry.dart';

void main() {
  test(
    'every edge changes only its axis and keeps the opposite edge fixed',
    () {
      const r = Rect.fromLTRB(100, 100, 400, 300);
      const delta = Offset(40, 30);
      expect(
        studyResizeSelection(r, StudySelectionHandle.left, delta, 24),
        const Rect.fromLTRB(140, 100, 400, 300),
      );
      expect(
        studyResizeSelection(r, StudySelectionHandle.right, delta, 24),
        const Rect.fromLTRB(100, 100, 440, 300),
      );
      expect(
        studyResizeSelection(r, StudySelectionHandle.top, delta, 24),
        const Rect.fromLTRB(100, 130, 400, 300),
      );
      expect(
        studyResizeSelection(r, StudySelectionHandle.bottom, delta, 24),
        const Rect.fromLTRB(100, 100, 400, 330),
      );
      expect(
        studyResizeSelection(
          r,
          StudySelectionHandle.topLeft,
          const Offset(500, 500),
          24,
        ),
        const Rect.fromLTRB(376, 276, 400, 300),
      );
      expect(
        studySelectionHandleAt(r, const Offset(398, 170), 12),
        StudySelectionHandle.right,
      );
      expect(
        studySelectionHandleAt(r, const Offset(101, 102), 12),
        StudySelectionHandle.topLeft,
      );
      expect(studySelectionHandleAt(r, r.center, 12), isNull);
      expect(
        studySelectionHandleAt(
          const Rect.fromLTWH(0, 0, 24, 24),
          const Offset(12, 0),
          16,
        ),
        StudySelectionHandle.top,
      );
    },
  );
  test(
    'toolbar never overlaps the crop, fixed controls or sidebar at any edge',
    () {
      const viewport = Rect.fromLTRB(12, 80, 1188, 788);
      const sidebar = Rect.fromLTRB(800, 80, 1200, 800);
      const dock = Rect.fromLTRB(300, 720, 700, 800);
      for (final selection in [
        const Rect.fromLTRB(50, 80, 700, 300),
        const Rect.fromLTRB(20, 500, 700, 780),
        const Rect.fromLTRB(12, 150, 700, 700),
        const Rect.fromLTRB(200, 200, 500, 400),
      ]) {
        final menu = studySelectionMenuRect(selection, viewport, [
          sidebar,
          dock,
        ]);
        expect(menu, isNotNull);
        expect(menu!.overlaps(selection.inflate(10)), isFalse);
        expect(menu.overlaps(sidebar), isFalse);
        expect(menu.overlaps(dock), isFalse);
        expect(viewport.contains(menu.topLeft), isTrue);
        expect(
          menu.right <= viewport.right && menu.bottom <= viewport.bottom,
          isTrue,
        );
      }
      expect(studySelectionMenuRect(viewport, viewport, []), isNull);
    },
  );
}
