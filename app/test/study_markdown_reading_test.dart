import 'dart:ui' as ui;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:gpt_markdown/gpt_markdown.dart' show LinkTextSpan;

import '../lib/features/study_canvas/outline_document.dart';
import '../lib/features/study_canvas/study_document.dart';
import '../lib/widgets/coursive_markdown.dart';
import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show close, sheet;
import 'study_pdf_column_test.dart' as pdf;

const line = 'process and process';
StudyCard imported([String source = 'assets/source.MARKDOWN']) => StudyCard(
  id: 'md',
  type: 'text',
  layer: 'main',
  source: source,
  title: '复习资料',
  rect: const Rect.fromLTWH(240, 200, 530, 400),
  text:
      '# CPU\n\n**process** and process\n\n${List.filled(18, '阅读下一段').join('\n\n')}',
);

Future<void> selectWord(WidgetTester t, int from, int to) async {
  final paragraph = t.renderObject<RenderParagraph>(
    find
        .byWidgetPredicate((w) => w is RichText && w.text.toPlainText() == line)
        .first,
  );
  final rect = paragraph.getBoxesForSelection(
    TextSelection(baseOffset: from, extentOffset: to),
  );
  final start = paragraph.localToGlobal(
    rect.first.toRect().centerLeft + const Offset(.2, 0),
  );
  final end = paragraph.localToGlobal(
    rect.last.toRect().centerRight - const Offset(.2, 0),
  );
  final pointer = await t.startGesture(start, kind: ui.PointerDeviceKind.mouse);
  await pointer.moveTo(Offset.lerp(start, end, .5)!);
  await t.pump();
  await pointer.moveTo(end);
  await pointer.up();
  await t.pumpAndSettle();
}

class _AsideApi extends fixture.FixtureApi {
  _AsideApi(super.doc);
  String? answer;

  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> query) async {
    final result = await super.studyContext(id, query) as Map;
    if ((query['objects'] as List?)?.contains('md') == true) {
      (result['sources'] as List).single['anchor'] = {'object_id': 'md'};
    }
    return result;
  }

  @override
  Future<dynamic> canvasRun(String id) async => {
    'state': 'done',
    'answer': answer ?? '旁问回答：${request!['question']}',
    'context_key': 'test',
    'schema_version': 2,
    'evidence': [],
  };
}

Finder annotationInput() => find.descendant(
  of: find.byType(TextFormField),
  matching: find.byType(TextField),
);
void main() {
  test('recognizes daemon asset suffix even after a title change', () {
    expect(imported('assets/source.md').readerCard, isTrue);
    expect(StudyCard.fromJson(imported().json()).markdownReader, isTrue);
    expect(imported('assets/source.txt').markdownReader, isFalse);
  });

  testWidgets('each new aside answer opens at its own first paragraph', (
    t,
  ) async {
    final api = _AsideApi((sheet()..cards.add(imported())).json());
    final key = await fixture.open(t, api);
    await selectWord(t, 0, 7);
    await t.tap(find.text('旁问'));
    await t.pumpAndSettle();

    final doc = key.currentState!.document!;
    final aside = doc.cards.last;
    final camera = doc.camera, zoom = doc.zoom;
    final frame = find.byKey(ValueKey('study-card/${aside.id}'));
    final messages = find.descendant(
      of: frame,
      matching: find.byWidgetPredicate(
        (widget) =>
            widget is Scrollable && widget.axisDirection == AxisDirection.up,
      ),
    );
    final input = find.descendant(of: frame, matching: find.byType(TextField));
    final send = find.descendant(of: frame, matching: find.byTooltip('发送'));

    for (final (question, opening) in [
      ('什么是 PCB？', '第一则回答的开头。'),
      ('它和上下文切换有什么关系？', '追问回答的开头。'),
    ]) {
      api.answer = [
        opening,
        for (var i = 1; i <= 40; i++) '用于模拟较长回复的第 $i 段。',
      ].join('\n\n');
      await t.enterText(input, question);
      await t.tap(send);
      await t.pumpAndSettle();

      final top = t.getTopLeft(messages).dy;
      final paragraph = find.descendant(
        of: frame,
        matching: find.text(opening, findRichText: true),
      );
      expect(t.getTopLeft(paragraph).dy - top, inInclusiveRange(0, 24));
      expect(doc.camera, camera);
      expect(doc.zoom, zoom);
    }
    await close(t);
  });

  testWidgets('a contents link scrolls only its own Markdown canvas card', (t) async {
    const heading = '9. 按需分页、缺页异常与页面置换';
    final text = '# 操作系统\n\n[去第九节](#9-按需分页缺页异常与页面置换)'
        '\n\n${List.filled(35, '前面章节的内容。').join('\n\n')}'
        '\n\n## $heading\n\n${List.filled(18, '分页的解释。').join('\n\n')}';
    final duplicate = StudyCard(
      id: 'other',
      type: 'text',
      layer: 'main',
      source: 'assets/other.md',
      title: '另一份讲义',
      rect: const Rect.fromLTWH(810, 200, 530, 400),
      text: text,
    );
    final api = fixture.FixtureApi(
      (sheet()..cards.addAll([imported()..text = text, duplicate])).json(),
    );
    final key = await fixture.open(t, api);
    final doc = key.currentState!.document!;
    final camera = doc.camera, zoom = doc.zoom;
    final source = find.byKey(const ValueKey('study-card/md'));
    await t.tap(find.descendant(
      of: source,
      matching: find.text('去第九节', findRichText: true),
    ));
    await t.pumpAndSettle();
    final cardScroll = find.descendant(of: source, matching: find.byType(Scrollable)).first;
    expect(find.text('找不到链接指向的标题'), findsNothing);
    expect(t.state<ScrollableState>(cardScroll).position.pixels, greaterThan(0));
    expect(find.descendant(
      of: source,
      matching: find.text(heading, findRichText: true).hitTestable(),
    ), findsOneWidget);
    expect(find.descendant(
      of: find.byKey(const ValueKey('study-card/other')),
      matching: find.text('去第九节', findRichText: true).hitTestable(),
    ), findsOneWidget);
    expect(doc.camera, camera);
    expect(doc.zoom, zoom);
    expect(doc.edit, 0);
    expect(find.text('这个链接无法打开'), findsNothing);
    await close(t);
  });

  testWidgets(
    'the selected Markdown corner reflows text and independently grows the viewport',
    (t) async {
      final card = imported()..text = List.filled(70, 'lecture').join(' ');
      final api = fixture.FixtureApi((sheet()..cards.add(card)).json());
      final key = await fixture.open(t, api);
      final current = key.currentState!.document!.cards.single;
      final input = t.getTopLeft(find.byKey(const ValueKey('study-input')));
      await t.tapAt(input + current.rect.topLeft + const Offset(90, 18));
      await t.pumpAndSettle();
      final grip = find.byKey(const ValueKey('study-resize-handle-2'));
      RenderParagraph paragraph() => t.renderObject<RenderParagraph>(
        find
            .byWidgetPredicate(
              (w) =>
                  w is RichText &&
                  w.text.toPlainText().contains('lecture lecture'),
            )
            .first,
      );
      final previousHeight = paragraph().size.height, original = current.rect;
      await t.drag(grip, const Offset(160, 0));
      await t.pumpAndSettle();
      expect(current.rect.width, closeTo(original.width + 160, .01));
      expect(current.rect.height, original.height);
      expect(paragraph().size.height, lessThan(previousHeight));
      await t.drag(grip, const Offset(0, 80));
      await t.pumpAndSettle();
      expect(current.rect.height, closeTo(original.height + 80, .01));
      expect(current.rect.width, closeTo(original.width + 160, .01));
      expect(current.contentScale, 1);
      await key.currentState!.flush();
      expect(StudyDocument.fromJson(api.doc).cards.single.rect, current.rect);
      await close(t);
    },
  );

  testWidgets('a different annotation on each occurrence survives reopening', (
    t,
  ) async {
    final api = _AsideApi((sheet()..cards.add(imported())).json());
    final key = await fixture.open(t, api);
    final d = key.currentState!.document!;
    expect(find.byType(CoursiveMarkdown), findsOneWidget);
    Future<void> annotate(int from, String value) async {
      await selectWord(t, from, from + 7);
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      await t.enterText(annotationInput(), value);
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
    }

    await annotate(0, '进程');
    await annotate(12, '处理');
    final notes = d.cards.where((c) => c.generation == 'markdown-annotation');
    expect(notes.map((n) => n.text), ['进程', '处理']);
    expect(notes.map((n) => n.sources.single.id).toSet(), hasLength(2));
    expect(notes.every((n) => n.sources.single.quote == 'process'), isTrue);
    final raw = d.cards.first.text;
    expect(notes.map((n) => n.sources.single.start), [
      raw.indexOf('process'),
      raw.lastIndexOf('process'),
    ]);
    expect(raw, imported().text);
    await selectWord(t, 12, 19);
    await t.tap(find.text('编辑 Markdown'));
    await t.pumpAndSettle();
    final editor = find.descendant(
      of: find.byType(AlertDialog),
      matching: find.byType(TextField),
    );
    final control = t.widget<TextField>(editor).controller!;
    expect(control.selection.textInside(control.text), 'process');
    await t.enterText(editor, '# 新标题\n\n$raw');
    await t.tap(find.text('保存').last);
    await t.pumpAndSettle();
    expect(d.cards.first.text, startsWith('# 新标题'));
    expect(
      notes.last.sources.single.start,
      d.cards.first.text.lastIndexOf('process'),
    );
    expect(await key.currentState!.flush(), isTrue);
    await close(t);
    final reopened = await fixture.open(t, api);
    await selectWord(t, 12, 19);
    await t.tap(find.text('编辑注释 · 处理'));
    await t.pumpAndSettle();
    final field = t.widget<TextField>(annotationInput());
    expect(field.controller!.text, '处理');
    await t.tap(find.byTooltip('保存'));
    await t.pumpAndSettle();
    await selectWord(t, 0, 7);
    await t.tap(find.text('旁问'));
    await t.pumpAndSettle();
    expect(
      t.widget<TextField>(find.byType(TextField).last).controller!.text,
      isEmpty,
    );
    expect(
      api.request,
      isNull,
      reason: 'opening an aside must not send a draft',
    );
    final aside = reopened.currentState!.document!.cards.last;
    expect(aside.passageAside, isTrue);
    expect(
      aside.sources.single.start,
      raw.indexOf('process') + '# 新标题\n\n'.length,
    );
    final frame = find.byKey(ValueKey('study-card/${aside.id}'));
    expect(frame, findsOneWidget);
    expect(
      find.descendant(of: frame, matching: find.byTooltip('旁问')),
      findsOneWidget,
    );
    expect(aside.rect.height, lessThan(200));
    expect(aside.expandedH, 0);
    expect(
      find.descendant(of: frame, matching: find.text('所选原文')),
      findsNothing,
    );
    expect(
      find.descendant(of: frame, matching: find.text('全课程')),
      findsNothing,
    );
    expect(
      find.descendant(of: frame, matching: find.byTooltip('选择参考资料')),
      findsNothing,
    );
    expect(
      find.byKey(const ValueKey('study-ai-close')).hitTestable(),
      findsNothing,
    );
    final input = find.descendant(of: frame, matching: find.byType(TextField));
    await t.enterText(input, '进程是什么？');
    await t.tap(find.descendant(of: frame, matching: find.byTooltip('发送')));
    await t.pumpAndSettle();
    final questionTitle = find.descendant(
      of: frame,
      matching: find.byTooltip('旁问 · 进程是什么？'),
    );
    expect(questionTitle, findsOneWidget);
    expect(api.request?['conversation_id'], aside.conversationId);
    expect(api.request?['question'], '进程是什么？');
    expect(api.request?['context_scope'], 'course');
    expect(find.byKey(const ValueKey('study-coverage')), findsNothing);
    expect(
      find.byKey(ValueKey('study-pin-answer-${api.request!['turn_id']}')),
      findsOneWidget,
    );
    expect(api.request?['selection']['selection'], {
      'object_id': 'md',
      'start': aside.sources.single.start,
      'end': aside.sources.single.end,
    });
    expect(
      api.conversations[aside.conversationId]?['turns'].single['answer'],
      '旁问回答：进程是什么？',
    );
    await t.enterText(input, '我的旁问草稿');
    await t.pump(const Duration(seconds: 1));
    final before = aside.rect;
    await t.dragFrom(
      t.getTopLeft(frame) + const Offset(90, 18),
      const Offset(36, 42),
    );
    await t.pumpAndSettle();
    expect(aside.rect, before.shift(const Offset(36, 42)));
    final expanded = aside.rect;
    await t.tap(find.descendant(of: frame, matching: find.byTooltip('收起')));
    await t.pumpAndSettle();
    expect(aside.collapsed, isTrue);
    expect(aside.rect.height, 40);
    expect(aside.rect.width, lessThan(expanded.width));
    expect(aside.rect.topLeft, expanded.topLeft);
    expect(input, findsNothing);
    expect(questionTitle, findsOneWidget);
    expect(await reopened.currentState!.flush(), isTrue);
    await close(t);
    api.conversations[aside.conversationId]!['query'] = {
      'objects': <String>[],
      'selection': <String, dynamic>{},
    };
    final again = await fixture.open(t, api);
    final restored = again.currentState!.document!.cards.last;
    expect(restored.rect, aside.rect);
    expect(restored.conversationId, aside.conversationId);
    expect(restored.collapsed, isTrue);
    expect(questionTitle, findsOneWidget);
    await t.tap(find.descendant(of: frame, matching: find.byTooltip('返回原文')));
    await t.pumpAndSettle();
    await selectWord(t, 0, 7);
    await t.tap(find.text('打开旁问 1'));
    await t.pumpAndSettle();
    expect(restored.collapsed, isFalse);
    expect(restored.rect, expanded);
    expect(t.widget<TextField>(input).controller!.text, '我的旁问草稿');
    await t.tap(find.descendant(of: frame, matching: find.byTooltip('更多')));
    await t.pumpAndSettle();
    expect(find.byIcon(Icons.check_box_outlined), findsOneWidget);
    await t.tap(find.text('全课程'));
    await t.pumpAndSettle();
    expect(
      again.currentState!.document!.cards.where((c) => c.passageAside),
      hasLength(1),
    );
    await t.tap(find.descendant(of: frame, matching: find.byTooltip('发送')));
    await t.pumpAndSettle();
    expect(api.request?['question'], '我的旁问草稿');
    expect(api.request?['context_scope'], 'selected');
    expect(
      api.request?['selection']['selection']['start'],
      aside.sources.single.start,
    );
    expect(api.conversations[aside.conversationId]?['turns'], hasLength(2));
    expect(questionTitle, findsOneWidget);
    await close(t);
  });

  testWidgets('a long aside fits up to its scrollable height limit', (t) async {
    final source = imported();
    final aside = StudyCard(
      id: 'aside',
      type: 'conversation',
      layer: 'main',
      generation: 'markdown-aside',
      conversationId: 'aside',
      title: '旁问 · process',
      rect: const Rect.fromLTWH(820, 200, 420, 160),
      sources: [
        StudyAnchor(
          id: 'md-0-5-12',
          objectId: source.id,
          kind: 'text',
          quote: 'process',
        ),
      ],
    );
    final api = _AsideApi((sheet()..cards.addAll([source, aside])).json());
    api.conversations[aside.conversationId] = {
      'revision': 1,
      'query': {
        'objects': [source.id],
      },
      'turns': [
        {
          'id': 'long',
          'state': 'done',
          'question': '「process」\n为什么？',
          'answer': List.generate(40, (i) => '第 $i 段完整解释。').join('\n\n'),
        },
      ],
    };
    final key = await fixture.open(t, api);
    final card = key.currentState!.document!.cards.last;
    final frame = find.byKey(const ValueKey('study-card/aside'));
    expect(find.byTooltip('旁问 · 为什么？'), findsOneWidget);
    expect(find.byTooltip('旁问 · process'), findsNothing);
    final list = find
        .descendant(of: frame, matching: find.byType(ListView))
        .first;
    final scroll = t.widget<ListView>(list).controller!;
    expect(card.rect.height, 440);
    expect(scroll.position.maxScrollExtent, greaterThan(0));
    expect(
      find.descendant(of: frame, matching: find.byTooltip('发送')).hitTestable(),
      findsOneWidget,
    );
    expect(key.currentState!.document!.canUndo, isFalse);
    await close(t);
  });

  testWidgets(
    'annotation header opens a fresh AI conversation without sending',
    (t) async {
      final handout = imported();
      final firstWord = handout.text.indexOf('process');
      final note = StudyCard(
        id: 'annotation',
        type: 'text',
        layer: 'main',
        generation: 'markdown-annotation',
        title: '进程笔记',
        text: '进程',
        rect: const Rect.fromLTWH(240, 100, 410, 350),
        sources: [
          StudyAnchor(
            id: 'source-range',
            objectId: handout.id,
            kind: 'text',
            start: firstWord,
            end: firstWord + 7,
            quote: 'process',
          ),
        ],
      );
      final api = _AsideApi((sheet()..cards.addAll([handout, note])).json());
      final key = await fixture.open(t, api);
      final frame = find.byKey(const ValueKey('study-card/annotation'));
      Finder inCard(Finder matching) =>
          find.descendant(of: frame, matching: matching);
      for (final label in ['返回原文', '收起', '更多']) {
        expect(inCard(find.byTooltip(label)), findsOneWidget);
      }
      expect(inCard(find.byIcon(Icons.notes_rounded)), findsNothing);
      expect(inCard(find.byIcon(Icons.drag_indicator_rounded)), findsNothing);

      final d = key.currentState!.document!;
      final card = d.cards.singleWhere((c) => c.passageNote);
      expect(inCard(find.byTooltip('编辑注释')), findsNothing);
      expect(inCard(find.byTooltip('新对话')), findsNothing);
      expect(card.rect.height, lessThan(120));
      final width = card.rect.width;
      await t.tap(inCard(find.byTooltip('收起')));
      await t.pumpAndSettle();
      expect(card.collapsed, isTrue);
      expect(card.rect.width, lessThan(width));
      expect(card.rect.height, lessThan(50));
      final title = inCard(find.byWidgetPredicate(
        (widget) => widget is RichText && widget.text.toPlainText() == '进程笔记',
      ));
      expect(t.renderObject<RenderParagraph>(title).didExceedMaxLines, isFalse);
      await t.tap(inCard(find.byTooltip('展开模块')));
      await t.pumpAndSettle();
      expect(card.rect.width, width);
      expect(card.rect.height, lessThan(120));
      await t.tap(inCard(find.byTooltip('更多')));
      await t.pumpAndSettle();
      await t.tap(find.text('编辑注释'));
      await t.pumpAndSettle();
      await t.enterText(annotationInput(), List.filled(24, '多行注释').join('\n'));
      await t.pumpAndSettle();
      expect(card.rect.height, inInclusiveRange(300, 440));
      await t.enterText(annotationInput(), '短注释');
      await t.tap(inCard(find.byTooltip('保存')));
      await t.pumpAndSettle();
      expect(card.rect.height, lessThan(120));
      expect(card.text, '短注释');
      for (var i = 0; i < 2; i++) {
        await t.tap(inCard(find.byTooltip('更多')));
        await t.pumpAndSettle();
        await t.tap(find.text('新对话'));
        await t.pumpAndSettle();
      }
      expect(
        d.cards
            .where((c) => c.type == 'conversation')
            .map((c) => c.conversationId)
            .toSet(),
        hasLength(2),
      );
      expect(api.request, isNull);
      await close(t);
    },
  );

  testWidgets(
    'dashed Markdown links keep their action and ordinary strike-through',
    (t) async {
      final handout = imported()..text = '~~删去~~ [资料](https://example.test/)';
      await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(handout)).json()),
      );
      final paragraph = t.renderObject<RenderParagraph>(
        find
            .byWidgetPredicate(
              (w) => w is RichText && w.text.toPlainText() == '删去 资料',
            )
            .first,
      );
      final spans = <TextSpan>[];
      paragraph.text.visitChildren((span) {
        if (span is TextSpan) spans.add(span);
        return true;
      });
      final strike = spans.firstWhere((span) => span.text == '删去');
      expect(strike.style?.decoration, TextDecoration.lineThrough);
      expect(strike.style?.decorationStyle, isNot(TextDecorationStyle.dashed));
      // The link container can own the decoration while the leaf owns text.
      LinkTextSpan? link;
      void findLink(InlineSpan span) {
        if (span is! TextSpan) return;
        if (span is LinkTextSpan) link = span;
        for (final child in span.children ?? const <InlineSpan>[]) {
          findLink(child);
        }
      }

      findLink(paragraph.text);
      expect(link?.style?.decorationStyle, TextDecorationStyle.dashed);
      expect(link?.onTap, isNotNull);
      await close(t);
    },
  );

  testWidgets(
    'saved marks follow each occurrence when Markdown wraps and a note is removed',
    (t) async {
      final handout = imported();
      handout.text += '\n\n[回到标题](#cpu)';
      final rawStart = handout.text.indexOf('process');
      final fingerprint = outlineNoteHash(handout.text);
      StudyCard mark(String id, int offset, bool note) => StudyCard(
        id: id,
        type: note ? 'text' : 'conversation',
        layer: 'main',
        generation: note ? 'markdown-annotation' : 'markdown-aside',
        conversationId: note ? '' : id,
        rect: Rect.fromLTWH(note ? 800 : 1150, 200, 320, 200),
        sources: [
          StudyAnchor(
            id: 'md-0-$offset-${offset + 7}',
            objectId: handout.id,
            kind: 'text',
            fingerprint: note ? 'source-$fingerprint' : fingerprint,
            start: note ? rawStart : 0,
            end: note ? rawStart + 7 : 0,
            quote: 'process',
          ),
        ],
      );
      // The renderer includes the heading and its following newline and space
      // in the selectable offsets.
      final api = _AsideApi(
        (sheet()
              ..cards.addAll([
                handout,
                mark('n', 5, true),
                mark('a', 17, false),
              ]))
            .json(),
      );
      final key = await fixture.open(t, api);
      final source = find.byKey(const ValueKey('markdown/md/-1'));
      final overlay = find
          .descendant(
            of: source,
            matching: find.byWidgetPredicate(
              (w) => w is CustomPaint && w.foregroundPainter != null,
            ),
          )
          .first;
      final words = find
          .descendant(
            of: source,
            matching: find.byWidgetPredicate(
              (w) => w is RichText && w.text.toPlainText() == line,
            ),
          )
          .first;
      List<Rect> boxes(
        int start,
        int end, {
        ui.BoxHeightStyle heightStyle = ui.BoxHeightStyle.tight,
      }) {
        final paragraph = t.renderObject<RenderParagraph>(words);
        final transform = paragraph.getTransformTo(t.renderObject(overlay));
        return paragraph
            .getBoxesForSelection(
              TextSelection(baseOffset: start, extentOffset: end),
              boxHeightStyle: heightStyle,
            )
            .map((box) => MatrixUtils.transformRect(transform, box.toRect()))
            .toList();
      }

      (ui.Path, ui.Path) paths() {
        final canvas = TestRecordingCanvas();
        final box = t.renderObject<RenderBox>(overlay);
        t
            .widget<CustomPaint>(overlay)
            .foregroundPainter!
            .paint(canvas, box.size);
        final paths = canvas.invocations
            .where((call) => call.invocation.memberName == #drawPath)
            .map((call) => call.invocation.positionalArguments.first as ui.Path)
            .toList();
        return (paths[0], paths[1]);
      }

      void verifyMarks({required bool hasNote}) {
        final (highlight, dash) = paths();
        final annotation = boxes(0, 7).first;
        final aside = boxes(12, 19, heightStyle: ui.BoxHeightStyle.max).first;
        expect(highlight.contains(annotation.center), hasNote);
        expect(highlight.contains(aside.center), isFalse);
        expect(dash.getBounds().left, closeTo(aside.left, .01));
        expect(dash.getBounds().top, closeTo(aside.bottom + 1, .01));
        expect(dash.computeMetrics().length, greaterThan(1));
      }

      verifyMarks(hasNote: true);
      final d = key.currentState!.document!;
      final sourceCard = d.cards.first;
      sourceCard.rect = Rect.fromLTWH(240, 200, 170, 400);
      d.changed();
      await t.pumpAndSettle();
      expect(boxes(12, 19).first.top, greaterThan(boxes(0, 7).first.top));
      verifyMarks(hasNote: true);
      await t.tap(find.byKey(const ValueKey('study-tool-pan')));
      await t.pumpAndSettle();
      verifyMarks(hasNote: true);
      d.cards.removeWhere((c) => c.id == 'n');
      d.changed();
      await t.pumpAndSettle();
      verifyMarks(hasNote: false);
      await close(t);
    },
  );

  testWidgets('a Markdown aside draws one level dash pattern per line', (
    t,
  ) async {
    const passage = '寄存器 Register 通过 EAX 保存当前状态。';
    final handout = imported()..text = '**寄存器 Register** 通过 `EAX` 保存当前状态。';
    final aside = StudyCard(
      id: 'aside',
      type: 'conversation',
      layer: 'main',
      generation: 'markdown-aside',
      conversationId: 'aside',
      rect: const Rect.fromLTWH(820, 200, 320, 200),
      sources: [
        StudyAnchor(
          id: 'md-0-0-${passage.length}',
          objectId: handout.id,
          kind: 'text',
          fingerprint: outlineNoteHash(handout.text),
          quote: passage,
        ),
      ],
    );
    final key = await fixture.open(
      t,
      _AsideApi((sheet()..cards.addAll([handout, aside])).json()),
    );
    final source = find.byKey(const ValueKey('markdown/md/-1'));
    final overlay = find
        .descendant(
          of: source,
          matching: find.byWidgetPredicate(
            (w) => w is CustomPaint && w.foregroundPainter != null,
          ),
        )
        .first;
    final words = find
        .descendant(
          of: source,
          matching: find.byWidgetPredicate(
            (w) => w is RichText && w.text.toPlainText() == passage,
          ),
        )
        .first;
    List<Offset> dashStarts() {
      final canvas = TestRecordingCanvas();
      t
          .widget<CustomPaint>(overlay)
          .foregroundPainter!
          .paint(canvas, t.getSize(overlay));
      final drawUnderline = canvas.invocations
          .where((call) => call.invocation.memberName == #drawPath)
          .last;
      final path =
          drawUnderline.invocation.positionalArguments.first as ui.Path;
      return [
        for (final dash in path.computeMetrics())
          dash.getTangentForOffset(0)!.position,
      ];
    }

    final paragraph = t.renderObject<RenderParagraph>(words);
    final selection = TextSelection(
      baseOffset: 0,
      extentOffset: passage.length,
    );
    final tight = paragraph.getBoxesForSelection(selection);
    expect(tight.map((b) => b.bottom).toSet(), hasLength(greaterThan(1)));
    final dashes = dashStarts();
    expect(dashes.map((p) => p.dy).toSet(), hasLength(1));
    for (var i = 1; i < dashes.length; i++) {
      expect(dashes[i].dx - dashes[i - 1].dx, closeTo(7, .01));
    }

    final doc = key.currentState!.document!;
    doc.cards.first.rect = const Rect.fromLTWH(240, 200, 220, 400);
    doc.changed();
    await t.pumpAndSettle();
    final lines = paragraph
        .getBoxesForSelection(selection, boxHeightStyle: ui.BoxHeightStyle.max)
        .map((b) => b.bottom)
        .toSet();
    expect(lines, hasLength(greaterThan(1)));
    expect(dashStarts().map((p) => p.dy).toSet(), hasLength(lines.length));
    await close(t);
  });

  testWidgets(
    'returning from a note centers and highlights its specific Markdown occurrence',
    (t) async {
      final source = imported()
        ..text = [
          line,
          ...List.filled(26, '阅读下一段'),
          line,
          ...List.filled(26, '继续阅读'),
        ].join('\n\n');
      final key = await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(source)).json()),
      );
      final card = find.byKey(const ValueKey('study-card/md'));
      final scrollable = find
          .descendant(of: card, matching: find.byType(Scrollable))
          .first;
      ScrollPosition position() =>
          t.state<ScrollableState>(scrollable).position;
      final passages = find.byWidgetPredicate(
        (widget) => widget is RichText && widget.text.toPlainText() == line,
      );
      final paragraph = t.renderObject<RenderParagraph>(passages.last);
      final word = paragraph
          .getBoxesForSelection(
            const TextSelection(baseOffset: 12, extentOffset: 19),
          )
          .single
          .toRect();
      position().jumpTo(position().maxScrollExtent / 2);
      await t.pump();
      final start = paragraph.localToGlobal(
        word.centerLeft + const Offset(.2, 0),
      );
      final end = paragraph.localToGlobal(
        word.centerRight - const Offset(.2, 0),
      );
      final pointer = await t.startGesture(
        start,
        kind: ui.PointerDeviceKind.mouse,
      );
      await pointer.moveTo(Offset.lerp(start, end, .5)!);
      await t.pump();
      await pointer.moveTo(end);
      await pointer.up();
      await t.pumpAndSettle();
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
      final anchor = key.currentState!.document!.cards.last.sources.single;
      expect(anchor.start, source.text.lastIndexOf('process'));
      position().jumpTo(0);
      await t.tap(find.byKey(const ValueKey('study-tool-pan')));
      await t.pumpAndSettle();

      await t.tap(find.byTooltip('返回原文'));
      await t.pump();
      await t.pump(const Duration(milliseconds: 350));
      expect(position().pixels, greaterThan(0));
      expect(find.text('返回原处'), findsNothing);
      final paint = find
          .descendant(
            of: find.byKey(const ValueKey('markdown/md/-1')),
            matching: find.byWidgetPredicate(
              (widget) =>
                  widget is CustomPaint && widget.foregroundPainter != null,
            ),
          )
          .first;
      final layer = t.renderObject<RenderBox>(paint);
      final target = t.renderObject<RenderParagraph>(passages.last);
      final viewport = RenderAbstractViewport.of(target) as RenderBox;
      expect(
        target.localToGlobal(word.center, ancestor: viewport).dy,
        closeTo(viewport.size.height / 2, .75),
      );
      final targetRect = MatrixUtils.transformRect(
        target.getTransformTo(layer),
        word,
      );
      expect(
        paint,
        paints..rrect(
          rrect: RRect.fromRectAndRadius(
            targetRect.inflate(2),
            const Radius.circular(4),
          ),
        ),
      );
      await t.pumpAndSettle();
      await close(t);
    },
  );

  testWidgets(
    'passage selection offers only app actions and standard text actions',
    (t) async {
      final messenger = t.binding.defaultBinaryMessenger;
      messenger.setMockMethodCallHandler(SystemChannels.processText, (
        call,
      ) async {
        expect(call.method, 'ProcessText.queryTextActions');
        return {'com.example.search': '外部搜索', 'com.example.ask': '外部问答'};
      });
      addTearDown(
        () => messenger.setMockMethodCallHandler(
          SystemChannels.processText,
          null,
        ),
      );
      final key = await fixture.open(
        t,
        fixture.FixtureApi((sheet()..cards.add(imported())).json()),
      );
      final paragraph = t.renderObject<RenderParagraph>(
        find
            .byWidgetPredicate(
              (w) => w is RichText && w.text.toPlainText() == line,
            )
            .first,
      );
      final word = paragraph.getBoxesForSelection(
        const TextSelection(baseOffset: 0, extentOffset: 7),
      );
      if (defaultTargetPlatform == TargetPlatform.android) {
        await t.longPressAt(
          paragraph.localToGlobal(word.first.toRect().center),
        );
        await t.pumpAndSettle();
      } else {
        await selectWord(t, 0, 7);
      }
      final region = t.state<SelectableRegionState>(
        find.byType(SelectableRegion),
      );
      expect(
        region.contextMenuButtonItems.map((item) => item.label),
        containsAll(['外部搜索', '外部问答']),
      );
      final menu = t.widget<AdaptiveTextSelectionToolbar>(
        find.byType(AdaptiveTextSelectionToolbar),
      );
      expect(
        menu.buttonItems!
            .where((item) => item.type == ContextMenuButtonType.custom)
            .map((item) => item.label),
        ['加注释', '旁问', '编辑 Markdown'],
      );
      final expected = [
        ContextMenuButtonType.copy,
        ContextMenuButtonType.selectAll,
        if (defaultTargetPlatform == TargetPlatform.android)
          ContextMenuButtonType.share,
      ];
      expect(menu.buttonItems!.map((item) => item.type), containsAll(expected));
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      expect(find.byType(AlertDialog), findsNothing);
      expect(annotationInput(), findsOneWidget);
      expect(
        key.currentState!.document!.cards.last.sources.single.quote,
        'process',
      );
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
      await close(t);
    },
    variant: const TargetPlatformVariant({
      TargetPlatform.android,
      TargetPlatform.macOS,
      TargetPlatform.windows,
    }),
  );

  testWidgets(
    'a cross-format quotation stays readable without inventing a source offset',
    (t) async {
      final api = _AsideApi((sheet()..cards.add(imported())).json());
      final key = await fixture.open(t, api);
      await selectWord(t, 0, 11);
      await t.tap(find.text('加注释'));
      await t.pumpAndSettle();
      await t.enterText(annotationInput(), '跨格式的摘句');
      await t.tap(find.byTooltip('保存'));
      await t.pumpAndSettle();
      final a = key.currentState!.document!.cards.last.sources.single;
      expect(a.quote, 'process and');
      expect(a.start, 0);
      expect(a.end, 0);
      await selectWord(t, 0, 11);
      expect(find.text('编辑注释 · 跨格式的摘句'), findsOneWidget);
      await t.tap(find.text('旁问'));
      await t.pumpAndSettle();
      final aside = key.currentState!.document!.cards.last;
      final frame = find.byKey(ValueKey('study-card/${aside.id}'));
      final input = find.descendant(
        of: frame,
        matching: find.byType(TextField),
      );
      expect(t.widget<TextField>(input).controller!.text, isEmpty);
      expect(await key.currentState!.flush(), isTrue);
      await close(t);

      // The old generated draft is the only text we clear during migration.
      api.conversations[aside.conversationId]!['draft'] =
          '「process and」\n这段话在原文里是什么意思？';
      await fixture.open(t, api);
      expect(t.widget<TextField>(input).controller!.text, isEmpty);
      await t.enterText(input, '这是什么意思？');
      await t.tap(find.descendant(of: frame, matching: find.byTooltip('发送')));
      await t.pumpAndSettle();
      expect(api.request?['question'], '这是什么意思？');
      expect(api.request?['selected_excerpt'], {
        'object_id': a.objectId,
        'text': 'process and',
      });
      expect(
        api.conversations[aside.conversationId]?['turns'].single['question'],
        '这是什么意思？',
      );
      await close(t);
    },
  );

  for (final doc in ['markdown', 'pdf']) {
    testWidgets(
      'hand pans across $doc and arrow returns ownership to the reader',
      (t) async {
        final card = doc == 'pdf' ? pdf.pdf() : imported();
        final key = await fixture.open(
          t,
          pdf.PDFApi((sheet()..cards.add(card)).json()),
        );
        final d = key.currentState!.document!;
        final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
        final original = d.cards.single.rect;
        await t.tap(find.byKey(const ValueKey('study-tool-pan')));
        await t.pumpAndSettle();
        await t.dragFrom(origin + original.center, const Offset(65, 45));
        await t.pumpAndSettle();
        expect(d.camera, const Offset(65, 45));
        expect(d.cards.single.rect, original);
        await t.tap(find.byKey(const ValueKey('study-tool-select')));
        await t.pumpAndSettle();
        final where = origin + original.center + d.camera;
        await t.dragFrom(where, const Offset(0, -55));
        await t.pumpAndSettle();
        expect(d.camera, const Offset(65, 45));
        if (doc == 'markdown') {
          final scroller = find
              .descendant(
                of: find.byKey(const PageStorageKey('study-reader/md')),
                matching: find.byType(Scrollable),
              )
              .first;
          expect(
            t.state<ScrollableState>(scroller).position.pixels,
            greaterThan(0),
          );
        } else {
          await t.tap(find.byTooltip('下一页'));
          await t.pumpAndSettle();
          expect(d.cards.single.page, greaterThan(card.page));
        }
        expect(d.strokes, isEmpty);
        await close(t);
      },
    );
  }
}
