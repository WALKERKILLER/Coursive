// Exports the shared host fixture and a seven-section outline, then reads
// the PDF back with Poppler: text layer, contents pages and headings that
// never end a page.
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';

import '../lib/features/study_canvas/outline_document.dart';
import '../lib/features/study_canvas/outline_pdf.dart';

Map fixture() => jsonDecode(
  File('../daemon/internal/library/testdata/outline-turn-result.json')
      .readAsStringSync(),
) as Map;

OutlineFonts fonts() {
  ByteData read(String name) =>
      ByteData.sublistView(File('assets/fonts/$name').readAsBytesSync());
  return OutlineFonts(
    read('NotoSansSC-Regular.ttf'),
    read('NotoSansSC-Bold.ttf'),
  );
}

bool poppler() => [
  'pdftotext',
  'pdfinfo',
].every((t) => Process.runSync('which', [t]).exitCode == 0);

Future<List<String>> pages(Uint8List pdf) async {
  final dir = await Directory.systemTemp.createTemp('outline-test-');
  try {
    final file = File('${dir.path}/out.pdf')..writeAsBytesSync(pdf);
    final text =
        Process.runSync('pdftotext', [
              '-layout',
              '-enc',
              'UTF-8',
              file.path,
              '-',
            ]).stdout
            as String;
    return text.split('\f')..removeLast();
  } finally {
    dir.deleteSync(recursive: true);
  }
}

Map item(
  String id,
  String kind,
  String text, {
  List<String> ev = const [],
  String review = 'pending',
  String? choice,
  Map? anchor,
}) => {
  'id': id,
  'kind': kind,
  'text': text,
  'evidence_ids': ev,
  'review': review,
  'choice': ?choice,
  'anchor': ?anchor,
};

/// Seven sections with Chinese, formulas, a long code block, a table, a
/// picture-sized formula, a handwritten tick, a video moment and a stale page.
(Map, List) richOutline() {
  final evidence = [
    {
      'id': 'e1',
      'source_id': 'pdf',
      'kind': 'exact_text',
      'status': 'verified',
      'label': '线性代数.pdf',
      'anchor': {
        'id': 'pdf',
        'object_id': 'pdf',
        'kind': 'pdf',
        'page': 6,
        'start': 0,
        'end': 9,
        'quote': '矩阵乘法不满足交换律',
        'original_path': 'courses/线代/线性代数.pdf',
      },
    },
    {
      'id': 'e2',
      'source_id': 'lecture-video',
      'kind': 'timestamped_transcript',
      'status': 'verified',
      'label': '第 3 讲录像',
      'anchor': {
        'id': 'lecture-video',
        'workspace_id': 'L',
        'object_id': 'lecture',
        'kind': 'video',
        'time_start_ms': 754000,
        'time_end_ms': 761000,
        'quote': '注意顺序不能换',
      },
    },
    {
      'id': 'e3',
      'source_id': 'old',
      'kind': 'exact_text',
      'status': 'verified',
      'label': '旧讲义.pdf',
      'anchor': {
        'id': 'old',
        'object_id': 'old',
        'kind': 'pdf',
        'page': 2,
        'quote': '已删除的段落',
      },
    },
  ];
  final code = [
    '```python',
    for (var i = 0; i < 70; i++)
      'result[$i] = sum(a[$i][k] * b[k][$i] for k in range(n))  # 第 $i 行',
    '```',
  ].join('\n');
  final sections = [
    for (var s = 1; s <= 7; s++)
      {
        'id': 's$s',
        'heading': '第 $s 章 矩阵与变换',
        'level': 1,
        'items': [
          item(
            'a$s',
            'fact',
            r'矩阵乘法一般不满足交换律，$AB \ne BA$。',
            ev: ['e1'],
            review: 'confirmed',
          ),
          item(
            'b$s',
            'summary',
            '${'可逆矩阵的逆唯一，且 (AB)^{-1} = B^{-1}A^{-1}。' * 6}\n\n\$\$\\det(AB)=\\det(A)\\det(B)\$\$',
          ),
          item('c$s', 'fact', '老师在课上强调：乘法顺序不能随意交换。', ev: ['e2']),
          if (s == 3) item('code', 'summary', '逐元素计算矩阵乘积：\n$code'),
          if (s == 4)
            item(
              'table',
              'summary',
              '| 运算 | 交换律 | 结合律 |\n| --- | --- | --- |\n| 加法 | 满足 | 满足 |\n| 乘法 | 不满足 | 满足 |',
            ),
          if (s == 5) item('stale', 'fact', '旧讲义的结论。', ev: ['e3']),
          if (s == 6)
            item(
              'tick',
              'unresolved_mark',
              '第 6 页旁的 ✓',
              review: 'needs_interpretation',
              anchor: {
                'id': 'selected-ink',
                'object_id': 'pdf',
                'kind': 'ink',
                'stroke_ids': ['s1'],
              },
            ),
          if (s == 6)
            item(
              'star',
              'unresolved_mark',
              '公式旁的星号',
              review: 'confirmed',
              choice: 'key',
            ),
          if (s == 7) item('later', 'fact', '分块矩阵的乘法规则', review: 'to_check'),
          item('q$s', 'question', '为什么矩阵乘法不满足交换律？'),
        ],
      },
  ];
  return (
    {
      'version': 1,
      'proposal_id': '0f8c2d3e-1a2b-4c5d-8e9f-0a1b2c3d4e5f',
      'state': 'draft',
      'title': '线性代数复习提纲',
      'sections': sections,
      'conflicts': [
        {
          'text': '课件与课堂对顺序的说法不同',
          'evidence_ids': ['e1', 'e2'],
        },
      ],
      'unresolved_questions': ['分块矩阵什么时候可以交换？'],
    },
    evidence,
  );
}

void main() {
  test(
    'the host fixture becomes a note with readable sources and back links',
    () {
      final r = fixture();
      final doc = outlineExport(
        r['outline_proposal'] as Map,
        r['evidence'] as List,
        lectureId: 'L',
      );
      final md = outlineMarkdown(doc);
      expect(md, startsWith('# 缓存失效复习\n'));
      expect(
        md,
        contains(
          '\n## 缓存失效\n\n- 缓存未命中时要从主存读取，代价是 \$t_{miss}=t_{hit}+t_{penalty}\$。 [1]\n',
        ),
      );
      expect(
        md,
        contains(
          '\n### 课堂补充\n\n- 老师强调失效时读取主存。 [2]\n- **问题** 写回策略如何影响 miss 代价？\n',
        ),
      );
      expect(md, contains('## 待核实'));
      expect(
        md,
        contains('### 来源说法不一致\n\n- 课件写“代价”，课堂只说读取主存，没有给出公式。 [1][2]\n'),
      );
      // The unread tick is only in the appendix, never stated as mastered.
      expect(md, contains('### 手写标记\n\n- **未确认** 所选手写与圈画（1 笔）未能读取，请确认它的含义\n'));
      expect(md, isNot(contains('已掌握')));
      expect(
        md,
        contains(
          '1. Real parser fixture · p3 · 第 3 页 — “计算 cache miss 的代价。”\n',
        ),
      );
      expect(
        md,
        contains(
          '2. Lecture · 08:32–09:10 — “缓存失效时需要读取主存。” · [在 Coursive 中打开](coursive://lecture/L?t=512)\n',
        ),
      );
    },
  );

  test('confirmed marks join the body, stale facts are flagged, the rest waits in the appendix', () {
    final (p, ev) = richOutline();
    final md = outlineMarkdown(outlineExport(p, ev, stale: {'e3'}));
    expect(md, contains('- **重点** 公式旁的星号\n'));
    expect(md, contains('- **未核实** 旧讲义的结论。 [3]\n'));
    expect(md, contains('### 待查\n\n- **待查** 分块矩阵的乘法规则\n'));
    expect(md, contains('- **未确认** 第 6 页旁的 ✓\n'));
    expect(
      md,
      contains(
        '线性代数.pdf · 第 6 页 — “矩阵乘法不满足交换律” · [在 Coursive 中打开](coursive://file/courses/${Uri.encodeComponent('线代')}/${Uri.encodeComponent('线性代数.pdf')})',
      ),
    );
    expect(md, contains('第 3 讲录像 · 12:34–12:41'));
    expect(md, contains('旧讲义.pdf · 第 2 页 — “已删除的段落”（依据已失效）'));
    // The code block closes on its own line before the citation.
    expect(md, contains('  ```\n'));
    final stats = outlineStats(p, {'e3'});
    expect(
      [stats.total, stats.reviewed, stats.marks, stats.stale, stats.unverified],
      [34, 8, 1, 1, 2],
    );
    expect(outlineNoteHash('a'), 'af63dc4c8601ec8c');
  });

  test('font coverage is known before layout', () {
    final f = fonts();
    expect(f.codepoints.containsAll('矩阵乘法✓→≠αβ，。“”'.runes), isTrue);
    final (p, ev) = richOutline();
    final doc = outlineExport(p, ev);
    expect(outlineMissingGlyphs(doc, f.codepoints), isEmpty);
    (p['sections'] as List).first['heading'] = '𠀀 生僻字';
    expect(outlineMissingGlyphs(outlineExport(p, ev), f.codepoints), {'𠀀'});
    expect(
      outlineFormulas(doc),
      containsAll([
        (tex: r'AB \ne BA', display: false),
        (tex: r'\det(AB)=\det(A)\det(B)', display: true),
      ]),
    );
  });

  test('a seven-section A4 outline has a text layer, true contents pages and no stranded headings', () async {
    if (!poppler()) {
      markTestSkipped('Poppler (pdftotext) is not installed');
      return;
    }
    final (p, ev) = richOutline();
    final doc = outlineExport(p, ev, stale: {'e3'});
    final pdf = await buildOutlinePdf(
      doc,
      const OutlineLayout(),
      fonts(),
      date: DateTime(2026, 11, 7),
    );
    Directory('build').createSync();
    File('build/outline-test.pdf').writeAsBytesSync(pdf);
    expect(String.fromCharCodes(pdf.take(5)), '%PDF-');
    final text = await pages(pdf);
    expect(text.length, greaterThan(3));
    expect(text.first, contains('线性代数复习提纲'));
    expect(text.first, contains('目录'));
    final all = text.join('\n');
    for (final s in [
      '矩阵乘法一般不满足交换律',
      '老师在课上强调',
      'result[69]',
      '不满足',
      '分块矩阵的乘法规则',
      '课件与课堂对顺序的说法不同',
      '在 Coursive 中打开',
    ]) {
      expect(all, contains(s), reason: s);
    }
    // Each contents entry names the page its heading is on.
    for (var s = 1; s <= 7; s++) {
      final heading = '第 $s 章 矩阵与变换';
      final line = text.first
          .split('\n')
          .firstWhere((l) => l.contains(heading));
      final page = int.parse(RegExp(r'(\d+)\s*$').firstMatch(line)!.group(1)!);
      expect(
        text[page - 1],
        contains(heading),
        reason: '$heading on page $page',
      );
    }
    for (final (i, page) in text.indexed.skip(1)) {
      final lines = page
          .split('\n')
          .map((l) => l.trim())
          .where((l) => l.isNotEmpty)
          .toList();
      // The last line before the footer is never a heading.
      final last = lines.length >= 2 ? lines[lines.length - 2] : '';
      expect(
        RegExp(r'^第 \d 章 矩阵与变换$').hasMatch(last),
        isFalse,
        reason: 'page ${i + 1} ends with "$last"',
      );
      expect(
        lines.last.replaceAll(' ', ''),
        endsWith('${i + 1}/${text.length}'),
      );
      // An item's introduction stays with its code, and a formula with its text.
      expect(last, isNot('• 逐元素计算矩阵乘积：'));
      if (i > 0) expect(lines.first, isNot(startsWith(r'\det')));
    }
    final letter = await buildOutlinePdf(
      doc,
      const OutlineLayout(
        paper: OutlinePaper.letter,
        landscape: true,
        compact: true,
      ),
      fonts(),
      date: DateTime(2026, 11, 7),
    );
    expect(letter, isNot(equals(pdf)));
  });
}
