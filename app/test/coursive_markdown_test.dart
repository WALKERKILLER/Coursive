import 'dart:convert';

import 'package:coursive/core/api.dart';
import 'package:coursive/features/library/note_reader.dart';
import 'package:coursive/widgets/md_reader.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/widgets/coursive_markdown.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:val_latex_flutter/val_latex_flutter.dart';

// Typical model-written note: inline and display math in paragraphs, lists
// and tables, bare environments, padded dollars, timestamps, code.
const note = r'''
### 静压强特性

- 斜面 $ABC$ 面积 $S$，三个坐标面面积 $S_x,\ S_y,\ S_z$ [01:35]：
  $$S_x = S\cos\alpha,\quad S_y = S\cos\beta$$
- 代入 $V=\tfrac13 S_x\,dx$，取极限 $dx\to 0$ 得 $p_x = p_n$。

速度 $ v = \frac{dx}{dt} $ 与区间 $[0:10]$ 无关，见 [12:03]–[13:40]。

$$\rho f_x - \frac{\partial p}{\partial x} = 0,\qquad
\rho f_y - \frac{\partial p}{\partial y} = 0$$

\begin{aligned}
dp &= \rho\left(f_x dx + f_y dy + f_z dz\right) \\
dW &= f_x dx + f_y dy + f_z dz
\end{aligned}

梯度 \( \nabla p \) 与 \[ \vec f = \dfrac{1}{\rho}\nabla p \]

| 量 | 公式 |
|---|---|
| 压强 | $p=F/S$ |

**结论：$p=p(x,y,z)$**，价格 $5 和 $10 不是公式。

```python
cost = "$5"  # [00:10] stays code
```
''';

void main() {
  testWidgets('courseware reader omits registered external text actions', (t) async {
    final messenger = t.binding.defaultBinaryMessenger;
    messenger.setMockMethodCallHandler(SystemChannels.processText, (_) async => {
      'example.lookup': '外部查词',
    });
    addTearDown(() => messenger.setMockMethodCallHandler(SystemChannels.processText, null));
    await t.pumpWidget(MaterialApp(
      home: Scaffold(body: MarkdownReader(doc: MdDoc('context switch'), controller: MdReaderController())),
    ));
    await t.pumpAndSettle();
    await t.longPress(find.text('context switch',findRichText:true));
    await t.pumpAndSettle();
    final selectable = t.state<SelectableRegionState>(find.byType(SelectableRegion));
    expect(selectable.contextMenuButtonItems.any((item)=>item.label == '外部查词'),isTrue);
    final toolbar = t.widget<AdaptiveTextSelectionToolbar>(find.byType(AdaptiveTextSelectionToolbar));
    expect(toolbar.buttonItems!.map((item)=>item.type), contains(ContextMenuButtonType.copy));
    expect(toolbar.buttonItems!.map((item)=>item.label),isNot(contains('外部查词')));
  }, variant: const TargetPlatformVariant({TargetPlatform.android}));

  test(
    'courseware keeps UTF-8 when the file server reports binary content',
    () async {
      const name = '目录/系统 调用.pdf';
      const markdown = '## 系统调用\n\n用户态 → 内核态';
      await http.runWithClient(
        () async {
          final api = Api('http://localhost', '');
          final (file, body) = await loadParsedCourseware(api, 'L', name);
          expect(file.path, name);
          expect(body, markdown);
        },
        () => MockClient((req) async {
          if (req.url.path.endsWith('/courseware')) {
            return http.Response(
              jsonEncode([
                {
                  'path': name,
                  'name': '系统 调用.pdf',
                  'parse': {'state': 'done', 'md': '目录/系统 调用.md'},
                },
              ]),
              200,
              headers: {'content-type': 'application/json; charset=utf-8'},
            );
          }
          expect(req.url.pathSegments, ['files', '目录', '系统 调用.md']);
          return http.Response.bytes(
            utf8.encode(markdown),
            200,
            headers: {'content-type': 'application/octet-stream'},
          );
        }),
      );
    },
  );
  test('prepareMarkdown repairs math and links timestamps outside math and code', () {
    final out = prepareMarkdown(note);
    expect(out, contains('\$v = \\frac{dx}{dt}\$'));
    expect(out, contains('\$\$\n\\begin{aligned}'));
    expect(out, contains('[01:35](coursive://seek?t=95)'));
    expect(out, contains('[12:03–13:40](coursive://seek?t=723&end=820)'));
    expect(out, contains(r'$[0:10]$'));
    expect(out, contains('# [00:10] stays code'));
    expect(prepareMarkdown(r'价格 $ 5 + $ 10'), r'价格 $ 5 + $ 10');
  });

  testWidgets('every formula renders as parsed math', (t) async {
    await t.pumpWidget(MaterialApp(
      home: Scaffold(body: SingleChildScrollView(child: CoursiveMarkdown(note, onRef: (_) {}))),
    ));
    await t.pumpAndSettle();
    final maths = t.widgetList<Math>(find.byType(Math)).toList();
    final errors = [for (final m in maths) ...parseLatex(m.tex).diagnostics.map((d) => '${m.tex}: $d')];
    final text = t.widgetList<RichText>(find.byType(RichText)).map((r) => r.text.toPlainText()).join('\n');
    expect(errors, isEmpty);
    // List: ABC, S, S_x…, display, tfrac, dx→0, p_x (7); v, [0:10], display
    // block, aligned, \( \), \[ \], table, bold (8).
    expect(maths.length, 15);
    final prose = text.replaceAll('价格 \$5 和 \$10', '').replaceAll('"\$5"', '');
    expect(prose.contains(r'$'), isFalse, reason: text);
    expect(text.contains(r'\begin'), isFalse);
  });

  test('lecture and library-file links parse as references', () {
    expect((CoursiveRef.parse('coursive://lecture/L1') as LectureRef).lectureId, 'L1');
    expect(CoursiveRef.parse('coursive://lecture/L1?t=30'), isA<TimeRef>());
    expect((CoursiveRef.parse('coursive://file/课程/a%20b.png') as FileRef).path, '课程/a b.png');
    expect(CoursiveRef.parse('coursive://nothing/x'), isNull);
    expect(CoursiveRef.parse('https://example.com'), isNull);
  });

  testWidgets('every link in a message responds to a tap', (t) async {
    final refs = <CoursiveRef>[];
    await t.pumpWidget(MaterialApp(
      home: Scaffold(
        body: Column(children: [
          CoursiveMarkdown('打开 [第 3 节](coursive://lecture/L3)，插图 [图 1](coursive://file/a/b.png)。', onRef: refs.add),
          CoursiveMarkdown('[未知链接未知链接](coursive://nothing/x)', onRef: refs.add),
        ]),
      ),
    ));
    await t.tap(find.text('第 3 节'));
    await t.tap(find.text('图 1'));
    expect(refs.map((r) => r.runtimeType), [LectureRef, FileRef]);
    // A link the app cannot open says so instead of doing nothing.
    await t.tap(find.textContaining('未知链接', findRichText: true));
    await t.pump();
    expect(find.text('这个链接无法打开'), findsOneWidget);
  });

  testWidgets('the section reader handles encoded and repeated headings', (t) async {
    final controller = MdReaderController();
    final fragment = Uri.encodeComponent('分页-1');
    final doc = MdDoc('## 目录\n\n[去第二个分页](#$fragment)\n\n'
        '## 分页\n\n${List.filled(25, '前一节的正文。').join('\n\n')}\n\n'
        '### **分页**\n\n第二个标题的正文。\n\n${List.filled(25, '后一节的正文。').join('\n\n')}');
    await t.pumpWidget(MaterialApp(
      home: Scaffold(body: MarkdownReader(doc: doc, controller: controller)),
    ));
    await t.tap(find.text('去第二个分页', findRichText: true));
    await t.pumpAndSettle();
    expect(find.text('第二个标题的正文。', findRichText: true).hitTestable(), findsOneWidget);
  });

  testWidgets('far headings stay reachable across uneven chapter heights', (t) async {
    final controller = MdReaderController();
    final md = StringBuffer();
    for (var i = 0; i < 80; i++) {
      md.writeln('## 章节 $i');
      md.writeln(i == 32 ? List.filled(200, '较长的一章。').join('\n\n') : '正文 $i');
    }
    final doc = MdDoc(md.toString());
    final saved = <Map<String, dynamic>>[];
    await t.pumpWidget(MaterialApp(
      home: Scaffold(body: MarkdownReader(doc: doc, controller: controller, onBookmark: saved.add)),
    ));
    for (final target in [55, 19, 64, 79, 1, 32, 36, 62]) {
      saved.clear();
      controller.jumpToSection(target);
      await t.pumpAndSettle();
      expect(find.text('章节 $target', findRichText: true).hitTestable(),
          findsOneWidget, reason: 'jump to section $target');
      expect(saved, hasLength(1));
      if (target == 79) expect(controller.current, target);
    }
    controller.jumpToSection(3);
    controller.jumpToSection(48);
    await t.pumpAndSettle();
    expect(find.text('章节 48', findRichText: true).hitTestable(), findsOneWidget);
    await t.pumpWidget(const SizedBox());
    controller.dispose();
  });

  testWidgets('wheel input takes over a pending search jump', (t) async {
    final controller = MdReaderController();
    final doc = MdDoc(List.generate(100, (i) => '## Part $i\n\nBody $i').join('\n\n'));
    await t.pumpWidget(MaterialApp(home: Scaffold(body: MarkdownReader(doc: doc, controller: controller))));
    final pos = t.state<ScrollableState>(find.byType(Scrollable).first).position;
    expect(controller.reveal('Body 90'), isTrue);
    await t.sendEventToBinding(const PointerScrollEvent(
      position: Offset(200, 200), scrollDelta: Offset(0, -160),
    ));
    final userOffset = pos.pixels;
    await t.pumpAndSettle();
    expect(pos.pixels, closeTo(userOffset, 1));
    await t.pump(const Duration(seconds: 2));
    expect(pos.pixels, closeTo(userOffset, 1));
    await t.pumpWidget(const SizedBox());
    controller.dispose();
  });

  testWidgets('missing local headings do not invoke the external link route', (t) async {
    await t.pumpWidget(const MaterialApp(home: Scaffold(body: SingleChildScrollView(
      child: CoursiveMarkdown('[找不到](#missing)\n\n## 标题\n\n正文'),
    ))));
    await t.tap(find.text('找不到', findRichText: true));
    await t.pumpAndSettle();
    expect(find.text('找不到链接指向的标题'), findsOneWidget);
    expect(find.text('这个链接无法打开'), findsNothing);
  });

  test('coursive-app:// links from exported notes parse like coursive://', () {
    final ref = CoursiveRef.parse('coursive-app://lecture/L1?t=723');
    expect(ref, isA<TimeRef>());
    expect((ref as TimeRef).lectureId, 'L1');
    expect(ref.seconds, 723);
    expect(CoursiveRef.parse('https://example.com/?t=1'), isNull);
  });
}
