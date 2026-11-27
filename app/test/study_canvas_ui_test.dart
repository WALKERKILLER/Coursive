// Protocol fixtures verify widgets, not real model inference.
import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;
import 'dart:convert';
import 'dart:typed_data';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../lib/core/api.dart';
import '../lib/core/models.dart' hide Source;
import '../lib/features/player/lecture_player.dart';
import '../lib/features/study_canvas/study_canvas.dart';
import '../lib/features/study_canvas/study_document.dart';
import '../lib/features/study_canvas/study_canvas_surface.dart';
import '../lib/theme/glass.dart';
import '../lib/theme/prefs.dart';
import '../lib/theme/theme.dart';
import '../lib/widgets/timestamp_links.dart';
import '../lib/widgets/md_reader.dart';

class FixtureApi extends Api {
  FixtureApi(this.doc) : super('http://127.0.0.1:9', 'fixture');
  Map<String, dynamic> doc;
  final responses = <String, dynamic>{};

  @override
  Future<dynamic> get(String path, [Map<String, String>? query]) async =>
      responses.containsKey(path) ? responses[path] : super.get(path, query);

  final syncEvents = StreamController<Map<String, dynamic>>.broadcast();
  Map? savedDraft;
  int saveConflicts = 0;
  Completer<Map<String, dynamic>>? nextRead;
  @override
  Stream<Map<String, dynamic>> sse(
    String path, {
    String method = 'GET',
    Object? body,
  }) => syncEvents.stream;
  @override
  Future<dynamic> post(String path, [Object? body, Duration? timeout]) async {
    if (path.endsWith('/draft')) {
      savedDraft = body as Map;
      return {'ok': true};
    }
    return super.post(path, body, timeout);
  }

  Uint8List? uploadedSnapshot;
  @override
  Future<dynamic> uploadStudyAttachment(
    String lecture,
    Uint8List bytes,
    Map selection,
  ) async {
    uploadedSnapshot = bytes;
    return {
      'id': 'snapshot',
      'sha256': 'fixture',
      'resource': 'saved.png',
      'mime_type': 'image/png',
      'width': 100,
      'height': 100,
      'selection': selection,
    };
  }

  Map? selection;
  Map? request;
  final conversations = <String, Map>{};
  @override
  Future<dynamic> studyConversation(String lecture, String id) async =>
      conversations[id] ??
      {
        'version': 1,
        'revision': 0,
        'id': id,
        'draft': '',
        'query': {'objects': [], 'selection': {}, 'include_video': false},
        'turns': [],
      };
  @override
  Future<dynamic> saveStudyConversation(
    String lecture,
    String id,
    Map data,
  ) async {
    conversations[id] = jsonDecode(
      jsonEncode({...data, 'revision': (data['revision'] as int) + 1}),
    ) as Map;
    return conversations[id];
  }

  @override
  Future<dynamic> studyCanvas(String id) async {
    final read = nextRead;
    nextRead = null;
    return read == null ? doc : read.future;
  }

  @override
  Future<dynamic> saveStudyCanvas(
    String id,
    Map<String, dynamic> document,
  ) async {
    if (saveConflicts > 0) {
      saveConflicts--;
      throw ApiError(409, 'another save arrived');
    }
    if (document['revision'] != doc['revision'] ||
        (document['base_hash'] ?? '') != (doc['base_hash'] ?? '')) {
      throw ApiError(409, 'stale canvas');
    }
    final revision = (doc['revision'] as int) + 1;
    doc = {...document, 'revision': revision, 'base_hash': 'saved-$revision'};
    return doc;
  }

  @override
  Future<dynamic> studyContext(String id, Map<String, dynamic> query) async {
    selection = query;
    final selected = query['selection'] as Map? ?? {};
    if ((selected['stroke_ids'] as List? ?? []).isNotEmpty) {
      return {
        'sources': [
          {
            'id': 'selected-ink',
            'kind': 'ink',
            'label': '所选笔迹',
            'availability': 'vision_required',
            'text': '',
            'anchor': {...selected, 'fingerprint': 'fixture-ink'},
          },
        ],
      };
    }
    return {
      'sources': [
        {
          'id': 'object-text',
          'label': '所选原文',
          'text': '真实选择',
          'availability': 'text',
          'anchor': {'object_id': 'text'},
        },
      ],
    };
  }

  @override
  Future<dynamic> startCanvasRun(Map<String, dynamic> value) async {
    request = value;
    return {'id': 'run'};
  }

  @override
  Future<dynamic> canvasRun(String id) async => {
    'state': 'error',
    'error': '尚未配置模型，请到设置添加',
    'trace': [],
  };
}

class DropFixtureApi extends FixtureApi {
  DropFixtureApi(super.doc);
  final imports = <String>[];

  @override
  Future<dynamic> importStudyAsset(
    String id, String name, Stream<List<int>> data, int length,
  ) async {
    expectSync((await data.single).length, length);
    imports.add(name);
    return {
      'objects': [
        StudyCard(
          id: 'dropped-${imports.length}', type: 'text', layer: 'main',
          rect: const Rect.fromLTWH(0, 0, 200, 320), title: name,
        ).json(),
      ],
    };
  }
}

class FixturePlayer implements LecturePlayer {
  @override
  final lectureId = 'fixture';
  @override
  final course = Course({'id': 'test-course'});
  @override
  List<Note> notes = [];
  @override
  Lecture? get lecture => null;
  @override
  List<CoursewareFile> get courseware => [];
  @override
  bool get mediaLoading => false;
  @override
  Object? get mediaError => null;
  @override
  Source? source;
  int? requestedPlayback;
  bool pausedAfterCitation = false;
  @override
  Future<void> playFrom(int ms, {int? ticket}) async {
    requestedPlayback = ms;
  }

  @override
  Future<void> pause() async {
    pausedAfterCitation = true;
  }
  @override
  List<Segment> get segments => [];
  @override
  final position = ValueNotifier(const Duration(seconds: 530));
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Future<GlobalKey<StudyCanvasState>> open(
  WidgetTester t,
  FixtureApi api, {
  GlobalKey? capture,
  Brightness brightness = Brightness.light,
  bool resetPreferences = true,
  bool seedLessonMaterials = false,
  LecturePlayer? player,
  Widget? video,
  Future<List<String>> Function()? clipboard,
  Widget Function(Widget app)? wrap,
}) async {
  if (resetPreferences) SharedPreferences.setMockInitialValues({});
  final prefs = await UiPrefs.load();
  t.view.physicalSize = const Size(1600, 1100);
  t.view.devicePixelRatio = 1;
  addTearDown(t.view.reset);
  final key = GlobalKey<StudyCanvasState>();
  final app = RepaintBoundary(
    key: capture,
    child: MaterialApp(
      theme: buildTheme(buildScheme(prefs, brightness), prefs),
      home: Scaffold(
        body: StudyCanvasSurface(
          prefs: prefs,
          child: StudyCanvas(
            key: key,
            api: api,
            seedLessonMaterials: seedLessonMaterials,
            ctl: player ?? FixturePlayer(),
            video: video ?? const Text('视频占位'),
            readClipboardFiles: clipboard ?? () async => [],
            panel: const SizedBox(width: 240),
            onClassic: () {},
            onAsk: (_) {},
            onBack: () {},
          ),
        ),
      ),
    ),
  );
  await t.pumpWidget(wrap == null ? app : wrap(app));
  await t.pumpAndSettle();
  return key;
}

class CountingStroke extends StudyStroke {
  CountingStroke(String id)
    : super(
        id: id,
        layer: 'main',
        points: const [
          InkPoint(Offset(100, 100), .5),
          InkPoint(Offset(110, 110), .5),
        ],
        color: 0xff000000,
        width: 3,
      );
  int paints = 0;
  @override
  void paint(Canvas canvas, {bool dark = false}) {
    paints++;
    super.paint(canvas, dark: dark);
  }
}

void main() {
  testWidgets('the primary recording header supports rename, delete and undo', (t) async {
    final player = FixturePlayer();
    final doc = StudyDocument()..cards.add(StudyCard(id: 'lecture', type: 'lecture',
        layer: 'main', rect: const Rect.fromLTWH(40, 120, 450, 280), title: '课程视频'));
    final api = FixtureApi(doc.json());
    final canvas = await open(t, api, player: player);
    Future<void> menu() async {
      await t.tap(find.byTooltip('更多').first);
      await t.pumpAndSettle();
    }
    await menu();
    await t.tap(find.text('重命名'));
    await t.pumpAndSettle();
    await t.enterText(find.byType(TextFormField), '进程与线程');
    await t.tap(find.text('保存'));
    await t.pumpAndSettle();
    expect(find.text('进程与线程'), findsOneWidget);
    final current = canvas.currentState!.document!;
    expect(StudyDocument.fromJson(current.json()).cards.single.headerTitle, '进程与线程');
    expect(current.cards.single.title, '课程视频');
    await menu();
    await t.tap(find.text('删除模块'));
    await t.pumpAndSettle();
    expect(current.cards, isEmpty);
    expect(player.pausedAfterCitation, isTrue);
    await t.tap(find.byTooltip('撤销'));
    await t.pumpAndSettle();
    expect(find.text('进程与线程'), findsOneWidget);
  });

  testWidgets('Expanded Markdown keeps the reader and returns to the canvas', (t) async {
    final doc = StudyDocument()
      ..cards.clear()
      ..cards.add(StudyCard(
        id: 'handout', type: 'text', layer: 'main',
        rect: const Rect.fromLTWH(100, 100, 620, 520),
        title: '复习讲义.md', source: 'study-assets/fixture/source.md',
        text: '# 概述\n\n${List.generate(80, (index) => '第 $index 段正文').join('\n\n')}\n\n## 练习\n问题',
      ));

    final key = await open(t, FixtureApi(doc.json()));
    final initialCamera = key.currentState!.document!.camera;
    final reader = find.byKey(const PageStorageKey('study-reader/handout'));
    final scroll = find.descendant(of: reader, matching: find.byType(Scrollable)).first;
    final scrollState = t.state<ScrollableState>(scroll);

    await t.drag(reader, const Offset(0, -280));
    await t.pumpAndSettle();
    final offset = scrollState.position.pixels;
    expect(offset, greaterThan(0));
    await t.tap(find.byTooltip('展开阅读或播放'));
    await t.pumpAndSettle();
    expect(find.widgetWithText(AppBar, '复习讲义.md'), findsOneWidget);
    expect(find.byType(ReaderToc), findsOneWidget);
    expect(find.byType(MarkdownReader), findsOneWidget);
    await t.tap(find.byIcon(Icons.search_rounded));
    await t.pumpAndSettle();
    expect(find.byType(ReaderSearchBar), findsOneWidget);
    await t.tap(find.byType(BackButton));
    await t.pumpAndSettle();
    expect(find.byType(AppBar), findsNothing);
    expect(t.state<ScrollableState>(scroll), same(scrollState));
    expect(scrollState.position.pixels, closeTo(offset, 1));
    expect(key.currentState!.document!.camera, initialCamera);
    expect(t.takeException(), isNull);
  });


  for (final expand in [false, true]) {
    testWidgets('Recording card opens the course player via ${expand ? 'header' : 'poster'}', (t) async {
      final player = FixturePlayer()
        ..source = Source(label: 'fixture', uri: 'file:///recording.mp4');
      final doc = StudyDocument()
        ..cards.clear()
        ..cards.addAll([
          StudyCard(id: 'lecture', type: 'lecture', layer: 'main',
              rect: const Rect.fromLTWH(40, 120, 450, 280), title: '主课堂'),
          StudyCard(id: 'copy', type: 'video', layer: 'main',
              rect: const Rect.fromLTWH(580, 120, 500, 320), title: '同节录像',
              sourceLectureId: 'fixture', originalPath: 'lesson/recording.mp4'),
        ]);
      final api = FixtureApi(doc.json())..responses['/api/lectures/fixture'] = {
        'assets': [
          {'kind': 'video', 'path': 'lesson/recording.mp4'},
        ],
      };
      final canvas = await open(t, api, player: player);
      final camera = canvas.currentState!.document!.camera;
      expect(find.byKey(const ValueKey('study-video-shelf-copy')), findsOneWidget);
      expect(find.byKey(const ValueKey('study-video-shelf')), findsOneWidget);
      final openCard = expand
          ? find.descendant(
              of: find.ancestor(of: find.text('同节录像'), matching: find.byType(Row)).first,
              matching: find.byTooltip('展开阅读或播放'),
            )
          : find.byIcon(Icons.play_arrow_rounded);
      await t.tap(openCard);
      await t.pumpAndSettle();
      expect(player.requestedPlayback, player.position.value.inMilliseconds);
      expect(find.widgetWithText(AppBar, '主课堂'), findsOneWidget);
      await t.tap(find.byType(BackButton));
      await t.pumpAndSettle();
      expect(canvas.currentState!.document!.camera, camera);
      expect(find.text('同节录像'), findsOneWidget);
    });
  }

  testWidgets('A recording citation returns to the expanded handout', (t) async {
    final player = FixturePlayer()
      ..source = Source(label: 'fixture', uri: 'file:///fixture.mp4');
    final doc = StudyDocument()
      ..cards.clear()
      ..cards.addAll([
        StudyCard(
          id: 'lecture', type: 'lecture', layer: 'main',
          rect: const Rect.fromLTWH(80, 120, 450, 280), title: '课程录像',
        ),
        StudyCard(
          id: 'handout', type: 'text', layer: 'main',
          rect: const Rect.fromLTWH(600, 120, 550, 420),
          title: '讲义.md', source: 'study-assets/fixture/source.md',
          text: '[回听原文](coursive://lecture/fixture?t=83)\n\n讲义正文',
        ),
      ]);

    await open(t, FixtureApi(doc.json()), player: player);
    final openHandout = find.descendant(
      of: find.ancestor(of: find.text('讲义.md'), matching: find.byType(Row)).first,
      matching: find.byTooltip('展开阅读或播放'),
    );
    await t.tap(openHandout);
    await t.pumpAndSettle();
    final reader = find.byType(MarkdownReader);
    final scroll = find.descendant(of: reader, matching: find.byType(Scrollable)).first;
    final scrollState = t.state<ScrollableState>(scroll);
    await t.tap(find.byIcon(Icons.search_rounded));
    await t.pumpAndSettle();
    final search = find.descendant(of: find.byType(ReaderSearchBar), matching: find.byType(TextField));
    await t.enterText(search, '正文');
    await t.pumpAndSettle();
    await t.tap(find.byType(TimestampChip));
    await t.pumpAndSettle();
    expect(player.requestedPlayback, 83000);
    expect(find.widgetWithText(AppBar, '课程录像'), findsOneWidget);
    await t.tap(find.byType(BackButton));
    await t.pumpAndSettle();
    expect(find.widgetWithText(AppBar, '讲义.md'), findsOneWidget);
    expect(t.state<ScrollableState>(scroll), same(scrollState));
    expect(t.widget<TextField>(search).controller!.text, '正文');
    expect(player.pausedAfterCitation, isTrue);
    expect(t.takeException(), isNull);
  });

  testWidgets('First-open materials yield to a canvas saved on another device', (t) async {
    final recording = StudyCard(
      id: 'lecture', type: 'lecture', layer: 'main',
      rect: const Rect.fromLTWH(0, 0, 880, 550), title: '课程录像',
    );
    final doc = StudyDocument()..cards.add(recording);
    final api = FixtureApi(doc.json());
    final note = {
      'id': 'seed-note', 'course_id': 'test-course', 'lecture_id': 'fixture',
      'title': '新的讲义', 'body': '原文',
    };
    api.responses['/api/notes/seed-note'] = note;
    api.responses['/api/lectures/fixture/courseware'] = [];
    final player = FixturePlayer()..notes = [Note(note)];

    final key = await open(t, api, player: player, seedLessonMaterials: true);
    expect(key.currentState!.document!.cards.any((c) => c.documentId == 'seed-note'), isTrue);
    final remote = StudyDocument()
      ..revision = 1
      ..baseHash = 'remote'
      ..cards.addAll([
        recording,
        StudyCard(
          id: 'existing', type: 'text', layer: 'main',
          rect: const Rect.fromLTWH(950, 30, 350, 220), title: '远端已编辑',
        ),
      ]);
    api.doc = remote.json();
    await t.pump(const Duration(seconds: 1));
    await t.pumpAndSettle();
    expect(key.currentState!.document!.cards.map((c) => c.id), ['lecture', 'existing']);
    expect(api.doc['revision'], 1);
    expect(api.savedDraft, isNull);
  });

  testWidgets('Dropping mixed files imports supported materials once', (t) async {
    final api = DropFixtureApi(StudyDocument().json());
    final key = await open(t, api);
    final target = t.widget<DropTarget>(find.byType(DropTarget));
    expect(target.enable, isTrue);
    const point = Offset(500, 300);
    target.onDragDone!(DropDoneDetails(
      files: [
        for (final name in ['课件.pdf', '复习 讲义.MD', '课堂.mp4', '略过.exe'])
          DropItemFile.fromData(Uint8List.fromList([1]), path: name),
      ],
      localPosition: point,
      globalPosition: point,
    ));
    await t.pumpAndSettle();
    for (var i = 0; i < 12 && api.imports.length < 3; i++) {
      await t.pump(const Duration(milliseconds: 100));
    }
    await t.pumpAndSettle();
    final notices = t.widgetList<SnackBar>(find.byType(SnackBar)).map((v) => (v.content as Text).data).toList();
    expect(api.imports, ['课件.pdf', '复习 讲义.MD', '课堂.mp4'], reason: '$notices');
    expect(key.currentState!.document!.cards.map((c) => c.title), api.imports);
    expect((api.doc['objects'] as List).length, 3);
  });

  testWidgets(
    'Command V reads Finder only on the active canvas outside text entry',
    (t) async {
      var reads = 0;
      final doc = StudyDocument()
        ..cards.add(StudyCard(
          id: 'practice', type: 'practice', layer: 'main',
          rect: const Rect.fromLTWH(130, 60, 520, 650), title: '自测',
          generation: 'fixture-run',
          blocks: [StudyBlock(type: 'exercise', text: '操作系统', answer: '答案')],
        ));
      final key = await open(t, FixtureApi(doc.json()), clipboard: () async {
        reads++;
        return [];
      });
      final platform = TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
      platform.setMockMethodCallHandler(SystemChannels.platform, (call) async =>
          call.method == 'Clipboard.getData' ? {'text': '正文'} : null);
      addTearDown(() => platform.setMockMethodCallHandler(SystemChannels.platform, null));
      Future<void> paste() async {
        await t.sendKeyDownEvent(LogicalKeyboardKey.metaLeft);
        await t.sendKeyEvent(LogicalKeyboardKey.keyV);
        await t.sendKeyUpEvent(LogicalKeyboardKey.metaLeft);
        await t.pump();
      }
      await t.sendKeyEvent(LogicalKeyboardKey.keyV);
      expect(reads, 0);
      await paste();
      expect(reads, 1);
      final field = find.widgetWithText(TextField, '我的作答');
      await t.tap(field);
      await t.pumpAndSettle();
      await paste();
      expect(reads, 1);
      expect(t.widget<TextField>(field).controller!.text, '正文');
      FocusManager.instance.primaryFocus!.unfocus();
      await t.pumpAndSettle();
      final dialog = showDialog<void>(
        context: key.currentContext!,
        builder: (_) => const AlertDialog(content: TextField(autofocus: true)),
      );
      await t.pumpAndSettle();
      await paste();
      expect(reads, 1);
      expect(t.widget<EditableText>(find.descendant(
        of: find.byType(AlertDialog), matching: find.byType(EditableText),
      )).controller.text, '正文');
      Navigator.of(key.currentContext!).pop();
      await t.pumpAndSettle();
      await dialog;
      await paste();
      expect(reads, 2);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
    skip: !Platform.isMacOS,
    variant: TargetPlatformVariant.only(TargetPlatform.macOS),
  );
  testWidgets(
    'a long new stroke retains already painted history through pen-up and undo',
    (t) async {
      final key = await open(t, FixtureApi(StudyDocument().json()));
      await t.tap(find.byKey(const ValueKey('study-tool-pen')));
      final doc = key.currentState!.document!;
      final history = List.generate(128, (i) => CountingStroke('test-$i'));
      doc.strokes.addAll(history);
      doc.changed();
      await t.pumpAndSettle();
      int paints() => history.fold(0, (n, s) => n + s.paints);
      final count = paints();
      expect(count, greaterThan(0));
      final pen = await t.startGesture(
        const Offset(420, 360),
        kind: ui.PointerDeviceKind.stylus,
      );
      for (var i = 0; i < 100; i++) {
        await pen.moveTo(Offset(421 + i * 3, i.isEven ? 367 : 360));
        if (i % 8 == 0) await t.pump();
      }
      await pen.up();
      await t.pumpAndSettle();
      expect(doc.strokes.length, greaterThan(129));
      expect(paints(), count);
      doc.undo();
      await t.pump();
      expect(doc.strokes, history);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Android pen mode suspends backdrop sampling until browsing resumes',
    (t) async {
      await open(t, FixtureApi(StudyDocument().json()));
      final filters = find.descendant(
        of: find.byType(GlassBackdropScope),
        matching: find.byType(BackdropFilter),
      );
      expect(filters, findsWidgets);
      await t.tap(find.byKey(const ValueKey('study-tool-pen')));
      await t.pumpAndSettle();
      expect(filters, findsNothing);
      await t.tap(find.byKey(const ValueKey('study-tool-select')));
      await t.pumpAndSettle();
      expect(filters, findsWidgets);
      await t.pumpWidget(const SizedBox());
    },
    variant: TargetPlatformVariant.only(TargetPlatform.android),
  );
  for (final scale in [.6, 1.0, 1.2]) {
    testWidgets('video footer fits text at zoom $scale and opens shelf', (
      t,
    ) async {
      final doc = StudyDocument()
        ..camera = const Offset(100, 120)
        ..zoom = scale
        ..cards.add(
          StudyCard(
            id: 'lecture',
            type: 'lecture',
            layer: 'main',
            rect: const Rect.fromLTWH(100, 100, 700, 500),
          ),
        );
      await open(t, FixtureApi(doc.json()));
      final button = find.byKey(const ValueKey('study-video-shelf'));
      final label = find.descendant(of: button, matching: find.byType(Text));
      final bounds = t.getRect(button);
      final text = t.getRect(label);
      expect(bounds.contains(text.topLeft), isTrue);
      expect(bounds.contains(text.bottomRight), isTrue);
      final rich = find.descendant(of: label, matching: find.byType(RichText));
      final paragraph = t.widget<RichText>(rich);
      final render = t.renderObject<RenderParagraph>(rich);
      final measured = TextPainter(
        text: paragraph.text,
        textDirection: paragraph.textDirection ?? TextDirection.ltr,
        textScaler: paragraph.textScaler,
        maxLines: 1,
      )..layout(maxWidth: render.size.width);
      expect(render.size.height, greaterThanOrEqualTo(measured.height));
      measured.dispose();
      expect(find.byTooltip('问 AI'), findsNothing);
      expect(find.byKey(const ValueKey('study-ai-tab')), findsOneWidget);
      await t.tap(find.byKey(const ValueKey('study-tool-select')));
      await t.pumpAndSettle();
      await t.tap(button);
      await t.pumpAndSettle();
      expect(find.text('这节课还没有字幕'), findsOneWidget);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    });
  }
  testWidgets(
    'paper appearance survives reopening without editing the course',
    (t) async {
      final api = FixtureApi(StudyDocument().json());
      final capture = GlobalKey();
      final key = await open(t, api, capture: capture);
      final before = jsonEncode(key.currentState!.document!.json());
      await t.tap(find.byTooltip('纸张背景'));
      await t.pumpAndSettle();
      await t.tap(find.text('空白'));
      await t.tap(find.text('米色'));
      await t.tap(find.text('完成'));
      await t.pumpAndSettle();
      expect(jsonEncode(key.currentState!.document!.json()), before);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
      await open(t, api, capture: capture, resetPreferences: false);
      final boundary =
          capture.currentContext!.findRenderObject() as RenderRepaintBoundary;
      final image = (await t.runAsync(() => boundary.toImage(pixelRatio: 1)))!;
      final pixels = (await t.runAsync(
        () => image.toByteData(format: ui.ImageByteFormat.rawRgba),
      ))!;
      final i = (500 * image.width + 500) * 4;
      expect(
        [pixels.getUint8(i), pixels.getUint8(i + 1), pixels.getUint8(i + 2)],
        [255, 246, 221],
      );
      image.dispose();
      await t.tap(find.byTooltip('纸张背景'));
      await t.pumpAndSettle();
      expect(
        t.widget<ChoiceChip>(find.widgetWithText(ChoiceChip, '米色')).selected,
        isTrue,
      );
      await t.tap(find.text('方格'));
      await t.tap(find.text('完成'));
      await t.pumpAndSettle();
      final prefs = await SharedPreferences.getInstance();
      expect(
        jsonDecode(prefs.getString('study.notein-layout.v2')!)['paper'],
        'grid',
      );
      await t.pumpWidget(const SizedBox());
      await api.syncEvents.close();
    },
  );

  testWidgets('edge docking persists without changing course content', (
    t,
  ) async {
    final doc = StudyDocument()
      ..camera = Offset.zero
      ..zoom = 1;
    final api = FixtureApi(doc.json());
    final key = await open(t, api);
    final before = jsonEncode(key.currentState!.document!.json());
    await t.sendEventToBinding(
      PointerScrollEvent(
        position: t.getCenter(
          find.byKey(const ValueKey('study-dock-handle-pens')),
        ),
        scrollDelta: const Offset(0, 80),
      ),
    );
    await t.pumpAndSettle();
    final stage = t.getRect(find.byKey(const ValueKey('study-input')));
    expect(stage.top, 0);
    expect(find.textContaining('拖动卡片标题'), findsNothing);
    final handle = find.byKey(const ValueKey('study-dock-handle-pens'));
    expect(t.widget<SizedBox>(handle).child, isNull);
    final swatch = find.byKey(const ValueKey('study-pen-color-4293284709'));
    expect(
      t.getRect(handle).bottom,
      lessThanOrEqualTo(t.getRect(swatch).top + .01),
    );
    Future<void> dragDock(Offset target) async {
      final start = t.getCenter(handle);
      final gesture = await t.startGesture(start);
      await gesture.moveTo((start + target) / 2);
      await t.pump(const Duration(milliseconds: 32));
      await gesture.moveTo(target);
      await t.pump(const Duration(milliseconds: 32));
      await gesture.up();
      await t.pumpAndSettle();
    }

    // Navigation stays at the top and dragging its background never pans ink.
    final modules = find.byKey(const ValueKey('study-top-navigation'));
    final original = t.getRect(modules);
    final toolbar = find.byKey(const ValueKey('study-top-tools'));
    final tools = t.getRect(toolbar);
    expect(find.byTooltip('手写 / 浏览'), findsNothing);
    expect(find.byTooltip('课程资料'), findsNothing);
    final actions = find.byKey(const ValueKey('study-top-actions'));
    expect(
      t
          .widgetList<Icon>(
            find.descendant(of: actions, matching: find.byType(Icon)),
          )
          .map((icon) => icon.icon),
      [Icons.more_horiz],
    );
    expect(
      find.descendant(
        of: toolbar,
        matching: find.byIcon(Icons.near_me_outlined),
      ),
      findsOneWidget,
    );
    expect(tools.height, 36);
    expect(tools.width, lessThan(stage.width * .6));
    expect((tools.center.dx - stage.center.dx).abs(), lessThan(1));
    await t.dragFrom(
      original.topLeft + const Offset(4, 4),
      const Offset(80, 280),
    );
    await t.pumpAndSettle();
    expect(t.getRect(modules), original);
    expect(
      find.byKey(const ValueKey('study-dock-handle-modules')),
      findsNothing,
    );
    expect(find.byKey(const ValueKey('study-dock-handle-tools')), findsNothing);

    // Docks keep an 8 px margin from the edge they snap to.
    const margin = 8.0;
    await dragDock(stage.centerLeft);
    expect(
      t.getRect(find.byKey(const ValueKey('study-dock-pens'))).left,
      margin,
    );
    expect(t.getRect(modules), original);
    for (final target in [
      stage.topCenter,
      stage.bottomCenter,
      stage.centerRight,
      stage.centerLeft,
    ]) {
      await dragDock(target);
      final rect = t.getRect(find.byKey(const ValueKey('study-dock-pens')));
      expect(stage.contains(rect.topLeft), isTrue);
      expect(rect.right, lessThanOrEqualTo(stage.right));
      expect(rect.bottom, lessThanOrEqualTo(stage.bottom));
      final error = t.takeException();
      expect(error, isNull, reason: 'docking to $target');
    }
    t.view.physicalSize = const Size(400, 700);
    await t.pumpAndSettle();
    expect(t.takeException(), isNull);
    t.view.physicalSize = const Size(1600, 1100);
    await t.pumpAndSettle();
    expect(jsonEncode(key.currentState!.document!.json()), before);
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
    await open(t, api, resetPreferences: false);
    expect(
      t.getRect(find.byKey(const ValueKey('study-dock-pens'))).left,
      margin,
    );
    await t.pumpWidget(const SizedBox());
    await api.syncEvents.close();
  });

  testWidgets(
    'pen dock unfolds and stays clear of the fixed conversation entry',
    (t) async {
      final api = FixtureApi(StudyDocument().json());
      final key = await open(t, api);
      final original = jsonEncode(key.currentState!.document!.json());
      await t.tap(find.byTooltip('收起笔盒'));
      await t.pumpAndSettle();
      final handle = find.byKey(const ValueKey('study-dock-handle-pens'));
      final drag = await t.startGesture(t.getCenter(handle));
      await drag.moveTo(const Offset(1450, 550));
      await t.pump();
      expect(
        find.byKey(const ValueKey('study-dock-floating-pens')),
        findsOneWidget,
      );
      await drag.up();
      await t.pumpAndSettle();
      expect(find.byTooltip('收起笔盒'), findsOneWidget);

      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      expect(find.byTooltip('发送'), findsOneWidget);
      // Fixed navigation remains available above the open sidebar.
      expect(find.byTooltip('页面与材料').hitTestable(), findsOneWidget);
      await t.tap(find.byKey(const ValueKey('study-ai-close')));
      await t.pumpAndSettle();
      expect(find.byTooltip('页面与材料').hitTestable(), findsOneWidget);
      void separated() {
        final rects = [
          t.getRect(find.byKey(const ValueKey('study-dock-pens'))),
          for (final id in ['navigation', 'tools', 'actions'])
            t.getRect(find.byKey(ValueKey('study-top-$id'))),
        ];
        for (var i = 0; i < rects.length; i++) {
          expect(rects[i].left, greaterThanOrEqualTo(0));
          expect(rects[i].right, lessThanOrEqualTo(t.view.physicalSize.width));
          expect(
            rects[i].bottom,
            lessThanOrEqualTo(t.view.physicalSize.height),
          );
          for (var j = i + 1; j < rects.length; j++) {
            expect(
              rects[i].overlaps(rects[j]),
              false,
              reason: '${rects[i]} vs ${rects[j]}',
            );
          }
        }
      }

      separated();
      t.view.physicalSize = const Size(400, 700);
      await t.pumpAndSettle();
      separated();
      expect(t.takeException(), isNull);
      t.view.physicalSize = const Size(1600, 1100);
      await t.pumpAndSettle();
      final aiRect = t.getRect(find.byKey(const ValueKey('study-ai-tab')));
      expect(jsonEncode(key.currentState!.document!.json()), original);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
      await open(t, api, resetPreferences: false);
      expect(t.getRect(find.byKey(const ValueKey('study-ai-tab'))), aiRect);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
      // Previously saved overlapping collapsed controls must also recover.
      SharedPreferences.setMockInitialValues({
        'study.notein-layout.v2': jsonEncode({
          for (final name in ['pens', 'tools', 'modules', 'ai'])
            name: {'edge': 'right', 'offset': .5},
          'pens_small': true,
          'tools_small': true,
        }),
      });
      await open(t, api, resetPreferences: false);
      separated();
      expect(find.byTooltip('展开笔盒'), findsOneWidget);
      expect(
        find.byKey(const ValueKey('study-tool-select')).hitTestable(),
        findsOneWidget,
      );
      expect(find.byTooltip('展开工具栏'), findsNothing);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets(
    'width slider edits one slot above the dock and survives reopening',
    (t) async {
      final api = FixtureApi(StudyDocument().json());
      await open(t, api);
      final button = find.byTooltip('笔宽 3');
      final buttonTop = t.getTopLeft(button).dy;
      await t.tap(button);
      await t.pumpAndSettle();
      final slider = find.byKey(const ValueKey('study-width-slider'));
      expect(t.getBottomLeft(slider).dy, lessThanOrEqualTo(buttonTop));
      await t.tapAt(t.getCenter(slider));
      await t.pumpAndSettle();
      final value = t.widget<Slider>(slider).value;
      expect(value, greaterThan(3));
      final label = value == value.roundToDouble()
          ? value.toInt().toString()
          : value.toStringAsFixed(1);
      await t.tapAt(const Offset(60, 120));
      await t.pumpAndSettle();
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
      await open(t, api, resetPreferences: false);
      expect(find.byTooltip('笔宽 $label'), findsOneWidget);
      expect(find.byTooltip('笔宽 1'), findsOneWidget);
      expect(find.byTooltip('笔宽 6'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );

  testWidgets('pen artwork shares a common height and baseline', (t) async {
    await open(t, FixtureApi(StudyDocument().json()));
    final tools = [
      'study-tool-pen',
      for (var i = 1; i < 5; i++) 'study-pen-$i',
      'study-tool-eraser',
      'study-tool-region',
    ];
    final bounds = <String, (int, int)>{};
    for (final key in tools) {
      final artwork = find.descendant(
        of: find.byKey(ValueKey(key)),
        matching: find.byType(CustomPaint),
      );
      final paint = t.widget<CustomPaint>(artwork);
      final recorder = ui.PictureRecorder();
      paint.painter!.paint(Canvas(recorder), const Size(22, 38));
      final picture = recorder.endRecording();
      final image = picture.toImageSync(22, 38);
      final pixels = (await t.runAsync(
        () => image.toByteData(format: ui.ImageByteFormat.rawRgba),
      ))!;
      image.dispose();
      picture.dispose();
      final rows = [
        for (var y = 0; y < 38; y++)
          if (Iterable<int>.generate(22)
              .any((x) => pixels.getUint8((y * 22 + x) * 4 + 3) >= 128))
            y,
      ];
      bounds[key] = (rows.first, rows.last);
    }
    final (referenceTip, referenceBase) = bounds.values.first;
    for (final MapEntry(key: key, value: (tip, base)) in bounds.entries) {
      // Pointed and rounded tips cover different fractions of the first pixel.
      expect(tip, closeTo(referenceTip, 2), reason: key);
      expect(base, closeTo(referenceBase, 1), reason: key);
    }
  });

  testWidgets(
    'pen selection recalls its settings and second tap opens settings',
    (t) async {
      final api = FixtureApi(StudyDocument().json());
      await open(t, api);
      final first = find.byKey(const ValueKey('study-tool-pen'));
      final second = find.byKey(const ValueKey('study-pen-1'));
      await t.tap(first);
      await t.pumpAndSettle();
      expect(find.text('实时笔迹预览'), findsNothing);
      await t.tap(find.byTooltip('笔宽 6'));
      await t.pumpAndSettle();
      expect(find.byKey(const ValueKey('study-width-slider')), findsOneWidget);
      await t.tapAt(const Offset(60, 120));
      await t.pumpAndSettle();
      await t.tap(second);
      await t.pumpAndSettle();
      expect(
        t
            .widget<IconButton>(
              find
                  .ancestor(
                    of: find.byTooltip('笔宽 6'),
                    matching: find.byType(IconButton),
                  )
                  .first,
            )
            .isSelected,
        isFalse,
      );
      await t.tap(first);
      await t.pumpAndSettle();
      expect(
        t
            .widget<IconButton>(
              find
                  .ancestor(
                    of: find.byTooltip('笔宽 6'),
                    matching: find.byType(IconButton),
                  )
                  .first,
            )
            .isSelected,
        isTrue,
      );
      await t.tap(first);
      await t.pumpAndSettle();
      expect(find.text('实时笔迹预览'), findsOneWidget);
      expect(
        t.getRect(find.byKey(const ValueKey('study-pen-popover'))).bottom,
        lessThan(t.getRect(first).top),
      );
      await t.tap(find.byTooltip('关闭设置'));
      await t.pumpAndSettle();
      await t.tap(find.byKey(const ValueKey('study-tool-eraser')));
      await t.pumpAndSettle();
      final color = find.byKey(const ValueKey('study-pen-color-4293284709'));
      await t.ensureVisible(color);
      await t.tap(color);
      await t.pumpAndSettle();
      expect(
        t
            .getSemantics(find.byKey(const ValueKey('study-tool-eraser')))
            .getSemanticsData()
            .hasFlag(ui.SemanticsFlag.isSelected),
        isFalse,
      );
      await t.pumpWidget(const SizedBox());
      await api.syncEvents.close();
    },
  );

  for (final brightness in Brightness.values) {
    testWidgets('canvas controls remain readable in ${brightness.name} mode', (
      t,
    ) async {
      await open(t, FixtureApi(StudyDocument().json()), brightness: brightness);

      void readable(Finder control, double minimum) {
        final glyph = find
            .descendant(of: control, matching: find.byType(RichText))
            .first;
        final foreground = t.widget<RichText>(glyph).text.style!.color!;
        final page = find
            .ancestor(of: glyph, matching: find.byType(Material))
            .evaluate()
            .map((e) => e.widget as Material)
            .firstWhere((m) => m.color?.a == 1)
            .color!;
        // Glass panes composite their fill over what lies beneath.
        final glass = find.ancestor(
          of: glyph,
          matching: find.byType(GlassSurface),
        );
        final background = glass.evaluate().isEmpty
            ? page
            : Color.alphaBlend(
                GlassSurface.fillOf(
                  t.element(glass.first),
                  depth: t.widget<GlassSurface>(glass.first).depth,
                  tint: t.widget<GlassSurface>(glass.first).tint,
                ),
                page,
              );
        final ink = Color.alphaBlend(foreground, background).computeLuminance();
        final paper = background.computeLuminance();
        final ratio =
            (ink > paper ? ink + .05 : paper + .05) /
            (ink > paper ? paper + .05 : ink + .05);
        expect(ratio, greaterThanOrEqualTo(minimum), reason: '$control');
      }

      readable(find.byKey(const ValueKey('study-tool-select')), 3);
      expect(find.byKey(const ValueKey('study-tool-hand')), findsNothing);
      readable(find.byTooltip('画笔与橡皮设置'), 3);
      readable(find.byTooltip('画布菜单'), 3);
      readable(find.byTooltip('插入图片、PDF 或文件'), 3);
      readable(find.byKey(const ValueKey('study-ai-tab')), 3);
      expect(
        t
            .widget<IconButton>(find.widgetWithIcon(IconButton, Icons.undo))
            .onPressed,
        isNull,
      );
      await t.tap(find.byTooltip('画笔与橡皮设置'));
      await t.pumpAndSettle();
      readable(find.text('笔盒'), 4.5);
      readable(find.text('实时笔迹预览'), 4.5);
      readable(find.text('圆珠笔与荧光笔保持均匀宽度；钢笔、铅笔与软笔响应压力。鼠标使用固定压力。'), 4.5);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    });
  }

  testWidgets(
    'latest saved whole canvas refreshes and pending edits finish automatically',
    (t) async {
      final first = StudyDocument()..baseHash = 'old';
      final api = FixtureApi(first.json());
      final key = await open(t, api);
      final remote = StudyDocument()..baseHash = 'remote';
      remote.cards.add(
        StudyCard(
          id: 'remote',
          type: 'text',
          layer: 'main',
          rect: const Rect.fromLTWH(40, 70, 300, 200),
          text: '另一端内容',
          title: '远端',
        ),
      );
      api.doc = remote.json();
      api.syncEvents.add({
        'type': 'sync_done',
        'data': {'downloaded': 1},
      });
      await t.pumpAndSettle();
      expect(key.currentState!.document!.cards.single.id, 'remote');
      final local = key.currentState!.document!;
      local.cards.single.text = '本机未保存';
      local.changed();
      await t.pump(const Duration(milliseconds: 10));
      final latest = StudyDocument.fromJson(remote.json())
        ..baseHash = 'new-remote';
      latest.cards.single.text = '另一端再次修改';
      api.doc = latest.json();
      api.syncEvents.add({
        'type': 'sync_done',
        'data': {'downloaded': 1},
      });
      await t.pumpAndSettle();
      expect(key.currentState!.document!.cards.single.text, '本机未保存');
      expect(find.text('保留本机'), findsNothing);
      expect(find.text('采用另一端'), findsNothing);
      await t.pump(const Duration(seconds: 2));
      await t.pumpAndSettle();
      expect(api.doc['objects'].single['text'], '本机未保存');
      expect(api.savedDraft!['objects'].single['text'], '另一端再次修改');
      expect(find.byTooltip('已保存到本机'), findsNothing);
      final next = StudyDocument.fromJson(api.doc)..baseHash = 'next';
      next.cards.clear();
      api.doc = next.json();
      api.syncEvents.add({
        'type': 'sync_done',
        'data': {'downloaded': 1},
      });
      await t.pumpAndSettle();
      expect(key.currentState!.document!.cards, isEmpty);
      await t.pumpWidget(const SizedBox());
      await api.syncEvents.close();
    },
  );

  testWidgets('canvas catches up when a sync notification is missed', (
    t,
  ) async {
    final api = FixtureApi(StudyDocument().json());
    final key = await open(t, api);
    final remote = StudyDocument()..baseHash = 'arrived-without-event';
    remote.cards.add(
      StudyCard(
        id: 'arrived',
        type: 'text',
        layer: 'main',
        rect: const Rect.fromLTWH(40, 70, 300, 200),
        text: '已到本机但未推送',
        title: '收到更新',
      ),
    );
    api.doc = remote.json();
    await t.pump(const Duration(seconds: 5));
    await t.pumpAndSettle();
    expect(key.currentState!.document!.cards.single.id, 'arrived');
    await t.pumpWidget(const SizedBox());
    await api.syncEvents.close();
  });

  testWidgets('a local edit started during a remote read finishes saving', (
    t,
  ) async {
    final api = FixtureApi(StudyDocument().json());
    final key = await open(t, api);
    final read = Completer<Map<String, dynamic>>();
    api.nextRead = read;
    api.syncEvents.add({
      'type': 'sync_done',
      'data': {'downloaded': 1},
    });
    await t.pump();
    final doc = key.currentState!.document!;
    doc.cards.add(
      StudyCard(
        id: 'typing',
        type: 'text',
        layer: 'main',
        rect: const Rect.fromLTWH(40, 70, 300, 200),
        text: '刚刚开始写',
        title: '本机',
      ),
    );
    doc.changed();
    api.doc = (StudyDocument()..baseHash = 'remote-read').json();
    read.complete(api.doc);
    await t.pumpAndSettle();
    expect(key.currentState!.document!.cards.single.text, '刚刚开始写');
    await t.pump(const Duration(seconds: 2));
    await t.pumpAndSettle();
    expect(api.doc['objects'].single['text'], '刚刚开始写');
    await t.pumpWidget(const SizedBox());
    await api.syncEvents.close();
  });

  testWidgets('a revision retry saves a format 8 canvas without downgrading it', (
    t,
  ) async {
    final initial = StudyDocument()..formatVersion = 8;
    final api = FixtureApi(initial.json());
    final key = await open(t, api);
    api.doc = {...api.doc, 'revision': 1, 'base_hash': 'remote'};
    final doc = key.currentState!.document!;
    doc.camera = const Offset(90, 100);
    doc.changed();

    expect(await key.currentState!.flush(), isTrue);
    expect(api.savedDraft?['version'], 8);
    expect(api.doc['version'], 8);
    expect(api.doc['revision'], 2);
    expect(api.doc['camera']['x'], 90);

    await t.pumpWidget(const SizedBox());
    await api.syncEvents.close();
  });

  for (final conflicts in [2, 3]) {
    testWidgets('canvas save retries are bounded after $conflicts races', (
      t,
    ) async {
      final api = FixtureApi(StudyDocument().json())..saveConflicts = conflicts;
      final key = await open(t, api);
      final doc = key.currentState!.document!;
      doc.camera = const Offset(90, 100);
      doc.changed();
      final saved = await key.currentState!.flush();
      expect(saved, conflicts == 2);
      expect(doc.camera, const Offset(90, 100));
      if (saved) {
        expect(api.doc['camera']['x'], 90);
      } else {
        expect(doc.edit, greaterThan(0));
      }
      await t.pumpWidget(const SizedBox());
      await api.syncEvents.close();
    });
  }

  testWidgets('pen dock does not draw ink and brush choice is saved', (
    t,
  ) async {
    final capture = GlobalKey();
    final key = await open(
      t,
      FixtureApi(StudyDocument().json()),
      capture: capture,
    );
    await t.tap(find.byKey(const ValueKey('study-tool-pen')));
    await t.pumpAndSettle();
    await t.tap(find.byTooltip('画笔与橡皮设置'));
    await t.pumpAndSettle();
    final header = t.getRect(find.text('笔盒'));
    final boundary =
        capture.currentContext!.findRenderObject() as RenderRepaintBoundary;
    final image = (await t.runAsync(() => boundary.toImage(pixelRatio: 1)))!;
    final pixels = (await t.runAsync(
      () => image.toByteData(format: ui.ImageByteFormat.rawRgba),
    ))!;
    var dark = 0;
    for (var y = header.top.ceil(); y < header.bottom.floor(); y++) {
      for (var x = header.left.ceil(); x < header.right.floor(); x++) {
        final i = (y * image.width + x) * 4;
        if (pixels.getUint8(i) < 120 &&
            pixels.getUint8(i + 1) < 120 &&
            pixels.getUint8(i + 2) < 120)
          dark++;
      }
    }
    image.dispose();
    expect(dark, greaterThan(20), reason: '笔触预览不能覆盖其边界外的笔盒标题');
    await t.tap(find.text('铅笔').last);
    await t.pumpAndSettle();
    expect(key.currentState!.document!.strokes, isEmpty);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('study.brush-presets.v1'), contains('pencil'));
    final saved = jsonDecode(prefs.getString('study.brush-presets.v1')!);
    expect(saved['presets'][0]['name'], '铅笔');
    // Older versions could save a new brush under a stale built-in name.
    saved['presets'][0]['name'] = '钢笔';
    await prefs.setString('study.brush-presets.v1', jsonEncode(saved));
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
    await open(t, FixtureApi(StudyDocument().json()), resetPreferences: false);
    expect(find.byTooltip('钢笔：点选，再点设置'), findsNothing);
    expect(find.byTooltip('铅笔：点选，再点设置'), findsNWidgets(2));
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
  testWidgets(
    'real text selection offsets and video time reach context; missing model is visible',
    (t) async {
      final d = StudyDocument()
        ..cards.add(
          StudyCard(
            id: 'text',
            type: 'text',
            layer: 'main',
            rect: const Rect.fromLTWH(130, 80, 500, 400),
            text: '前😀后选中内容',
            title: '材料',
          ),
        );
      final api = FixtureApi(d.json());
      final key = await open(t, api);
      final selectable = t.widget<SelectableText>(
        find.byWidgetPredicate(
          (w) => w is SelectableText && w.data == '前😀后选中内容',
        ),
      );
      selectable.onSelectionChanged!(
        const TextSelection(baseOffset: 3, extentOffset: 6),
        SelectionChangedCause.drag,
      );
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      await t.pumpAndSettle();
      expect(api.selection?['objects'], ['text']);
      expect(api.selection?['selection']['start'], 3);
      expect(api.selection?['selection']['end'], 6);
      expect(api.selection?['include_video'], false);
      expect(find.text('所选原文'), findsOneWidget);
      await t.enterText(find.byType(TextField).last, '解释选区');
      await t.tap(find.byTooltip('发送'));
      await t.pumpAndSettle();
      expect(find.text('尚未配置模型，请到设置添加'), findsOneWidget);
      expect(key.currentState!.document!.cards.length, 2);
      expect(api.conversations.values.single['turns'], isNotEmpty);

      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
  testWidgets('exercise responses persist and undo refreshes the editor', (
    t,
  ) async {
    final d = StudyDocument()
      ..cards.add(
        StudyCard(
          id: 'practice',
          type: 'practice',
          layer: 'main',
          rect: const Rect.fromLTWH(130, 60, 520, 650),
          title: '自测',
          generation: 'fixture-run',
          blocks: [
            StudyBlock(
              type: 'exercise',
              text: '为什么 miss 更慢？',
              answer: '需要访问主存',
            ),
          ],
        ),
      );
    final api = FixtureApi(d.json());
    final key = await open(t, api);
    final input = find.widgetWithText(TextField, '我的作答');
    await t.tap(input);
    await t.enterText(input, '需要访问下一层存储');
    await t.pumpAndSettle();
    expect(
      key.currentState!.document!.cards.single.responses['0'],
      '需要访问下一层存储',
    );
    await key.currentState!.flush();
    expect(
      StudyDocument.fromJson(api.doc).cards.single.responses['0'],
      '需要访问下一层存储',
    );
    key.currentState!.document!.undo();
    await t.pumpAndSettle();
    expect(t.widget<TextField>(input).controller!.text, '');
    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
  testWidgets('drawn ink is attached as one bounded real PNG after 「看图」', (
    t,
  ) async {
    final api = FixtureApi(StudyDocument().json());
    final key = await open(t, api);
    await t.tap(find.byKey(const ValueKey('study-tool-pen')));
    await t.pumpAndSettle();
    await t.dragFrom(
      const Offset(400, 350),
      const Offset(160, 90),
      kind: ui.PointerDeviceKind.stylus,
    );
    await t.pumpAndSettle();
    expect(key.currentState!.document!.strokes, isNotEmpty);
    await t.runAsync(() async {
      await t.tap(find.byKey(const ValueKey('study-ai-tab')));
      for (var i = 0; i < 80 && api.selection == null; i++) {
        await t.pump();
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
    });
    await t.pumpAndSettle();
    // 「看图」 is off, so nothing was captured yet.
    expect(api.uploadedSnapshot, isNull);
    // Opting in takes the screenshot, outside the fake clock.
    await t.runAsync(() async {
      await t.tap(find.text('看图'));
      for (var i = 0; i < 80 && api.uploadedSnapshot == null; i++) {
        await t.pump();
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
    });
    await t.pumpAndSettle();
    await t.enterText(find.byType(TextField).last, '解释这条实际笔迹');
    // Real raster readback must run outside the widget test's fake clock.
    await t.runAsync(() async {
      await t.tap(find.byTooltip('发送'));
      for (var i = 0; i < 80; i++) {
        await t.pump();
        await Future<void>.delayed(const Duration(milliseconds: 10));
      }
    });
    await t.pumpAndSettle();
    final attachment = (api.request!['attachments'] as List).single as Map;
    expect(attachment['id'], 'snapshot');
    final bytes = api.uploadedSnapshot!;
    expect(bytes.take(8), [137, 80, 78, 71, 13, 10, 26, 10]);
    final codec = (await t.runAsync(() => ui.instantiateImageCodec(bytes)))!;
    final frame = (await t.runAsync(codec.getNextFrame))!;
    expect(frame.image.width, lessThanOrEqualTo(1600));
    expect(frame.image.height, lessThanOrEqualTo(1600));
    expect(bytes.length, lessThanOrEqualTo(3 * 1024 * 1024));
    final pixels = (await t.runAsync(
      () => frame.image.toByteData(format: ui.ImageByteFormat.rawRgba),
    ))!;
    var dark = 0;
    for (var i = 0; i < pixels.lengthInBytes; i += 4) {
      if (pixels.getUint8(i) < 120 &&
          pixels.getUint8(i + 1) < 120 &&
          pixels.getUint8(i + 2) < 120)
        dark++;
    }
    expect(dark, greaterThan(10), reason: '选区附件必须包含实际画出的深色笔迹');
    frame.image.dispose();
    codec.dispose();

    await t.pumpWidget(const SizedBox());
    await t.pumpAndSettle();
  });
  testWidgets(
    'clicking a citation focuses the moved source and exposes its real excerpt',
    (t) async {
      final d = StudyDocument()
        ..cards.addAll([
          StudyCard(
            id: 'source',
            type: 'text',
            layer: 'main',
            rect: const Rect.fromLTWH(1800, 800, 500, 600),
            title: '原资料',
            text: '前😀后选中内容',
          ),
          StudyCard(
            id: 'answer',
            type: 'explanation',
            layer: 'main',
            rect: const Rect.fromLTWH(130, 60, 520, 440),
            title: '协议卡',
            generation: 'protocol-fixture',
            blocks: [StudyBlock(type: 'paragraph', text: '引用导航测试')],
            sources: [
              StudyAnchor(
                id: 'object-source',
                objectId: 'source',
                kind: 'text',
                workspaceId: 'fixture',
                start: 3,
                end: 6,
                quote: '后选中',
              ),
            ],
          ),
        ]);
      final key = await open(t, FixtureApi(d.json()));
      final before = key.currentState!.document!.camera;
      await t.tap(find.textContaining('TEXT · 后选中'));
      await t.pumpAndSettle();
      expect(key.currentState!.document!.camera, isNot(before));
      // It lands like a citation: the passage lit, and a way back.
      expect(
        key.currentState!.document!.cards
            .singleWhere((c) => c.id == 'source')
            .text,
        '前😀后选中内容',
      );
      expect(find.text('原资料'), findsOneWidget);
      expect(key.currentState!.returnFromSource(), isTrue);
      await t.pumpAndSettle();
      expect(key.currentState!.document!.camera, before);
      await t.pumpWidget(const SizedBox());
      await t.pumpAndSettle();
    },
  );
}
