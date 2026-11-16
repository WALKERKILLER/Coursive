import 'dart:math' as math;
import 'dart:ui';

import '../../core/ink_erase.dart';
import 'study_document.dart';

Rect studyOriginalPaper(StudyCard card, double aspect) {
  final height = card.collapsed ? card.expandedH : card.contentRect.height;
  final area = Rect.fromLTWH(
    card.contentRect.left,
    card.contentRect.top + 36,
    card.contentRect.width,
    math.max(1, height - (card.type == 'pdfdoc' ? 72 : 36)),
  );
  final width = math.min(area.width, area.height * aspect);
  return Rect.fromCenter(
    center: area.center,
    width: width,
    height: width / aspect,
  );
}

StudyStroke _reanchor(
  StudyStroke s,
  String anchor,
  int page,
  Rect from,
  Rect to,
) {
  final sx = to.width / from.width, sy = to.height / from.height;
  return StudyStroke(
    id: s.id,
    layer: s.layer,
    anchor: anchor,
    page: page,
    gesture: s.gesture,
    points: [
      for (final p in s.points)
        InkPoint(
          Offset(
            to.left + (p.p.dx - from.left) * sx,
            to.top + (p.p.dy - from.top) * sy,
          ),
          p.pressure,
        ),
    ],
    color: s.color,
    width: s.width * math.sqrt(sx * sy),
    tool: s.tool,
    brush: s.brush,
  );
}

extension StudyPDFColumn on StudyDocument {
  void erasePDFInk(
    StudyCard card,
    int page,
    double aspect,
    List<Offset> trail,
    Size size,
    double radius,
    String mode,
  ) {
    final paper = studyOriginalPaper(card, aspect);
    final eraser = InkEraser(trail, size, radius, mode);
    final next = <StudyStroke>[];
    var touched = false;
    for (final s in strokes) {
      if (s.anchor != card.id ||
          !editable(s.layer) ||
          !(s.page == page || (s.page == 0 && card.page == page))) {
        next.add(s);
        continue;
      }
      final box = s.bounds;
      final bounds =
          [box.topLeft, box.topRight, box.bottomLeft, box.bottomRight]
              .map(card.contentPoint)
              .map((p) => Rect.fromPoints(p, p))
              .reduce((a, b) => a.expandToInclude(b));
      final normalizedBounds = Rect.fromLTRB(
        (bounds.left - paper.left) / paper.width,
        (bounds.top - paper.top) / paper.height,
        (bounds.right - paper.left) / paper.width,
        (bounds.bottom - paper.top) / paper.height,
      );
      final fragments = eraser.clipSamples(
        s.points,
        position: (sample) {
          final p = card.contentPoint(sample.p);
          return Offset(
            (p.dx - paper.left) / paper.width,
            (p.dy - paper.top) / paper.height,
          );
        },
        interpolate: InkPoint.lerp,
        bounds: normalizedBounds,
      );
      if (fragments.length == 1 && identical(fragments.first, s.points)) {
        next.add(s);
        continue;
      }
      touched = true;
      for (final f in fragments)
        next.add(
          StudyStroke(
            id: studyId(),
            layer: s.layer,
            anchor: s.anchor,
            page: s.page,
            color: s.color,
            width: s.width,
            gesture: s.gesture,
            brush: s.brush,
            tool: s.tool,
            points: f,
          ),
        );
    }
    if (touched) {
      checkpoint();
      strokes = next;
      changed();
    }
  }

  List<StudyCard> pdfPages(String root) =>
      cards.where((c) => c.pdfColumn == root && c.type == 'pdf').toList()
        ..sort((a, b) => a.page.compareTo(b.page));

  void beginPDFColumn(StudyCard root) {
    if (root.type != 'pdfdoc' ||
        !cards.contains(root) ||
        !canTransformCard(root)) {
      throw const FormatException('请先解锁文档和批注图层');
    }
    if (cards.length + root.pageCount - pdfPages(root.id).length > 2000) {
      throw const FormatException('展开后超过画布 2000 张卡片上限');
    }
    if (root.pdfColumn == root.id) return;
    checkpoint();
    root.pdfColumn = root.id;
    if (!root.collapsed) root.expandedH = root.rect.height;
    root.collapsed = true;
    root.rect = Rect.fromLTWH(
      root.rect.left,
      root.rect.top,
      root.rect.width,
      60,
    );
    changed();
  }

  void appendPDFPage(StudyCard root, int page, Map data) {
    if (!cards.contains(root) || root.pdfColumn != root.id) return;
    final pages = pdfPages(root.id);
    if (pages.any((c) => c.page == page)) return;
    if (!canTransformCard(root) || pages.any((c) => !canTransformCard(c))) {
      throw const FormatException('请先解锁文档和批注图层');
    }
    final aspect = (data['width'] as num) / (data['height'] as num);
    if (!aspect.isFinite || aspect <= 0 || page < 1 || page > root.pageCount) {
      throw const FormatException('PDF 页面尺寸或页码无效');
    }
    final before = pages.where((c) => c.page < page).lastOrNull;
    final top = (before?.rect.bottom ?? root.rect.bottom) + 24;
    final card = StudyCard(
      id: studyId(),
      type: 'pdf',
      layer: root.layer,
      rect: Rect.fromLTWH(
        root.rect.left,
        top,
        root.rect.width,
        root.rect.width / aspect + 36,
      ),
      title: root.title,
      source: root.source,
      documentId: root.documentId,
      path: '${data['file']}',
      text: '${data['text'] ?? ''}',
      textBoxes: (data['boxes'] as List? ?? []).cast<Map>(),
      page: page,
      pageCount: root.pageCount,
      pageAspect: aspect,
      pdfColumn: root.id,
    );
    var nextTop = card.rect.bottom + 24;
    for (final after in pages.where((c) => c.page > page)) {
      final old = after.rect;
      transformCards(
        {after.id},
        old,
        Rect.fromLTWH(root.rect.left, nextTop, old.width, old.height),
      );
      nextTop = after.rect.bottom + 24;
    }
    final from = studyOriginalPaper(root, aspect),
        to = studyOriginalPaper(card, aspect);
    strokes = [
      for (final s in strokes)
        if (s.anchor == root.id &&
            (s.page == page || (s.page == 0 && root.page == page)))
          _reanchor(s, card.id, page, from, to)
        else
          s,
    ];
    cards.add(card);
    changed();
  }

  void collapsePDFColumn(String id) {
    final root = cards
        .where((c) => c.id == id && c.type == 'pdfdoc')
        .firstOrNull;
    if (root == null) throw const FormatException('主文档已删除，请先撤销删除');
    final pages = pdfPages(id);
    if (!canTransformCard(root) || pages.any((c) => !canTransformCard(c))) {
      throw const FormatException('请先解锁文档和批注图层');
    }
    checkpoint();
    final byId = {for (final p in pages) p.id: p};
    strokes = [
      for (final s in strokes)
        if (byId[s.anchor] case final StudyCard page)
          _reanchor(
            s,
            root.id,
            page.page,
            studyOriginalPaper(page, page.pageAspect),
            studyOriginalPaper(root, page.pageAspect),
          )
        else
          s,
    ];
    connections = [
      for (final c in connections)
        StudyConnection(
          c.id,
          byId.containsKey(c.from) ? root.id : c.from,
          byId.containsKey(c.to) ? root.id : c.to,
          c.label,
        ),
    ]..removeWhere((c) => c.from == c.to);
    cards.removeWhere((c) => byId.containsKey(c.id));
    root.pdfColumn = '';
    root.collapsed = false;
    root.rect = Rect.fromLTWH(
      root.rect.left,
      root.rect.top,
      root.rect.width,
      math.max(240, root.expandedH),
    );
    changed();
  }
}
