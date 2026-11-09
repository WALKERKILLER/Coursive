import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/features/plugins/canvas_fallback.dart';

Widget host(Widget child) => MaterialApp(home: Scaffold(body: Center(child: child)));

void main() {
  testWidgets('shows the title, source and text summary of a stored object', (tester) async {
    await tester.pumpWidget(
      host(
        const CanvasFallback(
          model: CanvasFallbackModel(
            id: 'obj-1',
            type: 'review-lab.card',
            pluginId: 'review-lab',
            title: '符号解释',
            source: '课程视频 00:12–00:34；PDF 第 3–5 页',
            summary: '先展开定义，再代入。',
            revision: 2,
          ),
        ),
      ),
    );

    expect(find.text('符号解释'), findsOneWidget);
    expect(find.text('课程视频 00:12–00:34；PDF 第 3–5 页'), findsOneWidget);
    expect(find.text('先展开定义，再代入。'), findsOneWidget);
    expect(find.text('插件已不可用，以下内容仍保留在本机'), findsOneWidget);
  });

  testWidgets('exports the data the plugin left behind', (tester) async {
    Map<String, dynamic>? exported;
    await tester.pumpWidget(
      host(
        CanvasFallback(
          model: const CanvasFallbackModel(
            id: 'obj-1',
            type: 'review-lab.card',
            pluginId: 'review-lab',
            title: '符号解释',
            revision: 3,
            data: {'steps': 3},
          ),
          onExport: (data) => exported = data,
        ),
      ),
    );

    await tester.tap(find.text('导出数据'));
    expect(exported, isNotNull);
    expect(exported!['id'], 'obj-1');
    expect(exported!['pluginId'], 'review-lab');
    expect(exported!['revision'], 3);
    expect(exported!['data'], {'steps': 3});
  });

  testWidgets('hides the export action when there is no data', (tester) async {
    await tester.pumpWidget(
      host(const CanvasFallback(model: CanvasFallbackModel(id: 'obj-1', title: '符号解释'), onExport: _ignore)),
    );

    expect(find.text('导出数据'), findsNothing);
    expect(find.text('这个对象没有可显示的文字'), findsOneWidget);
  });

  testWidgets('reads daemon JSON and survives malformed fields', (tester) async {
    final model = CanvasFallbackModel.fromJson({
      'id': 'obj-1',
      'title': 42,
      'source': ['not', 'a', 'string'],
      'summary': null,
      'revision': '3',
      'data': 'not a map',
    });
    expect(model.id, 'obj-1');
    expect(model.title, '');
    expect(model.source, '');
    expect(model.revision, 0);
    expect(model.data, isEmpty);

    await tester.pumpWidget(host(CanvasFallback(model: model)));
    expect(find.text('未命名的插件对象'), findsOneWidget);
    expect(find.text('这个对象没有可显示的文字'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('empty input shows a readable state instead of a blank card', (tester) async {
    await tester.pumpWidget(host(CanvasFallback(model: CanvasFallbackModel.fromJson(null))));
    expect(find.text('无法显示这个插件对象，内容可能已损坏'), findsOneWidget);

    await tester.pumpWidget(host(CanvasFallback(model: CanvasFallbackModel.fromJson(const {}))));
    expect(find.text('无法显示这个插件对象，内容可能已损坏'), findsOneWidget);
  });

  test('the fallback model is empty only when nothing readable is left', () {
    expect(CanvasFallbackModel.fromJson(const {'revision': 4}).isEmpty, isTrue);
    expect(CanvasFallbackModel.fromJson(const {'id': 'obj-1'}).isEmpty, isFalse);
    expect(CanvasFallbackModel.fromJson(const {'data': {'steps': 3}}).isEmpty, isFalse);
  });
}

void _ignore(Map<String, dynamic> _) {}
