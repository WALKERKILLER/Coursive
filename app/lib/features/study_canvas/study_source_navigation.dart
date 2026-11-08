part of 'study_canvas.dart';

// Following bound material and passages read through 「全课程」. Both land
// as a citation does, with one 「返回原处」 (specs 0027 and 0029).

extension _StudySourceNavigation on StudyCanvasState {
  /// A bound material's pill or a card's source, shown like a citation.
  Future<void> _focusSource(StudyAnchor anchor) => _followEvidence({
    'id': anchor.id,
    'kind': anchor.end > anchor.start && anchor.quote.isNotEmpty
        ? 'exact_text'
        : 'source',
    'status': 'verified',
    'anchor': anchor.json(),
  });

  /// A course passage is re-read by the daemon first; a second click while
  /// that runs joins it instead of importing the material twice.
  Future<void> _followCourseEvidence(Map evidence) {
    final source = Map<String, dynamic>.from(evidence['course_source'] as Map);
    final key = '${source['id']}:${source['offset']}:${source['end']}';
    return _sourceNavigations.putIfAbsent(
      key,
      () => _resolveCourseEvidence(
        evidence,
        source,
      ).whenComplete(() => _sourceNavigations.remove(key)),
    );
  }

  /// Finds the card holding a cited course passage, importing the material
  /// once, and follows the passage there. A changed file is reported, not
  /// guessed at.
  Future<void> _resolveCourseEvidence(
    Map evidence,
    Map<String, dynamic> cited,
  ) async {
    final d = document!;
    try {
      final source = Map<String, dynamic>.from(
        await widget.api.resolveStudySource(widget.ctl.lectureId, cited) as Map,
      );
      if (!mounted || document != d) return;
      final text = '${source['text'] ?? ''}';
      final path = '${source['video_path'] ?? source['path']}';
      final lecture = '${source['lecture_id']}';
      final page = (source['page'] as num? ?? 0).toInt();
      final exactPDF = source['kind'] == 'pdf' && page > 0;
      final video = source['video_path'] != null;
      final excerpt = !video && !exactPDF;
      final identity = excerpt
          ? 'excerpt:${source['id']}:${source['offset']}:${source['fingerprint']}'
          : '';
      var card = d.cards
          .where(
            (c) =>
                c.originalPath == path &&
                c.sourceLectureId == lecture &&
                (excerpt
                    ? c.documentId == identity
                    : c.type == (video ? 'video' : 'pdfdoc')),
          )
          .firstOrNull;
      if (card != null && !d.visible(card.layer)) {
        throw FormatException(l10n.studyEvidenceMissing);
      }
      if (card == null) {
        if (!d.editable(d.activeLayer) || d.cards.length >= 1998) {
          throw FormatException(l10n.studyLocked);
        }
        if (excerpt) {
          card = StudyCard(
            id: studyId(),
            type: 'text',
            layer: d.activeLayer,
            rect: _freeSpot(const Size(480, 360)),
            title: '${evidence['label'] ?? ''}',
            text: text,
            originalPath: path,
            sourceLectureId: lecture,
            documentId: identity,
          );
        } else {
          final imported = await widget.api.importStudyMaterial(
            widget.ctl.lectureId,
            path,
          );
          if (!mounted || document != d) return;
          card = StudyCard.fromJson((imported['objects'] as List).first as Map);
          card.layer = d.activeLayer;
          card.rect = _freeSpot(card.rect.size);
        }
        d.checkpoint();
        d.cards.add(card);
        d.changed();
        if (!await flush() || !mounted || document != d) return;
        _update(() {});
        await WidgetsBinding.instance.endOfFrame;
        if (!mounted || document != d) return;
      }
      var start = 0, end = 0, quote = '';
      if (excerpt) {
        end = text.length;
        quote = text;
      } else if (exactPDF) {
        // The passage is lit where the page's own text holds it verbatim;
        // otherwise the page itself is the citation.
        final shown = card.page == page
            ? card.text
            : '${(await widget.api.studyPDFPage(widget.ctl.lectureId, card.source, page) as Map)['text']}';
        if (!mounted || document != d) return;
        final words = text.trim();
        final at = words.isEmpty ? -1 : shown.indexOf(words);
        if (at >= 0) {
          start = at;
          end = at + words.length;
          quote = words;
        }
      }
      if (excerpt && card.sources.isEmpty) {
        card.sources.add(
          StudyAnchor(
            id: '${evidence['id']}',
            objectId: card.id,
            kind: 'text',
            workspaceId: widget.ctl.lectureId,
            sourceLectureId: lecture,
            originalPath: path,
            quote: text.substring(0, math.min(2000, text.length)),
          ),
        );
      }
      await _followEvidence({
        'id': evidence['id'],
        'label': evidence['label'],
        'kind': video
            ? 'timestamped_transcript'
            : end > start
            ? 'exact_text'
            : 'source',
        'status': 'verified',
        'anchor': StudyAnchor(
          id: '${evidence['id']}',
          objectId: card.id,
          kind: video
              ? 'video'
              : exactPDF
              ? 'pdf'
              : 'text',
          workspaceId: widget.ctl.lectureId,
          sourceLectureId: lecture,
          originalPath: path,
          page: page,
          start: start,
          end: end,
          timeStartMs: (source['time_start_ms'] as num? ?? 0).toInt(),
          timeEndMs: (source['time_end_ms'] as num? ?? 0).toInt(),
          quote: quote,
        ).json(),
      });
    } on ApiError catch (e) {
      // 409: the file changed since the answer read it.
      if (mounted) {
        _toast(
          e.status == 409
              ? l10n.studyEvidenceStale
              : l10n.studyCitationUnavailable,
        );
      }
    } catch (e) {
      if (mounted) {
        _toast(
          e is FormatException ? e.message : l10n.studyCitationUnavailable,
        );
      }
    }
  }
}
