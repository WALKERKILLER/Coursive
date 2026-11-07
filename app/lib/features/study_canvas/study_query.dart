import 'dart:convert';

import 'study_document.dart';

/// Drop stale live references without rewriting historical answers or citations.
Map<String, dynamic> reconcileStudyQuery(
  Map<String, dynamic> query,
  StudyDocument document,
) {
  final out = jsonDecode(jsonEncode(query)) as Map<String, dynamic>;
  final visible = document.layers
      .where((l) => l.visible)
      .map((l) => l.id)
      .toSet();
  final cards = document.cards
      .where((c) => visible.contains(c.layer))
      .map((c) => c.id)
      .toSet();
  final chosen = ((out['objects'] as List?) ?? [])
      .where(cards.contains)
      .toSet();
  if (out.containsKey('objects')) out['objects'] = chosen.toList();
  for (final key in [
    'selections',
    'object_pages',
    'object_video_ms',
    'video_ranges',
  ]) {
    final values = out[key] as Map?;
    values?.removeWhere((id, _) => !chosen.contains(id));
  }
  final selection = out['selection'] as Map?;
  if (selection != null) {
    final object = selection['object_id'];
    if (object != null && object != '' && !chosen.contains(object)) {
      out['selection'] = {};
    } else if (selection['stroke_ids'] is List) {
      final strokes = document.strokes
          .where((s) => visible.contains(s.layer))
          .map((s) => s.id)
          .toSet();
      selection['stroke_ids'] = (selection['stroke_ids'] as List)
          .where(strokes.contains)
          .toList();
    }
  }
  return out;
}
