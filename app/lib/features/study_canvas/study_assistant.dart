part of 'study_canvas.dart';

extension StudyAssistant on StudyCanvasState {
  Future<void> _refreshLinkedNotes(StudyDocument d, {String? noteId}) async {
    for (final card
        in d.cards
            .where(
              (c) => c.linkedNote && (noteId == null || c.documentId == noteId),
            )
            .toList()) {
      try {
        final n = await widget.api.get(
          '/api/notes/${Uri.encodeComponent(card.documentId)}',
        ) as Map;
        if (!mounted || document != d || !d.cards.contains(card)) return;
        if (n['course_id'] != widget.ctl.course?.id) continue;
        final text = n['body'] as String? ?? '';
        final title = n['title'] as String? ?? card.title;
        if (card.text == text && card.title == title) continue;
        final rebase = card.text.isNotEmpty && card.text != text;
        _update(() {
          if (rebase) _rebaseMarkdownNotes(card, text);
          card.text = text;
          card.title = title;
        });
        if (rebase) d.changed();
      } catch (_) {
        // The card may be visible before its note has arrived from sync.
      }
    }
  }

  void _showAssistantCard(StudyCard card) {
    if (_viewport.isEmpty) return;
    final r = card.visualBounds;
    final zoom = math
        .min(
          (_viewport.width - 72) / r.width,
          (_viewport.height - 130) / r.height,
        )
        .clamp(.05, 1.5)
        .toDouble();
    _flyTo(_viewport.center(Offset.zero) - r.center * zoom, zoom);
  }

  Future<bool> runAssistantCommand(Map<String, dynamic> op) async {
    final d = document;
    if (!mounted ||
        widget.standalone ||
        d == null ||
        _editingCanvas ||
        _remotePending ||
        op['lecture_id'] != widget.ctl.lectureId ||
        op['base_hash'] != d.baseHash)
      return false;
    final c = d.cards.where((c) => c.id == op['card_id']).firstOrNull;
    if (c == null || !d.visible(c.layer)) return false;
    double? coordinate(String key) {
      final v = (op[key] as num?)?.toDouble();
      return v != null && v.isFinite && v.abs() <= 1e8 ? v : null;
    }

    switch (op['action']) {
      case 'move':
      case 'resize':
        if (!d.canTransformCard(c) || c.collapsed) return false;
        final session = StudyTransformSession(
          d,
          {c.id},
          {},
          pivot: c.rect.center,
        );
        if (op['action'] == 'move') {
          final x = coordinate('x'), y = coordinate('y');
          if (x == null || y == null) return false;
          session.preview(translation: Offset(x, y) - c.rect.topLeft);
        } else {
          final w = coordinate('width'), h = coordinate('height');
          if (w == null ||
              h == null ||
              w < 24 ||
              h < 24 ||
              w > 100000 ||
              h > 100000)
            return false;
          final size = c.contentRect.size;
          final angle = StudyCardTransform(c).angle;
          final dw = w - size.width, dh = h - size.height;
          session.previewCorner(
            2,
            Offset(
              dw * math.cos(angle) - dh * math.sin(angle),
              dw * math.sin(angle) + dh * math.cos(angle),
            ),
          );
        }
        session.commit();
        return flush();
      case 'pdf_page':
        final page = op['page'];
        if (c.type != 'pdfdoc' ||
            page is! int ||
            page < 1 ||
            page > c.pageCount ||
            c.collapsed)
          return false;
        if (c.pdfColumn.isNotEmpty) {
          final target = d
              .pdfPages(c.id)
              .where((p) => p.page == page)
              .firstOrNull;
          if (target == null) return false;
          _showAssistantCard(target);
          return true;
        }
        final reader = _pdfBodies[c.id]?.currentState?._reader.currentState;
        if (reader?.writing == true) return false;
        if (reader != null) {
          reader.jumpToPage(page);
        } else if (_inlinePDF && d.editable(c.layer)) {
          d.checkpoint();
          c.page = page;
          d.changed();
        } else {
          return false;
        }
        _showAssistantCard(c);
        return flush();
      case 'pdf_view':
        final view = op['view'];
        if (c.type != 'pdfdoc' ||
            !d.editable(c.layer) ||
            view is! String ||
            !const {'scroll', 'paged', 'column'}.contains(view))
          return false;
        await _setPDFView(c, view);
        return mounted &&
            (view == 'column'
                ? c.pdfColumn.isNotEmpty
                : c.pdfColumn.isEmpty && c.pdfPaged == (view == 'paged'));
    }
    return false;
  }
}
