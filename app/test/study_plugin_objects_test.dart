import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;

import '../lib/core/app_state.dart';
import '../lib/core/daemon_service.dart';
import '../lib/core/plugins.dart';
import '../lib/core/ui_slots.dart';
import '../lib/features/plugins/plugin_actions.dart';
import '../lib/features/plugins/plugin_contributions.dart';
import '../lib/features/study_canvas/study_document.dart';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

import 'study_canvas_ui_test.dart' as fixture;
import 'study_handwriting_test.dart' show sheet, close;

/// The shared canvas-object cases the daemon validates too
/// (daemon/internal/pluginfabric/canvas_fixtures_test.go).
Map<String, dynamic> _fixtures() {
  for (final candidate in [
    '${Directory.current.path}/../daemon/internal/pluginfabric/testdata/canvas',
    '${Directory.current.path}/daemon/internal/pluginfabric/testdata/canvas',
  ]) {
    final file = File('$candidate/cases.json');
    if (file.existsSync()) {
      return (jsonDecode(file.readAsStringSync()) as Map)
          .cast<String, dynamic>();
    }
  }
  fail(
    '找不到 daemon/internal/pluginfabric/testdata/canvas：请从仓库根目录或 app/ 运行 flutter test',
  );
}

Map<String, dynamic> _apply(Map base, Map? set) {
  final out = jsonDecode(jsonEncode(base)) as Map<String, dynamic>;
  for (final e in (set ?? const {}).entries) {
    final parts = '${e.key}'.split('.');
    var at = out;
    for (final p in parts.take(parts.length - 1)) {
      at = at[p] as Map<String, dynamic>;
    }
    if (e.value == null) {
      at.remove(parts.last);
    } else {
      at[parts.last] = e.value;
    }
  }
  return out;
}

/// A valid object of [plugin] at [revision].
Map<String, dynamic> object(
  String id, {
  String plugin = 'review-lab',
  int revision = 1,
  String title = '复习卡',
  Rect rect = const Rect.fromLTWH(280, 320, 300, 120),
}) => {
  'id': id,
  'type': '$plugin.card',
  'schemaVersion': 1,
  'geometry': {
    'x': rect.left,
    'y': rect.top,
    'w': rect.width,
    'h': rect.height,
  },
  'content': {
    'title': title,
    'text': '先展开定义，再代入。',
    'data': {
      'cards': [
        {'front': 'Q1', 'back': 'A1'},
      ],
    },
  },
  'sourceAnchors': [
    {
      'kind': 'lecture',
      'lectureId': 'fixture',
      'startMs': 12000,
      'endMs': 34000,
    },
  ],
  'createdBy': {'pluginId': plugin, 'pluginVersion': '0.1.0'},
  'revision': revision,
  'interactionPolicy': {'mode': 'select'},
};

StudyDocument withObjects(List<Map<String, dynamic>> objects) =>
    StudyDocument.fromJson({
      ...sheet().json(),
      'version': 5,
      'plugin_objects': objects,
    });

void main() {
  test('the canvas validates every shared fixture as the daemon does', () {
    final f = _fixtures();
    expect(f['schema'], 'coursive/canvas-object@$canvasSchemaVersion');
    final cases = (f['cases'] as List).cast<Map>();
    expect(cases.where((c) => c['code'] == null), isNotEmpty);
    expect(cases.where((c) => c['code'] != null), isNotEmpty);
    for (final c in cases) {
      final problems = validateCanvasObject(
        _apply(f['base'] as Map, c['set'] as Map?),
      );
      if (c['code'] == null) {
        expect(problems, isEmpty, reason: '${c['name']}');
      } else {
        expect(problems, isNotEmpty, reason: '${c['name']}');
        expect(
          '${problems.first.code} at ${problems.first.path}',
          '${c['code']} at ${c['path']}',
          reason: '${c['name']}',
        );
      }
    }
  });

  test('plugin objects save in format 5 and load back unchanged', () {
    expect(sheet().json()['version'], lessThan(5));
    expect(sheet().json().containsKey('plugin_objects'), isFalse);
    final d = withObjects([object('deck-1')]);
    final saved = d.json();
    expect(saved['version'], 5);
    expect(saved['plugin_objects'], [object('deck-1')]);
    final again = StudyDocument.fromJson(jsonDecode(jsonEncode(saved)) as Map);
    expect(again.pluginObjects.single.json(), object('deck-1'));
    expect(
      () => StudyDocument.fromJson({
        ...saved,
        'plugin_objects': [
          {...object('bad'), 'revision': 0},
        ],
      }),
      throwsFormatException,
    );
  });

  test(
    'a batch commits whole as one undo entry, and undo moves the revision',
    () {
      final d = withObjects([]);
      final batch = d.checkPluginBatch('review-lab', {
        'revision': 0,
        'objects': [object('deck-1'), object('deck-2')],
        'label': '钉到画布',
      });
      expect(batch.needsApproval, isFalse);
      d.commitPluginBatch(batch);
      expect(d.pluginObjects.map((o) => o.id), ['deck-1', 'deck-2']);
      expect(d.pluginRevision, 1);
      expect(d.canUndo, isTrue);

      d.undo();
      expect(d.pluginObjects, isEmpty);
      expect(d.pluginRevision, 2);
      d.redo();
      expect(d.pluginObjects, hasLength(2));
      expect(d.pluginRevision, 3);
    },
  );

  test('a stale, invalid or foreign batch commits nothing', () {
    final d = withObjects([object('mine'), object('theirs', plugin: 'lab-two')])
      ..cards.add(
        StudyCard(id: 'card-1', type: 'note', layer: 'main', rect: Rect.zero),
      );
    final before = jsonEncode(d.json());
    void refused(Map batch, String why) {
      expect(
        () => d.commitPluginBatch(d.checkPluginBatch('review-lab', batch)),
        throwsFormatException,
        reason: why,
      );
      expect(jsonEncode(d.json()), before, reason: why);
      expect(d.canUndo, isFalse, reason: why);
    }

    refused({'revision': 0}, 'empty');
    refused({
      'revision': 7,
      'objects': [object('new')],
    }, 'stale canvas revision');
    refused({
      'revision': 0,
      'objects': [
        object('new'),
        {...object('broken'), 'type': 'card'},
      ],
    }, 'one invalid object');
    refused({
      'revision': 0,
      'objects': [object('mine', revision: 3)],
    }, 'object revision mismatch');
    refused({
      'revision': 0,
      'objects': [object('new', revision: 2)],
    }, 'a create past revision 1');
    refused({
      'revision': 0,
      'objects': [object('theirs', revision: 2)],
    }, 'another plugin object');
    refused({
      'revision': 0,
      'delete': ['theirs'],
    }, 'deleting another plugin object');
    refused({
      'revision': 0,
      'objects': [object('lab', plugin: 'lab-two')],
    }, 'writing as another plugin');
    refused({
      'revision': 0,
      'objects': [object('card-1')],
    }, 'an id the canvas already uses');
    refused({
      'revision': 0,
      'objects': [object('mine', revision: 2)],
      'delete': ['mine'],
    }, 'write and delete together');
    refused({
      'revision': 0,
      'delete': ['gone'],
    }, 'deleting what is gone');
  });

  test(
    'an overwrite or delete needs approval, and a change in between refuses it',
    () {
      final d = withObjects([object('mine')]);
      d.connections.add(StudyConnection('edge', 'mine', 'mine-2', ''));
      final overwrite = d.checkPluginBatch('review-lab', {
        'revision': 0,
        'objects': [object('mine', revision: 2, title: '新标题')],
      });
      expect(overwrite.needsApproval, isTrue);
      final delete = d.checkPluginBatch('review-lab', {
        'revision': 0,
        'delete': ['mine'],
      });
      expect(delete.needsApproval, isTrue);

      d.commitPluginBatch(overwrite);
      expect(d.pluginObjects.single.title, '新标题');
      expect(() => d.commitPluginBatch(delete), throwsFormatException);
      expect(d.pluginObjects, hasLength(1));

      d.commitPluginBatch(
        d.checkPluginBatch('review-lab', {
          'revision': d.pluginRevision,
          'delete': ['mine'],
        }),
      );
      expect(d.pluginObjects, isEmpty);
      expect(d.connections, isEmpty);
      d.undo();
      expect(d.pluginObjects.single.title, '新标题');
      expect(d.connections.single.id, 'edge');
    },
  );

  test('the fallback keeps the title, source and data, and export keeps the anchors', () {
    final o = withObjects([object('deck-1')]).pluginObjects.single;
    final model = o.fallback;
    expect(model.title, '复习卡');
    expect(model.pluginId, 'review-lab');
    expect(model.source, '课程视频 00:12–00:34');
    expect(model.summary, '先展开定义，再代入。');
    expect(model.data['cards'], [
      {'front': 'Q1', 'back': 'A1'},
    ]);
    expect(o.json()['sourceAnchors'], object('deck-1')['sourceAnchors']);
  });

  testWidgets('a stroke drawn across a plugin object stays a stroke', (
    t,
  ) async {
    final api = fixture.FixtureApi(withObjects([object('deck-1')]).json());
    final key = await fixture.open(t, api);
    final d = key.currentState!.document!;
    // No plugin store here: the object is drawn through the fallback.
    expect(find.text('插件已不可用，以下内容仍保留在本机'), findsOneWidget);
    expect(find.text('复习卡'), findsOneWidget);

    await t.tap(find.byKey(const ValueKey('study-tool-pen')));
    await t.pumpAndSettle();
    final origin = t.getTopLeft(find.byKey(const ValueKey('study-input')));
    final pen = await t.startGesture(
      origin + const Offset(300, 380),
      kind: ui.PointerDeviceKind.stylus,
    );
    for (var x = 310; x <= 520; x += 10) {
      await pen.moveTo(origin + Offset(x.toDouble(), 380));
      await t.pump(const Duration(milliseconds: 8));
    }
    await pen.up();
    await t.pumpAndSettle();
    expect(d.strokes, isNotEmpty);
    expect(d.pluginObjects.single.json(), object('deck-1'));
    await close(t);
  });

  testWidgets(
    'a canvas tool pins an object, and removing it asks nothing and undoes',
    (t) async {
      final slots = UiSlotRegistry()
        ..replaceAll([
          UiSlotContribution(
            target: 'canvas.toolbar.tools',
            id: 'pin',
            pluginId: 'review-lab',
            builder: (context, host) => FilledButton(
              onPressed: () => PluginViewPatchScope.maybeOf(context)!.onPatch(
                'review-lab',
                PluginViewPatch(
                  viewId: 'pin',
                  revision: 0,
                  patch: {
                    'canvas': {
                      'revision': host['canvas_revision'],
                      'objects': [
                        if ((host['canvas_objects'] as List).isEmpty)
                          object('deck-1')
                        else
                          object('deck-1', revision: 2, title: '换一组'),
                      ],
                    },
                  },
                ),
              ),
              child: Text('钉到画布 ${host['lecture_id']}'),
            ),
          ),
        ]);
      final store = PluginStore(AppState(DaemonService()))
        ..installed = [
          {'id': 'review-lab', 'title': '复习卡实验室', 'state': 'enabled'},
        ];
      final api = fixture.FixtureApi(sheet().json());
      final key = await fixture.open(
        t,
        api,
        wrap: (app) => MultiProvider(
          providers: [
            ChangeNotifierProvider<PluginStore>.value(value: store),
            ChangeNotifierProvider.value(value: slots),
          ],
          child: app,
        ),
      );
      final d = key.currentState!.document!;

      await t.tap(find.byKey(const ValueKey('study-plugins')));
      await t.pumpAndSettle();
      await t.tap(find.text('钉到画布 fixture'));
      await t.pumpAndSettle();
      expect(d.pluginObjects.single.id, 'deck-1');
      expect(
        find.byKey(const ValueKey('study-plugin-entry/deck-1')),
        findsOneWidget,
      );

      // Overwriting asks first; declining changes nothing.
      await t.tap(find.text('钉到画布 fixture'));
      await t.pumpAndSettle();
      expect(find.text('允许「复习卡实验室」修改画布吗？'), findsOneWidget);
      expect(find.text('覆盖「换一组」'), findsOneWidget);
      await t.tap(find.text('放弃'));
      await t.pumpAndSettle();
      expect(d.pluginObjects.single.title, '复习卡');
      await t.tap(find.text('钉到画布 fixture'));
      await t.pumpAndSettle();
      await t.tap(find.text('应用改动'));
      await t.pumpAndSettle();
      expect(d.pluginObjects.single.title, '换一组');

      await t.tap(find.byTooltip('移除对象'));
      await t.pumpAndSettle();
      expect(d.pluginObjects, isEmpty);
      d.undo();
      expect(d.pluginObjects.single.title, '换一组');

      await key.currentState!.flush();
      expect(api.doc['version'], 5);
      expect((api.doc['plugin_objects'] as List).single['id'], 'deck-1');
      await close(t);
    },
  );
}
