// Lays out a reviewed outline as a paginated PDF with a real text layer
// (spec 0028, ADR 0030). The bytes built here are the bytes previewed and
// saved; nothing is drawn twice.
import 'dart:typed_data';

import 'package:flutter/services.dart' show rootBundle;
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;

import '../../l10n/l10n.dart';
import 'outline_document.dart';

enum OutlinePaper { a4, letter }

class OutlineLayout {
  const OutlineLayout({
    this.paper = OutlinePaper.a4,
    this.landscape = false,
    this.compact = false,
  });
  final OutlinePaper paper;
  final bool landscape, compact;

  PdfPageFormat get format {
    final f = paper == OutlinePaper.a4
        ? PdfPageFormat.a4
        : PdfPageFormat.letter;
    return landscape ? f.landscape : f.portrait;
  }

  double get margin => (compact ? 12 : 20) * PdfPageFormat.mm;

  OutlineLayout copyWith({
    OutlinePaper? paper,
    bool? landscape,
    bool? compact,
  }) => OutlineLayout(
    paper: paper ?? this.paper,
    landscape: landscape ?? this.landscape,
    compact: compact ?? this.compact,
  );

  @override
  bool operator ==(Object other) =>
      other is OutlineLayout &&
      other.paper == paper &&
      other.landscape == landscape &&
      other.compact == compact;

  @override
  int get hashCode => Object.hash(paper, landscape, compact);
}

/// The embedded Noto Sans SC subset and the characters it can draw.
class OutlineFonts {
  OutlineFonts(ByteData regular, ByteData bold)
    : regular = pw.Font.ttf(regular),
      bold = pw.Font.ttf(bold),
      codepoints = ttfCodepoints(regular);
  final pw.Font regular, bold;
  final Set<int> codepoints;

  static Future<OutlineFonts> load() async => OutlineFonts(
    await rootBundle.load('assets/fonts/NotoSansSC-Regular.ttf'),
    await rootBundle.load('assets/fonts/NotoSansSC-Bold.ttf'),
  );
}

/// The Unicode code points a TrueType font maps to a glyph (cmap 4 or 12).
Set<int> ttfCodepoints(ByteData d) {
  final out = <int>{};
  int? cmap;
  for (var i = 0, n = d.getUint16(4); i < n; i++) {
    final at = 12 + 16 * i;
    if (String.fromCharCodes([
          for (var k = 0; k < 4; k++) d.getUint8(at + k),
        ]) ==
        'cmap') {
      cmap = d.getUint32(at + 8);
    }
  }
  if (cmap == null) return out;
  int? best, bestFormat;
  for (var i = 0, n = d.getUint16(cmap + 2); i < n; i++) {
    final at = cmap + 4 + 8 * i;
    final platform = d.getUint16(at), encoding = d.getUint16(at + 2);
    final table = cmap + d.getUint32(at + 4), format = d.getUint16(table);
    final unicode =
        platform == 0 || (platform == 3 && (encoding == 1 || encoding == 10));
    if (unicode && (format == 12 || (format == 4 && bestFormat != 12))) {
      best = table;
      bestFormat = format;
    }
  }
  if (best == null) return out;
  if (bestFormat == 12) {
    for (var g = 0, n = d.getUint32(best + 12); g < n; g++) {
      final at = best + 16 + 12 * g;
      final start = d.getUint32(at),
          end = d.getUint32(at + 4),
          glyph = d.getUint32(at + 8);
      for (var c = start; c <= end; c++) {
        if (glyph + c - start != 0) out.add(c);
      }
    }
    return out;
  }
  final seg2 = d.getUint16(best + 6);
  final ends = best + 14,
      starts = ends + seg2 + 2,
      deltas = starts + seg2,
      ranges = deltas + seg2;
  for (var s = 0; s < seg2; s += 2) {
    final start = d.getUint16(starts + s), end = d.getUint16(ends + s);
    final delta = d.getInt16(deltas + s), range = d.getUint16(ranges + s);
    for (var c = start; c <= end && c != 0xffff; c++) {
      final glyph = range == 0
          ? (c + delta) & 0xffff
          : d.getUint16(ranges + s + range + 2 * (c - start));
      if (glyph != 0) out.add(c);
    }
  }
  return out;
}

/// A piece of an item's text: a paragraph, code, a display formula or a table.
sealed class OutlineTextBlock {
  const OutlineTextBlock();
}

class OutlineParagraph extends OutlineTextBlock {
  const OutlineParagraph(this.text);
  final String text;
}

class OutlineCode extends OutlineTextBlock {
  const OutlineCode(this.lines);
  final List<String> lines;
}

class OutlineMath extends OutlineTextBlock {
  const OutlineMath(this.tex);
  final String tex;
}

class OutlineTable extends OutlineTextBlock {
  const OutlineTable(this.rows);
  final List<List<String>> rows;
}

final _tableRule = RegExp(r'^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$');
List<String> _cells(String line) {
  var l = line.trim();
  if (l.startsWith('|')) l = l.substring(1);
  if (l.endsWith('|')) l = l.substring(0, l.length - 1);
  return l.split('|').map((c) => c.trim()).toList();
}

/// Splits Markdown an item may carry into blocks; inline marks stay in text.
List<OutlineTextBlock> outlineTextBlocks(String text) {
  final out = <OutlineTextBlock>[];
  final lines = text.split('\n');
  final para = <String>[];
  void flush() {
    if (para.isNotEmpty) out.add(OutlineParagraph(para.join('\n')));
    para.clear();
  }

  for (var i = 0; i < lines.length; i++) {
    final line = lines[i], t = line.trim();
    if (t.startsWith('```')) {
      flush();
      final code = <String>[];
      while (++i < lines.length && !lines[i].trim().startsWith('```')) {
        code.add(lines[i]);
      }
      out.add(OutlineCode(code));
    } else if (t.startsWith(r'$$') || t.startsWith(r'\[')) {
      flush();
      final close = t.startsWith(r'$$') ? r'$$' : r'\]';
      final tex = StringBuffer(t.substring(2));
      var body = tex.toString();
      while (!body.trimRight().endsWith(close) && ++i < lines.length) {
        tex.write('\n${lines[i]}');
        body = tex.toString();
      }
      body = body.trimRight();
      if (body.endsWith(close)) body = body.substring(0, body.length - 2);
      if (body.trim().isNotEmpty) out.add(OutlineMath(body.trim()));
    } else if (t.contains('|') &&
        i + 1 < lines.length &&
        _tableRule.hasMatch(lines[i + 1])) {
      flush();
      final rows = [_cells(line)];
      i++;
      while (i + 1 < lines.length && lines[i + 1].contains('|')) {
        rows.add(_cells(lines[++i]));
      }
      out.add(OutlineTable(rows));
    } else if (t.isEmpty) {
      flush();
    } else {
      para.add(t);
    }
  }
  flush();
  return out;
}

/// Every formula of [doc], so they can be drawn before layout.
Set<({String tex, bool display})> outlineFormulas(OutlineExport doc) => {
  for (final b in doc.blocks)
    if (b is OutlineEntry)
      for (final t in outlineTextBlocks(b.text))
        ...switch (t) {
          OutlineMath(:final tex) => [(tex: tex, display: true)],
          OutlineParagraph(:final text) => [
            for (final r in outlineInline(text))
              if (r.kind == OutlineRunKind.math ||
                  r.kind == OutlineRunKind.displayMath)
                (tex: r.text, display: r.kind == OutlineRunKind.displayMath),
          ],
          _ => const <({String tex, bool display})>[],
        },
};

/// Characters in [doc] the embedded font cannot draw; formulas are images.
Set<String> outlineMissingGlyphs(OutlineExport doc, Set<int> codepoints) {
  final missing = <String>{};
  void check(String s) {
    for (final r in s.runes) {
      if (r > 0x20 && r != 0x7f && !codepoints.contains(r)) {
        missing.add(String.fromCharCode(r));
      }
    }
  }

  check(doc.title);
  for (final b in doc.blocks) {
    switch (b) {
      case OutlineHeading(:final text) || OutlineRemark(:final text):
        check(text);
      case OutlineEntry(:final text, :final tag):
        check(tag ?? '');
        for (final t in outlineTextBlocks(text)) {
          switch (t) {
            case OutlineParagraph(:final text):
              for (final r in outlineInline(text)) {
                if (r.kind != OutlineRunKind.math &&
                    r.kind != OutlineRunKind.displayMath) {
                  check(r.text);
                }
              }
            case OutlineCode(:final lines):
              check(lines.join());
            case OutlineTable(:final rows):
              check(rows.expand((r) => r).join());
            case OutlineMath():
          }
        }
    }
  }
  for (final s in doc.sources) {
    check('${s.label}${s.locator}${s.quote}');
  }
  return missing;
}

/// A formula or ink picture drawn ahead of layout, sized in points;
/// [baseline] is a formula's alphabetic baseline from its top.
typedef OutlineImage = ({
  Uint8List png,
  double width,
  double height,
  double? baseline,
});

String outlineMathKey(String tex, bool display) =>
    '${display ? 'D' : 'I'}:$tex';
String outlineInkKey(Map item) => 'ink:${item['id']}';

const _ink = PdfColor.fromInt(0xff1d2125);
const _muted = PdfColor.fromInt(0xff69707a);
const _rule = PdfColor.fromInt(0xffdadde2);
const _accent = PdfColor.fromInt(0xff2c6770);
const _amber = PdfColor.fromInt(0xffa15f0a);
const _paper = PdfColor.fromInt(0xfff4f5f7);

Future<Uint8List> buildOutlinePdf(
  OutlineExport doc,
  OutlineLayout layout,
  OutlineFonts fonts, {
  Map<String, OutlineImage> images = const {},
  DateTime? date,
}) async {
  final pdf = pw.Document(
    title: doc.title,
    creator: 'Coursive',
    producer: 'Coursive',
    compress: true,
  );
  final base = pw.TextStyle(fontSize: 10.5, lineSpacing: 4.5, color: _ink);
  final mono = pw.TextStyle(
    font: pw.Font.courier(),
    fontFallback: [fonts.regular],
    fontSize: 9,
    color: _ink,
  );
  final theme = pw.ThemeData.withFont(
    base: fonts.regular,
    bold: fonts.bold,
    fontFallback: [fonts.regular],
  ).copyWith(defaultTextStyle: base);
  final pageTheme = pw.PageTheme(
    pageFormat: layout.format,
    margin: pw.EdgeInsets.all(layout.margin),
    theme: theme,
  );
  final format = layout.format;
  final contentWidth = format.width - 2 * layout.margin,
      contentHeight = format.height - 2 * layout.margin;

  final headings = doc.blocks.whereType<OutlineHeading>().toList();
  // Sections and the appendix's groups go into the contents when they fit.
  final room = ((contentHeight - 150) / 17).floor();
  final deep = headings.length + 1 <= room;
  final toc =
      headings.where((h) => h.level == 2).length +
          (doc.sources.isEmpty ? 0 : 1) >=
      4;

  pw.InlineSpan inline(String text, pw.TextStyle style) {
    final spans = <pw.InlineSpan>[];
    for (final r in outlineInline(text)) {
      switch (r.kind) {
        case OutlineRunKind.text:
          spans.add(pw.TextSpan(text: r.text));
        case OutlineRunKind.bold:
          spans.add(
            pw.TextSpan(
              text: r.text,
              style: pw.TextStyle(fontWeight: pw.FontWeight.bold),
            ),
          );
        case OutlineRunKind.code:
          spans.add(
            pw.TextSpan(
              text: r.text,
              style: mono.copyWith(
                fontSize: (style.fontSize ?? 10.5) - 1,
                background: const pw.BoxDecoration(color: _paper),
              ),
            ),
          );
        case OutlineRunKind.math || OutlineRunKind.displayMath:
          final display = r.kind == OutlineRunKind.displayMath;
          final image = images[outlineMathKey(r.text, display)];
          // A span's bottom sits on the text baseline and a positive
          // baseline raises it, so the formula drops by its own descent.
          spans.add(
            image == null
                ? pw.TextSpan(text: r.text, style: mono)
                : pw.WidgetSpan(
                    baseline: switch (image.baseline) {
                      final b? => b - image.height,
                      null => -image.height * .22,
                    },
                    child: pw.Image(
                      pw.MemoryImage(image.png),
                      width: image.width,
                      height: image.height,
                    ),
                  ),
          );
      }
    }
    return pw.TextSpan(style: style, children: spans);
  }

  pw.InlineSpan refs(List<int> numbers) => pw.TextSpan(
    children: [
      for (final n in numbers)
        pw.WidgetSpan(
          baseline: 3.5,
          child: pw.Link(
            destination: 'source-$n',
            child: pw.Padding(
              padding: const pw.EdgeInsets.only(left: 1.5),
              child: pw.Text(
                '$n',
                style: pw.TextStyle(
                  fontSize: 6.8,
                  color: _accent,
                  fontWeight: pw.FontWeight.bold,
                ),
              ),
            ),
          ),
        ),
    ],
  );

  pw.InlineSpan tag(String? text, {bool warn = false}) => text == null
      ? const pw.TextSpan(text: '')
      : pw.TextSpan(
          text: '$text  ',
          style: pw.TextStyle(
            fontSize: 8.8,
            fontWeight: pw.FontWeight.bold,
            color: warn ? _amber : _accent,
          ),
        );

  // Long code may break between slices of lines, never inside a line.
  List<pw.Widget> code(List<String> lines) {
    const size = 8, r = pw.Radius.circular(3);
    final slices = [
      for (var i = 0; i < lines.length; i += size)
        lines.sublist(i, (i + size).clamp(0, lines.length)),
    ];
    return [
      for (final (i, slice)
          in (slices.isEmpty
                  ? [
                      <String>[''],
                    ]
                  : slices)
              .indexed)
        pw.Container(
          width: double.infinity,
          padding: pw.EdgeInsets.fromLTRB(
            8,
            i == 0 ? 6 : 0,
            8,
            i == slices.length - 1 || slices.isEmpty ? 6 : 0,
          ),
          decoration: pw.BoxDecoration(
            color: _paper,
            borderRadius: pw.BorderRadius.only(
              topLeft: i == 0 ? r : pw.Radius.zero,
              topRight: i == 0 ? r : pw.Radius.zero,
              bottomLeft: i == slices.length - 1 ? r : pw.Radius.zero,
              bottomRight: i == slices.length - 1 ? r : pw.Radius.zero,
            ),
          ),
          child: pw.Text(
            slice.join('\n'),
            style: mono.copyWith(lineSpacing: 2.5),
          ),
        ),
    ];
  }

  pw.Widget block(OutlineTextBlock b) => switch (b) {
    OutlineParagraph(:final text) => pw.RichText(text: inline(text, base)),
    OutlineCode(:final lines) => pw.Column(children: code(lines)),
    OutlineMath(:final tex) => switch (images[outlineMathKey(tex, true)]) {
      final image? => pw.Center(
        child: pw.Padding(
          padding: const pw.EdgeInsets.symmetric(vertical: 4),
          child: pw.Image(
            pw.MemoryImage(image.png),
            width: image.width > contentWidth - 20
                ? contentWidth - 20
                : image.width,
          ),
        ),
      ),
      null => pw.Text(tex, style: mono),
    },
    OutlineTable(:final rows) => pw.TableHelper.fromTextArray(
      data: rows,
      headerCount: 1,
      border: const pw.TableBorder(
        horizontalInside: pw.BorderSide(color: _rule, width: .5),
        top: pw.BorderSide(color: _rule, width: .5),
        bottom: pw.BorderSide(color: _rule, width: .5),
      ),
      headerStyle: base.copyWith(fontWeight: pw.FontWeight.bold, fontSize: 9.5),
      cellStyle: base.copyWith(fontSize: 9.5),
      headerDecoration: const pw.BoxDecoration(color: _paper),
      cellAlignment: pw.Alignment.centerLeft,
      cellPadding: const pw.EdgeInsets.symmetric(horizontal: 5, vertical: 3),
    ),
  };

  // A rough height, enough to keep a short item and its formula together.
  double estimate(OutlineTextBlock b) => switch (b) {
    OutlineParagraph(:final text) =>
      15 *
              (text.runes.fold<double>(
                        0,
                        (w, r) => w + (r < 0x2e80 ? 5.8 : 10.5),
                      ) /
                      (contentWidth - 13))
                  .ceil() +
          5,
    OutlineCode(:final lines) => 12.5 * lines.length + 17,
    OutlineMath(:final tex) =>
      (images[outlineMathKey(tex, true)]?.height ?? 14) + 13,
    OutlineTable(:final rows) => 17.0 * rows.length + 8,
  };

  List<pw.Widget> entry(OutlineEntry e, {bool appendix = false}) {
    final parts = outlineTextBlocks(e.text);
    final warn =
        e.tag == l10n.outlineTagUnverified ||
        e.tag == l10n.outlineTagCheck ||
        e.tag == l10n.outlineTagUnconfirmed;
    final ink = e.item == null ? null : images[outlineInkKey(e.item!)];
    final last = parts.lastOrNull;
    // Very long paragraphs may continue on the next page.
    if (parts.length == 1 &&
        last is OutlineParagraph &&
        last.text.length > 700 &&
        ink == null) {
      return [
        pw.SizedBox(height: 2),
        pw.RichText(
          overflow: pw.TextOverflow.span,
          text: pw.TextSpan(
            children: [
              const pw.TextSpan(
                text: '•  ',
                style: pw.TextStyle(color: _muted),
              ),
              tag(e.tag, warn: warn),
              inline(last.text, base),
              refs(e.refs),
            ],
          ),
        ),
        pw.SizedBox(height: 5),
      ];
    }
    final pieces = <pw.Widget>[];
    for (final (i, p) in parts.indexed) {
      if (p is OutlineParagraph) {
        pieces.add(
          pw.RichText(
            text: pw.TextSpan(
              children: [
                if (i == 0) tag(e.tag, warn: warn),
                inline(p.text, base),
                if (identical(p, last)) refs(e.refs),
              ],
            ),
          ),
        );
      } else {
        if (i == 0 && e.tag != null) {
          pieces.add(pw.RichText(text: tag(e.tag, warn: warn)));
        }
        pieces.addAll(p is OutlineCode ? code(p.lines) : [block(p)]);
      }
    }
    if (pieces.isEmpty) pieces.add(pw.RichText(text: tag(e.tag, warn: warn)));
    if (last is! OutlineParagraph && e.refs.isNotEmpty) {
      pieces.add(pw.RichText(text: refs(e.refs)));
    }
    // Estimates run short on mixed scripts; leave a quarter of slack.
    final total = 1.25 * parts.fold<double>(0, (h, p) => h + estimate(p));
    double gap(int i) => i == pieces.length - 1 || parts.length == 1
        ? 5
        : (pieces[i + 1] is pw.Container && pieces[i] is pw.Container ? 0 : 5);
    return [
      // A short item stays whole; a long one keeps its opening with what follows.
      if (parts.length > 1)
        pw.NewPage(
          freeSpace: total <= contentHeight * .4
              ? total
              : (1.25 * (estimate(parts[0]) + estimate(parts[1]))).clamp(
                  0,
                  contentHeight * .6,
                ),
        ),
      for (final (i, piece) in pieces.indexed)
        if (i == 0)
          pw.Padding(
            padding: pw.EdgeInsets.only(bottom: gap(i)),
            child: pw.Row(
              crossAxisAlignment: pw.CrossAxisAlignment.start,
              children: [
                pw.SizedBox(
                  width: 13,
                  child: pw.Text(
                    appendix ? '○' : '•',
                    style: base.copyWith(color: _muted),
                  ),
                ),
                if (ink != null)
                  pw.Container(
                    width: 54,
                    height: 40,
                    margin: const pw.EdgeInsets.only(right: 8, top: 1),
                    padding: const pw.EdgeInsets.all(3),
                    decoration: pw.BoxDecoration(
                      border: pw.Border.all(color: _rule, width: .6),
                      borderRadius: const pw.BorderRadius.all(
                        pw.Radius.circular(3),
                      ),
                    ),
                    child: pw.Image(
                      pw.MemoryImage(ink.png),
                      fit: pw.BoxFit.contain,
                    ),
                  ),
                pw.Expanded(child: piece),
              ],
            ),
          )
        else
          pw.Padding(
            padding: pw.EdgeInsets.only(left: 13, bottom: gap(i)),
            child: piece,
          ),
    ];
  }

  pw.Widget heading(OutlineHeading h) {
    final style = switch (h.level) {
      2 => pw.TextStyle(
        fontSize: 14.5,
        fontWeight: pw.FontWeight.bold,
        color: _ink,
      ),
      3 => pw.TextStyle(
        fontSize: 12,
        fontWeight: pw.FontWeight.bold,
        color: _ink,
      ),
      _ => pw.TextStyle(
        fontSize: 10.8,
        fontWeight: pw.FontWeight.bold,
        color: _muted,
      ),
    };
    return pw.Header(
      level: h.level - 1,
      title: h.level == 2 || (deep && h.level == 3) ? h.text : null,
      decoration: h.level == 2
          ? const pw.BoxDecoration(
              border: pw.Border(bottom: pw.BorderSide(color: _rule, width: .6)),
            )
          : const pw.BoxDecoration(),
      margin: pw.EdgeInsets.only(
        top: h.level == 2 ? 16 : 10,
        bottom: h.level == 2 ? 8 : 5,
      ),
      padding: pw.EdgeInsets.only(bottom: h.level == 2 ? 4 : 0),
      child: pw.Text(h.text, style: style),
    );
  }

  List<pw.Widget> blocks(List<OutlineBlock> list, {bool appendix = false}) => [
    for (final b in list)
      ...switch (b) {
        // A heading never ends a page: it moves on with what follows it.
        OutlineHeading() => [
          pw.NewPage(freeSpace: b.level == 2 ? 96 : 64),
          heading(b),
        ],
        OutlineRemark(:final text) => [
          pw.Padding(
            padding: const pw.EdgeInsets.only(bottom: 6),
            child: pw.Text(
              text,
              style: base.copyWith(color: _muted, fontSize: 9.5),
            ),
          ),
        ],
        OutlineEntry() => entry(b, appendix: appendix),
      },
  ];

  final items = doc.body.whereType<OutlineEntry>().length;
  final sections = doc.body
      .whereType<OutlineHeading>()
      .where((h) => h.level == 2)
      .length;
  final day = date ?? DateTime.now();
  final cover = [
    pw.Text(
      doc.title,
      style: pw.TextStyle(
        fontSize: 21,
        fontWeight: pw.FontWeight.bold,
        color: _ink,
        lineSpacing: 3,
      ),
    ),
    pw.SizedBox(height: 6),
    pw.Text(
      l10n.outlinePdfMeta(
        '${day.year}-${day.month.toString().padLeft(2, '0')}-${day.day.toString().padLeft(2, '0')}',
        sections,
        items,
        doc.sources.length,
      ),
      style: base.copyWith(color: _muted, fontSize: 9),
    ),
    pw.SizedBox(height: 14),
  ];
  if (toc) {
    pdf.addPage(
      pw.Page(
        pageTheme: pageTheme,
        build: (_) => pw.Column(
          crossAxisAlignment: pw.CrossAxisAlignment.start,
          children: [
            ...cover,
            pw.SizedBox(height: 10),
            pw.Text(
              l10n.outlineContents,
              style: pw.TextStyle(
                fontSize: 11,
                fontWeight: pw.FontWeight.bold,
                color: _muted,
              ),
            ),
            pw.SizedBox(height: 8),
            pw.DefaultTextStyle(
              style: base.copyWith(fontSize: 10),
              child: pw.TableOfContent(),
            ),
          ],
        ),
      ),
    );
  }
  pdf.addPage(
    pw.MultiPage(
      pageTheme: pageTheme,
      maxPages: 400,
      footer: (c) => pw.Container(
        margin: const pw.EdgeInsets.only(top: 10),
        padding: const pw.EdgeInsets.only(top: 4),
        decoration: const pw.BoxDecoration(
          border: pw.Border(top: pw.BorderSide(color: _rule, width: .5)),
        ),
        child: pw.Row(
          mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
          children: [
            pw.Text(
              doc.title,
              style: base.copyWith(fontSize: 7.5, color: _muted),
            ),
            pw.Text(
              '${c.pageNumber} / ${c.pagesCount}',
              style: base.copyWith(fontSize: 7.5, color: _muted),
            ),
          ],
        ),
      ),
      build: (_) => [
        if (!toc) ...cover,
        ...blocks(doc.body),
        ...blocks(doc.appendix, appendix: true),
        if (doc.sources.isNotEmpty) ...[
          pw.NewPage(freeSpace: 96),
          heading(OutlineHeading(2, l10n.outlineSources)),
          for (final s in doc.sources)
            pw.Anchor(
              name: 'source-${s.number}',
              child: pw.Padding(
                padding: const pw.EdgeInsets.only(bottom: 6),
                child: pw.Row(
                  crossAxisAlignment: pw.CrossAxisAlignment.start,
                  children: [
                    pw.SizedBox(
                      width: 18,
                      child: pw.Text(
                        '${s.number}',
                        style: base.copyWith(
                          fontSize: 9,
                          color: _accent,
                          fontWeight: pw.FontWeight.bold,
                        ),
                      ),
                    ),
                    pw.Expanded(
                      child: pw.Column(
                        crossAxisAlignment: pw.CrossAxisAlignment.start,
                        children: [
                          pw.RichText(
                            text: pw.TextSpan(
                              style: base.copyWith(fontSize: 9.5),
                              children: [
                                pw.TextSpan(
                                  text: [
                                    s.label,
                                    s.locator,
                                  ].where((v) => v.isNotEmpty).join(' · '),
                                ),
                                if (s.stale)
                                  pw.TextSpan(
                                    text: '  ${l10n.outlineSourceStale}',
                                    style: pw.TextStyle(
                                      color: _amber,
                                      fontWeight: pw.FontWeight.bold,
                                      fontSize: 8.5,
                                    ),
                                  ),
                              ],
                            ),
                          ),
                          if (s.quote.isNotEmpty)
                            pw.Text(
                              '“${s.quote}”',
                              style: base.copyWith(
                                fontSize: 8.8,
                                color: _muted,
                                lineSpacing: 3,
                              ),
                            ),
                          if (s.link.isNotEmpty)
                            pw.UrlLink(
                              destination: s.link,
                              child: pw.Text(
                                l10n.outlineOpenSource,
                                style: base.copyWith(
                                  fontSize: 8.5,
                                  color: _accent,
                                ),
                              ),
                            ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ],
    ),
  );
  return pdf.save();
}
