// A reviewed outline as semantic blocks: the single source for the lecture
// note's Markdown and the exported PDF (spec 0028).
import 'dart:convert';

import 'package:flutter/widgets.dart' show StringCharacters;

import '../../core/format.dart';
import '../../l10n/l10n.dart';

/// Meanings a learner can give a handwritten mark, in display order.
const outlineChoices = ['mastered', 'key', 'review', 'custom', 'keep'];

String outlineChoiceLabel(String choice, [String custom = '']) =>
    switch (choice) {
      'mastered' => l10n.outlineChoiceMastered,
      'key' => l10n.outlineChoiceKey,
      'review' => l10n.outlineChoiceReview,
      'custom' =>
        custom.trim().isEmpty ? l10n.outlineChoiceCustom : custom.trim(),
      _ => l10n.outlineChoiceKeep,
    };

/// The outline proposal of a turn, when it has one.
Map? outlineProposal(Map turn) {
  final p = (turn['result'] as Map?)?['outline_proposal'];
  return p is Map && p['version'] == 1 ? p : null;
}

Iterable<Map> outlineItems(Map proposal) sync* {
  for (final s in proposal['sections'] as List? ?? const []) {
    if (s is! Map) continue;
    for (final i in s['items'] as List? ?? const []) {
      if (i is Map) yield i;
    }
  }
}

List<String> outlineEvidenceIds(Map item) => [
  for (final id in item['evidence_ids'] as List? ?? const []) '$id',
];

/// A fact counts as checked only while one of its citations still holds.
bool outlineVerified(Map item, Set<String> stale) =>
    outlineEvidenceIds(item).any((id) => !stale.contains(id));

class OutlineStats {
  const OutlineStats({
    required this.reviewed,
    required this.total,
    required this.stale,
    required this.marks,
    required this.unverified,
  });
  final int reviewed, total, stale, marks, unverified;
}

OutlineStats outlineStats(Map proposal, Set<String> stale) {
  var reviewed = 0, total = 0, broken = 0, marks = 0, unverified = 0;
  for (final i in outlineItems(proposal)) {
    total++;
    if (i['review'] == 'confirmed') reviewed++;
    if (i['review'] == 'needs_interpretation') marks++;
    if (outlineEvidenceIds(i).any(stale.contains)) broken++;
    if (i['kind'] == 'fact' && !outlineVerified(i, stale)) unverified++;
  }
  return OutlineStats(
    reviewed: reviewed,
    total: total,
    stale: broken,
    marks: marks,
    unverified: unverified,
  );
}

/// FNV-1a 64 of a saved note body; a different hash later means the note
/// changed somewhere else.
String outlineNoteHash(String body) {
  var h = BigInt.parse('cbf29ce484222325', radix: 16);
  final prime = BigInt.parse('100000001b3', radix: 16);
  final mask = (BigInt.one << 64) - BigInt.one;
  for (final b in utf8.encode(body)) {
    h = ((h ^ BigInt.from(b)) * prime) & mask;
  }
  return h.toRadixString(16).padLeft(16, '0');
}

/// The note an outline is saved as; retries reuse it instead of adding one.
String outlineNoteId(Map proposal) => 'outline-${proposal['proposal_id']}';

sealed class OutlineBlock {
  const OutlineBlock();
}

class OutlineHeading extends OutlineBlock {
  const OutlineHeading(this.level, this.text);

  /// 2 to 4; the title is level 1.
  final int level;
  final String text;
}

/// A list entry: an outline item, conflict, question or unread source.
class OutlineEntry extends OutlineBlock {
  const OutlineEntry(this.text, {this.refs = const [], this.tag, this.item});
  final String text;
  final List<int> refs;
  final String? tag;

  /// The outline item this entry shows; marks use it to find their ink.
  final Map? item;
}

/// A short line under a heading.
class OutlineRemark extends OutlineBlock {
  const OutlineRemark(this.text);
  final String text;
}

/// A cited source, numbered by first use.
class OutlineSource {
  const OutlineSource({
    required this.number,
    required this.label,
    required this.locator,
    required this.quote,
    required this.link,
    required this.stale,
  });
  final int number;
  final String label, locator, quote, link;
  final bool stale;
}

class OutlineExport {
  const OutlineExport({
    required this.title,
    required this.body,
    required this.appendix,
    required this.sources,
  });
  final String title;
  final List<OutlineBlock> body, appendix;
  final List<OutlineSource> sources;

  Iterable<OutlineBlock> get blocks => [...body, ...appendix];
}

String _locator(Map e) {
  final a = e['anchor'] as Map? ?? const {};
  int n(Object? v) => v is num ? v.toInt() : 0;
  return switch ('${e['kind']}') {
    'timestamped_transcript' =>
      '${fmtClock(n(a['time_start_ms']))}–${fmtClock(n(a['time_end_ms']))}',
    'visual_observation' => l10n.studyEvidenceSeen,
    _ when a['kind'] == 'ink' => l10n.studyEvidenceInk,
    _ when n(a['page']) > 0 => l10n.studySourcePage(n(a['page'])),
    _ => '',
  };
}

/// A `coursive://` link back to the cited place, when the app can open it.
String _link(Map e, String lectureId) {
  final a = e['anchor'] as Map? ?? const {};
  if (a['kind'] == 'video' && a['object_id'] == 'lecture') {
    final lecture = '${a['workspace_id'] ?? ''}'.isNotEmpty
        ? '${a['workspace_id']}'
        : lectureId;
    final ms = a['time_start_ms'] is num ? a['time_start_ms'] as num : 0;
    return 'coursive://lecture/${Uri.encodeComponent(lecture)}?t=${ms ~/ 1000}';
  }
  final path = '${a['original_path'] ?? ''}';
  if (path.isEmpty) return '';
  return 'coursive://file/${path.split('/').map(Uri.encodeComponent).join('/')}';
}

/// What a reviewed outline says, split into the body (what the learner
/// accepts as notes) and an appendix of what still needs checking:
/// conflicts, uninterpreted marks, items marked for checking, open
/// questions and sources that were not read.
OutlineExport outlineExport(
  Map proposal,
  List evidence, {
  Set<String> stale = const {},
  String lectureId = '',
}) {
  final byId = {for (final e in evidence.whereType<Map>()) '${e['id']}': e};
  final numbers = <String, int>{};
  final sources = <OutlineSource>[];
  List<int> refs(List<String> ids) => [
    for (final id in ids)
      if (byId[id] case final e?)
        numbers.putIfAbsent(id, () {
          final quote = '${(e['anchor'] as Map?)?['quote'] ?? ''}'.trim();
          sources.add(
            OutlineSource(
              number: sources.length + 1,
              label: '${e['label'] ?? ''}'.trim(),
              locator: _locator(e),
              quote: quote,
              link: _link(e, lectureId),
              stale: stale.contains(id) || e['status'] != 'verified',
            ),
          );
          return sources.length;
        }),
  ];
  final body = <OutlineBlock>[];
  final marks = <OutlineEntry>[], checks = <OutlineEntry>[];
  for (final s
      in (proposal['sections'] as List? ?? const []).whereType<Map>()) {
    final heading = '${s['heading'] ?? ''}'.trim();
    final level = s['level'] is int ? (s['level'] as int).clamp(1, 3) : 1;
    // A section whose items all wait in the appendix has no heading here.
    final at = body.length;
    for (final i in (s['items'] as List? ?? const []).whereType<Map>()) {
      final text = '${i['text'] ?? ''}'.trim();
      if (text.isEmpty) continue;
      final cited = refs(outlineEvidenceIds(i));
      final choice = '${i['choice'] ?? ''}';
      if (i['kind'] == 'unresolved_mark') {
        if (const {'mastered', 'key', 'review', 'custom'}.contains(choice)) {
          body.add(
            OutlineEntry(
              text,
              refs: cited,
              tag: outlineChoiceLabel(choice, '${i['choice_text'] ?? ''}'),
              item: i,
            ),
          );
        } else {
          marks.add(
            OutlineEntry(
              text,
              refs: cited,
              tag: choice == 'keep'
                  ? l10n.outlineChoiceKeep
                  : l10n.outlineTagUnconfirmed,
              item: i,
            ),
          );
        }
      } else if (i['review'] == 'to_check') {
        checks.add(
          OutlineEntry(text, refs: cited, tag: l10n.outlineTagCheck, item: i),
        );
      } else {
        body.add(
          OutlineEntry(
            text,
            refs: cited,
            tag: i['kind'] == 'question'
                ? l10n.outlineTagQuestion
                : i['kind'] == 'fact' && !outlineVerified(i, stale)
                ? l10n.outlineTagUnverified
                : null,
            item: i,
          ),
        );
      }
    }
    if (heading.isNotEmpty && body.length > at) {
      body.insert(at, OutlineHeading(level + 1, heading));
    }
  }
  final conflicts = [
    for (final c
        in (proposal['conflicts'] as List? ?? const []).whereType<Map>())
      if ('${c['text'] ?? ''}'.trim().isNotEmpty)
        OutlineEntry(
          '${c['text']}'.trim(),
          refs: refs([
            for (final id in c['evidence_ids'] as List? ?? const []) '$id',
          ]),
        ),
  ];
  List<OutlineEntry> plain(String key) => [
    for (final q in proposal[key] as List? ?? const [])
      if ('$q'.trim().isNotEmpty) OutlineEntry('$q'.trim()),
  ];
  final groups = [
    (l10n.outlineConflicts, conflicts),
    (l10n.outlineMarks, marks),
    (l10n.outlineChecks, checks),
    (l10n.outlineQuestions, plain('unresolved_questions')),
    (l10n.outlineUnread, plain('missing_sources')),
  ].where((g) => g.$2.isNotEmpty);
  final appendix = <OutlineBlock>[
    if (groups.isNotEmpty) ...[
      OutlineHeading(2, l10n.outlineAppendix),
      OutlineRemark(l10n.outlineAppendixNote),
      for (final g in groups) ...[OutlineHeading(3, g.$1), ...g.$2],
    ],
  ];
  final title = '${proposal['title'] ?? ''}'.trim();
  return OutlineExport(
    title: title.isEmpty ? l10n.outlineUntitled : title,
    body: body,
    appendix: appendix,
    sources: sources,
  );
}

/// The lecture note body: headings, lists with numbered citations, the
/// appendix and a source list whose `coursive://` links sit beside readable
/// file names, pages and times.
String outlineMarkdown(OutlineExport doc) {
  final out = StringBuffer('# ${doc.title}\n');
  void entry(OutlineEntry e) {
    final lines = e.text.split('\n');
    final refs = e.refs.map((n) => '[$n]').join();
    final tail = lines.last.trimLeft();
    // A closing fence or display formula keeps its own line.
    final ownLine = tail.startsWith('```') || tail.startsWith(r'$$');
    final head = e.tag == null ? '' : '**${e.tag}** ';
    out.write('- $head${lines.first}');
    for (final l in lines.skip(1)) {
      out.write('\n  $l');
    }
    if (refs.isNotEmpty) out.write(ownLine ? '\n  $refs' : ' $refs');
    out.write('\n');
  }

  for (final b in doc.blocks) {
    switch (b) {
      case OutlineHeading(:final level, :final text):
        out.write('\n${'#' * level} $text\n\n');
      case OutlineRemark(:final text):
        out.write('$text\n\n');
      case OutlineEntry():
        entry(b);
    }
  }
  if (doc.sources.isNotEmpty) {
    out.write('\n## ${l10n.outlineSources}\n\n');
    for (final s in doc.sources) {
      final where = [s.label, s.locator].where((v) => v.isNotEmpty).join(' · ');
      out.write('${s.number}. $where');
      if (s.quote.isNotEmpty) out.write(' — “${_short(s.quote)}”');
      if (s.stale) out.write('（${l10n.outlineSourceStale}）');
      if (s.link.isNotEmpty) {
        out.write(' · [${l10n.outlineOpenSource}](${s.link})');
      }
      out.write('\n');
    }
  }
  return out.toString().replaceAll(RegExp(r'\n{3,}'), '\n\n');
}

String _short(String quote) {
  final q = quote.replaceAll(RegExp(r'\s+'), ' ');
  return q.characters.length > 80 ? '${q.characters.take(80)}…' : q;
}
