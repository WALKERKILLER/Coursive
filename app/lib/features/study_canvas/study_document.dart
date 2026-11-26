import 'dart:collection';
import 'dart:convert';
import 'dart:math' as math;
import 'dart:ui';

import 'package:flutter/foundation.dart';
import 'package:flutter/painting.dart' show HSLColor;

import '../../l10n/l10n.dart';
import '../../core/ink_erase.dart';
import 'study_content.dart';
import 'study_brush.dart';
import 'study_plugin_objects.dart';
export 'study_content.dart';
export 'study_brush.dart';
export 'study_plugin_objects.dart';

part 'study_stroke_index.dart';

String studyId() =>
    '${DateTime.now().microsecondsSinceEpoch.toRadixString(36)}-${_nextId++}';
int _nextId = 0;
double _n(dynamic v, [double fallback = 0]) =>
    v is num && v.isFinite ? v.toDouble() : fallback;

class StudyLayer {
  StudyLayer(this.id, this.name, {this.visible = true, this.locked = false});
  final String id;
  String name;
  bool visible, locked;
  StudyLayer copy() => StudyLayer(id, name, visible: visible, locked: locked);
  Map<String, dynamic> json() => {
    'id': id,
    'name': name,
    'visible': visible,
    'locked': locked,
  };
}

class StudyCard {
  StudyCard({
    required this.id,
    required this.type,
    required this.layer,
    required this.rect,
    this.title = '',
    this.text = '',
    this.path = '',
    this.source = '',
    this.documentId = '',
    this.page = 0,
    this.pageCount = 0,
    this.pageAspect = .75,
    this.textBoxes = const [],
    this.generation = '',
    this.conversationId = '',
    this.sourceLectureId = '',
    this.originalPath = '',
    this.pdfColumn = '',
    this.pdfPaged = false,
    this.collapsed = false,
    this.positionLocked = false,
    this.expandedH = 0,
    this.quarterTurns = 0,
    this.rotation = 0,
    this.contentScale = 1,
    List<StudyBlock>? blocks,
    List<StudyAnchor>? sources,
    Map<String, String>? responses,
  }) : responses = responses ?? {},
       blocks = blocks ?? [],
       sources = sources ?? [];
  final String id, type;
  String layer, title, text, path, source;
  Rect rect;
  String documentId, generation, conversationId, sourceLectureId, originalPath;
  String pdfColumn;
  bool pdfPaged;
  bool collapsed;
  bool positionLocked;
  double expandedH;
  // A folded aside is drawn at its title width. The saved frame keeps the
  // unfolded layout width so opening an older canvas needs no migration.
  double? unfoldedLayoutWidth;

  Rect withLayoutWidth(double width) {
    final worldWidth = width * contentScale;
    final delta = (worldWidth - contentRect.width) / 2;
    final angle = rotation + quarterTurns * math.pi / 2;
    return Rect.fromCenter(
      center: rect.center + Offset(delta * math.cos(angle), delta * math.sin(angle)),
      width: quarterTurns.isEven ? worldWidth : rect.width,
      height: quarterTurns.isEven ? rect.height : worldWidth,
    );
  }
  int page, pageCount, quarterTurns;
  double rotation, contentScale;
  double pageAspect;
  Offset mapWorld(Offset world, double angle) {
    final v = world - rect.center;
    return rect.center +
        Offset(
          v.dx * math.cos(angle) - v.dy * math.sin(angle),
          v.dx * math.sin(angle) + v.dy * math.cos(angle),
        );
  }

  List<Offset> get corners => [
    for (final p in [
      rect.topLeft,
      rect.topRight,
      rect.bottomRight,
      rect.bottomLeft,
    ])
      mapWorld(p, rotation),
  ];
  Rect get visualBounds {
    if (rotation == 0) return rect;
    final points = corners;
    return points
        .skip(1)
        .fold(
          Rect.fromPoints(points.first, points.first),
          (r, p) => r.expandToInclude(Rect.fromPoints(p, p)),
        );
  }

  Rect get contentRect => quarterTurns.isEven
      ? rect
      : Rect.fromCenter(
          center: rect.center,
          width: rect.height,
          height: rect.width,
        );
  Offset contentPoint(Offset p) =>
      mapWorld(p, -rotation - quarterTurns * math.pi / 2);
  bool containsWorld(Offset p) => rect.contains(mapWorld(p, -rotation));
  List<Map> textBoxes;
  List<StudyBlock> blocks;
  List<StudyAnchor> sources;
  Map<String, String> responses;
  bool get lessonNote => type == 'text' && generation == 'lesson-note' && RegExp(r'^lesson-[0-9a-f]{32}$').hasMatch(documentId);
  bool get linkedNote => type == 'text' && generation == 'linked-note' && documentId.isNotEmpty;
  bool get markdownReader =>
      type == 'text' &&
      (lessonNote || linkedNote || pinnedAnswer ||
          source.toLowerCase().endsWith('.md') ||
          source.toLowerCase().endsWith('.markdown'));
  bool get passageAside => type == 'conversation' && generation == 'markdown-aside';
  // For asides, a zero saved expanded height means fit the content.
  bool get fitsAsideHeight => passageLinked && expandedH == 0;
  bool get passageNote => type == 'text' && generation == 'markdown-annotation';
  bool get passageLinked => passageAside || passageNote;
  bool get onCanvas => type != 'conversation' || passageAside;
  bool get duplicable => type != 'lecture' && type != 'conversation';
  bool get readerCard => generated || markdownReader;
  bool get readingSurface =>
      readerCard || passageAside || type == 'text' || type == 'pdf' || type == 'pdfdoc';
  bool get pinnedAnswer =>
      (type == 'text' || type == 'practice') &&
      generation.startsWith('canvas-turn:');
  bool get handout => type == 'practice' && pinnedAnswer;
  bool get generated =>
      const {'note', 'explanation', 'knowledge', 'practice'}.contains(type);
  String get contentStamp => jsonEncode({
    "type": type,
    "text": text,
    "path": path,
    "source": source,
    "page": page,
    "blocks": blocks.map((b) => b.json()).toList(),
    "responses": responses,
  });
  String get contentText {
    final value = generated ? blocks.map((b) => b.plainText).join('\n') : text;
    final keys = responses.keys.toList()..sort();
    return responses.isEmpty
        ? value
        : '$value\n用户作答\n${keys.map((k) => '$k: ${responses[k]}').join('\n')}';
  }

  factory StudyCard.fromJson(Map m) => StudyCard(
    id: '${m['id']}',
    type: '${m['type']}',
    layer: '${m['layer']}',
    rect: Rect.fromLTWH(
      _n(m['x']),
      _n(m['y']),
      _n(m['w'], 600),
      _n(m['h'], 440),
    ),
    title: '${m['title'] ?? ''}',
    text: '${m['text'] ?? ''}',
    path: '${m['path'] ?? ''}',
    source: '${m['source'] ?? ''}',
    documentId: '${m['document_id'] ?? m['source'] ?? ''}',
    page: (m['page'] as num? ?? 0).toInt(),
    pageCount: (m['page_count'] as num? ?? 0).toInt(),
    pageAspect: _n(m['page_aspect'], .75),
    textBoxes: (m['text_boxes'] as List? ?? []).cast<Map>(),
    generation: '${m['generation'] ?? ''}',
    conversationId: '${m['conversation_id'] ?? ''}',
    sourceLectureId: '${m['source_lecture_id'] ?? ''}',
    originalPath: '${m['original_path'] ?? ''}',
    pdfColumn: '${m['pdf_column'] ?? ''}',
    pdfPaged: m['pdf_paged'] == true,
    collapsed: m['collapsed'] == true,
    positionLocked: m['position_locked'] == true,
    expandedH: _n(m['expanded_h']),
    rotation: _n(m['rotation']),
    contentScale: _n(m['content_scale'], 1),
    quarterTurns: (m['quarter_turns'] as num? ?? 0).toInt(),
    responses: Map<String, String>.from(m['responses'] as Map? ?? {}),
    blocks: [
      for (final b in m['blocks'] as List? ?? []) StudyBlock.fromJson(b as Map),
    ],
    sources: [
      for (final a in m['sources'] as List? ?? [])
        StudyAnchor.fromJson(a as Map),
    ],
  );
  StudyCard copy() => StudyCard(
    id: id,
    type: type,
    layer: layer,
    rect: rect,
    title: title,
    text: text,
    path: path,
    source: source,
    documentId: documentId,
    page: page,
    pageCount: pageCount,
    pageAspect: pageAspect,
    textBoxes: textBoxes,
    generation: generation,
    conversationId: conversationId,
    sourceLectureId: sourceLectureId,
    originalPath: originalPath,
    pdfColumn: pdfColumn,
    pdfPaged: pdfPaged,
    collapsed: collapsed,
    positionLocked: positionLocked,
    expandedH: expandedH,
    quarterTurns: quarterTurns,
    rotation: rotation,
    contentScale: contentScale,
    blocks: [for (final block in blocks) block.copy()],
    sources: List.of(sources),
    responses: Map.of(responses),
  )..unfoldedLayoutWidth = unfoldedLayoutWidth;
  Map<String, dynamic> json() {
    final frame = collapsed && unfoldedLayoutWidth != null
        ? withLayoutWidth(unfoldedLayoutWidth!)
        : rect;
    return {
    'id': id,
    'type': type,
    'layer': layer,
    'x': frame.left,
    'y': frame.top,
    'w': frame.width,
    'h': frame.height,
    'title': title,
    if (!linkedNote) 'text': text,
    'path': path,
    'source': source,
    'document_id': documentId,
    'page': page,
    'page_count': pageCount,
    'page_aspect': pageAspect,
    'text_boxes': textBoxes,
    'generation': generation,
    'conversation_id': conversationId,
    'source_lecture_id': sourceLectureId,
    'original_path': originalPath,
    if (pdfColumn.isNotEmpty) 'pdf_column': pdfColumn,
    if (pdfPaged) 'pdf_paged': true,
    'collapsed': collapsed,
    if (positionLocked) 'position_locked': true,
    'expanded_h': expandedH,
    if (rotation != 0) 'rotation': rotation,
    if (contentScale != 1) 'content_scale': contentScale,
    if (quarterTurns != 0) 'quarter_turns': quarterTurns,
    'responses': responses,
    'blocks': blocks.map((b) => b.json()).toList(),
    'sources': sources.map((a) => a.json()).toList(),
    };
  }
}

class InkPoint {
  const InkPoint(this.p, this.pressure);
  final Offset p;
  final double pressure;
  static InkPoint lerp(InkPoint a, InkPoint b, double t) => InkPoint(
    Offset.lerp(a.p, b.p, t)!,
    a.pressure + (b.pressure - a.pressure) * t,
  );
  List<double> json() => [p.dx, p.dy, pressure];
}

class StudyStroke {
  StudyStroke({
    required this.id,
    required this.layer,
    required this.points,
    required this.color,
    required this.width,
    this.tool = 'pen',
    this.anchor = '',
    this.page = 0,
    String? gesture,
    this.brush,
  }) : gesture = gesture ?? id;
  final String id, layer, tool, anchor, gesture;
  final int page;
  final List<InkPoint> points;
  final int color;
  final double width;
  final StudyBrush? brush;
  late final Rect bounds = points.fold<Rect>(
    Rect.fromCircle(center: points.first.p, radius: width),
    (r, p) => r.expandToInclude(Rect.fromCircle(center: p.p, radius: width)),
  );
  // Pictures are bounded to 64 segments. Long input never regenerates its prefix.
  // CustomPainter's repaint boundary: https://api.flutter.dev/flutter/rendering/CustomPainter-class.html
  Picture? _picture, _darkPicture;

  /// [dark] draws dark ink light for dark paper (see [inkOnDark]).
  void paint(Canvas canvas, {bool dark = false}) {
    Picture record(int c) {
      final recorder = PictureRecorder();
      paintInk(
        Canvas(recorder),
        points,
        c,
        width,
        tool,
        brush: brush,
        darkPaper: dark,
      );
      return recorder.endRecording();
    }

    final picture = dark
        ? (_darkPicture ??= record(inkOnDark(color, tool)))
        : (_picture ??= record(color));
    canvas.drawPicture(picture);
  }

  Map<String, dynamic> json() => {
    'id': id,
    'layer': layer,
    'anchor': anchor,
    'page': page,
    'gesture': gesture,
    if (brush != null) 'brush': brush!.json(),
    'color': color,
    'width': width,
    'tool': tool,
    'points': points.map((p) => p.json()).toList(),
  };
  factory StudyStroke.fromJson(Map m) => StudyStroke(
    id: '${m['id']}',
    layer: '${m['layer']}',
    anchor: '${m['anchor'] ?? ''}',
    page: (m['page'] as num? ?? 0).toInt(),
    gesture: m['gesture'] as String?,
    brush: m['brush'] is Map ? StudyBrush.fromJson(m['brush'] as Map) : null,
    color: (m['color'] as num).toInt(),
    width: _n(m['width'], 3),
    tool: '${m['tool']}',
    points: [
      for (final p in m['points'] as List)
        InkPoint(Offset(_n(p[0]), _n(p[1])), _n(p[2], .5)),
    ],
  );
}

/// The colour ink takes on dark paper: dark ink turns light with its hue
/// kept, so notes written on white paper stay legible in dark mode. Saved
/// colours never change; highlighters and light ink stay as they are.
int inkOnDark(int color, [String tool = 'pen']) {
  if (tool == 'highlighter') return color;
  final hsl = HSLColor.fromColor(Color(color));
  if (hsl.lightness >= .55) return color;
  return hsl
      .withLightness((1 - hsl.lightness).clamp(.74, .92))
      .toColor()
      .toARGB32();
}

/// Whether ink anchored to [type] sits on a printed page (PDF, image) that
/// keeps its light background in dark mode.
bool studyOnPrintedPage(String? type) =>
    type == 'pdf' || type == 'pdfdoc' || type == 'image';

void paintInk(
  Canvas canvas,
  List<InkPoint> points,
  int color,
  double width,
  String tool, {
  bool darkPaper = false,
  StudyBrush? brush,
}) {
  if (points.isEmpty) return;
  final paint = Paint()
    ..color = Color(color)
        .withValues(alpha: brush?.opacity ?? (tool == 'highlighter' ? .3 : 1))
    ..blendMode = tool == 'highlighter' && !darkPaper
        ? BlendMode.darken
        : BlendMode.srcOver
    ..strokeCap = brush?.squareTip == true ? StrokeCap.square : StrokeCap.round
    ..strokeJoin = StrokeJoin.round;
  if (points.length == 1) {
    if (tool == 'highlighter' && brush?.squareTip == true) {
      canvas.drawRect(
        Rect.fromCenter(center: points.first.p, width: width, height: width),
        paint,
      );
    } else {
      canvas.drawCircle(points.first.p, width * .5, paint);
    }
    return;
  }
  if (tool == 'highlighter') {
    final path = Path()..moveTo(points.first.p.dx, points.first.p.dy);
    for (final point in points.skip(1)) {
      path.lineTo(point.p.dx, point.p.dy);
    }
    canvas.drawPath(
      path,
      paint
        ..style = PaintingStyle.stroke
        ..strokeWidth = width,
    );
    return;
  }
  // A constant-width opaque round pen has the same outline along a single
  // round-joined path. Submit it once, including when replaying legacy ink.
  final fixedWidth =
      brush?.family == 'ballpoint' ||
      brush?.pressure == 0 ||
      points.every((p) => p.pressure == points.first.pressure);
  if (fixedWidth &&
      brush?.family != 'pencil' &&
      brush?.squareTip != true &&
      (brush?.opacity ?? 1) == 1 &&
      points.length > 2) {
    paint.strokeWidth =
        width *
        (brush?.radiusScale(points.first.pressure) ??
            (.35 + .65 * points.first.pressure));
    final path = Path()..moveTo(points.first.p.dx, points.first.p.dy);
    var hasLength = false;
    for (final p in points.skip(1)) {
      hasLength |= p.p != points.first.p;
      path.lineTo(p.p.dx, p.p.dy);
    }
    if (hasLength) {
      canvas.drawPath(path, paint..style = PaintingStyle.stroke);
    } else {
      canvas.drawCircle(points.first.p, paint.strokeWidth / 2, paint);
    }
    return;
  }
  for (var i = 1; i < points.length; i++) {
    paint.strokeWidth =
        width *
        (brush != null
            ? brush.radiusScale(
                (points[i].pressure + points[i - 1].pressure) / 2,
              )
            : tool == 'highlighter'
            ? 1
            : .35 + .65 * (points[i].pressure + points[i - 1].pressure) / 2);
    canvas.drawLine(points[i - 1].p, points[i].p, paint);
    if (brush?.family == 'pencil') {
      final delta = points[i].p - points[i - 1].p;
      if (delta.distance > 0) {
        final normal =
            Offset(-delta.dy, delta.dx) /
            delta.distance *
            paint.strokeWidth *
            .22;
        final grain = Paint()
          ..color = Color(color).withValues(alpha: brush!.opacity * .18)
          ..strokeWidth = math.max(.25, width * .08);
        canvas.drawLine(points[i - 1].p + normal, points[i].p + normal, grain);
      }
    }
  }
}

class _StudySnapshot {
  _StudySnapshot(StudyDocument d)
    : appliedRuns = Set.of(d.appliedRuns),
      layers = d.layers.map((a) => a.copy()).toList(),
      cards = d.cards.map((a) => a.copy()).toList(),
      connections = [
        for (final link in d.connections)
          StudyConnection(link.id, link.from, link.to, link.label),
      ],
      strokes = d._ink.snapshot(),
      pluginObjects = List.of(d.pluginObjects);
  final Set<String> appliedRuns;
  final List<StudyLayer> layers;
  final List<StudyCard> cards;
  final List<StudyStroke> strokes;
  final List<StudyConnection> connections;
  final List<StudyPluginObject> pluginObjects;
}

class StudyDocument extends ChangeNotifier {
  static const currentFormatVersion = 8;
  static const _positionLockFormat = 8;
  int revision = 0, edit = 0, formatVersion = 3;

  /// The plugin-object revision a plugin batch is checked against. It moves on
  /// every change to [pluginObjects], undo and redo included, and is not saved.
  int pluginRevision = 0;
  String baseHash = "";
  Set<String> appliedRuns = {};
  Offset camera = const Offset(40, 70);
  double zoom = .75;
  String activeLayer = 'main';
  List<StudyLayer> layers = [StudyLayer('main', l10n.studyDefaultLayer)];
  List<StudyCard> cards = [];
  _IndexedStrokes _ink = _IndexedStrokes(const []);
  List<StudyStroke> get strokes => _ink;
  set strokes(List<StudyStroke> value) => _ink = _IndexedStrokes(value);
  int get inkPointCount => _ink.pointCount;
  Iterable<StudyStroke> inkIn(Rect area) =>
      _ink.intersecting(area).map((i) => _ink[i]);
  Iterable<StudyStroke> inkOnCard(String id) => _ink.anchored(id);
  List<StudyConnection> connections = [];

  /// Persistent plugin output (`pluginfabric.CanvasObject`), saved as
  /// `plugin_objects` in format 5. The canvas draws them and never hit-tests
  /// them, so they cannot take a stroke or a selection.
  List<StudyPluginObject> pluginObjects = [];
  final _undo = <_StudySnapshot>[], _redo = <_StudySnapshot>[];
  bool get canUndo => _undo.isNotEmpty;
  bool get canRedo => _redo.isNotEmpty;
  bool visible(String id) => layers.any((a) => a.id == id && a.visible);
  bool strokeVisible(StudyStroke s) {
    if (!visible(s.layer)) return false;
    if (s.anchor.isEmpty) return true;
    final card = cards.where((c) => c.id == s.anchor).firstOrNull;
    return card == null ||
        (!card.collapsed && (s.page == 0 || card.page == s.page));
  }

  bool editable(String id) =>
      layers.any((a) => a.id == id && a.visible && !a.locked);
  bool canTransformCard(StudyCard card) =>
      !card.positionLocked &&
      editable(card.layer) &&
      (!layers.any((l) => l.locked) ||
          !inkOnCard(card.id).any(
            (s) => layers.any((l) => l.id == s.layer && l.locked),
          ));
  factory StudyDocument.fromJson(Map m) {
    final version = m['version'];
    if (version is! int || version < 1 || version > currentFormatVersion) {
      throw FormatException(l10n.studyUnknownVersion);
    }
    final d = StudyDocument();
    d.formatVersion = math.max(3, version);
    d.revision = (m['revision'] as num).toInt();
    d.baseHash = m['base_hash'] as String? ?? '';
    d.appliedRuns = (m['applied_runs'] as List? ?? []).cast<String>().toSet();
    final c = m['camera'] as Map;
    d.camera = Offset(_n(c['x']), _n(c['y']));
    d.zoom = _n(c['z'], 1).clamp(.05, 8);
    d.layers = [
      for (final a in m['layers'] as List)
        StudyLayer(
          '${a['id']}',
          '${a['name']}',
          visible: a['visible'] == true,
          locked: a['locked'] == true,
        ),
    ];
    d.activeLayer = d.layers.first.id;
    d.cards = [
      for (final a in m['objects'] as List) StudyCard.fromJson(a as Map),
    ];
    d.connections = [
      for (final a in m['connections'] as List? ?? [])
        StudyConnection.fromJson(a as Map),
    ];
    // Older files may contain long strokes; normalize their render/erase work into bounded chunks.
    d.strokes = [
      for (final a in m['strokes'] as List)
        ...chunks(StudyStroke.fromJson(a as Map)),
    ];
    d.pluginObjects = [
      for (final a in m['plugin_objects'] as List? ?? [])
        StudyPluginObject.fromJson(a),
    ];
    // A candidate wrote position locks before assigning them a format.
    if (d.cards.any((card) => card.positionLocked)) {
      d.formatVersion = math.max(d.formatVersion, _positionLockFormat);
    }
    return d;
  }
  StudyDocument();
  Map<String, dynamic> json() => {
    'version': math.max(
      formatVersion,
      cards.any((c) => c.positionLocked)
          ? _positionLockFormat
          : cards.any((c) => c.contentScale != 1)
          ? 7
          : cards.any(
              (c) =>
                  c.rotation != 0 || (c.quarterTurns != 0 && c.type != 'image'),
            )
          ? 6
          : pluginObjects.isNotEmpty
          ? 5
          : cards.any((c) => c.quarterTurns != 0)
          ? 4
          : 3,
    ),
    'revision': revision,
    if (baseHash.isNotEmpty) 'base_hash': baseHash,
    'applied_runs': appliedRuns.toList(),
    'camera': {'x': camera.dx, 'y': camera.dy, 'z': zoom},
    'layers': layers.map((a) => a.json()).toList(),
    'objects': cards.map((a) => a.json()).toList(),
    'strokes': strokes.map((a) => a.json()).toList(),
    'connections': connections.map((a) => a.json()).toList(),
    if (pluginObjects.isNotEmpty)
      'plugin_objects': pluginObjects.map((a) => a.json()).toList(),
  };
  static List<StudyStroke> chunks(StudyStroke s) {
    // A translucent gesture must composite once, including after reopening.
    if (s.tool == 'highlighter' || s.points.length <= 65) return [s];
    return [
      for (var i = 0; i < s.points.length - 1; i += 64)
        StudyStroke(
          id: '${s.id}-$i',
          layer: s.layer,
          points: s.points.sublist(i, math.min(i + 65, s.points.length)),
          color: s.color,
          width: s.width,
          tool: s.tool,
          anchor: s.anchor,
          page: s.page,
          gesture: s.gesture,
          brush: s.brush,
        ),
    ];
  }

  void checkpoint() {
    _undo.add(_StudySnapshot(this));
    if (_undo.length > 40) _undo.removeAt(0);
    _redo.clear();
  }

  void changed() {
    edit++;
    notifyListeners();
  }

  void _restore(_StudySnapshot s) {
    appliedRuns = Set.of(s.appliedRuns);
    layers = s.layers.map((a) => a.copy()).toList();
    cards = s.cards.map((a) => a.copy()).toList();
    strokes = s.strokes;
    connections = [
      for (final link in s.connections)
        StudyConnection(link.id, link.from, link.to, link.label),
    ];
    if (s.pluginObjects.length != pluginObjects.length ||
        Iterable.generate(pluginObjects.length)
            .any((i) => !identical(s.pluginObjects[i], pluginObjects[i])))
      pluginRevision++;
    pluginObjects = List.of(s.pluginObjects);
    if (!layers.any((a) => a.id == activeLayer)) activeLayer = layers.first.id;
    changed();
  }

  void undo() {
    if (!canUndo) return;
    _redo.add(_StudySnapshot(this));
    _restore(_undo.removeLast());
  }

  void redo() {
    if (!canRedo) return;
    _undo.add(_StudySnapshot(this));
    _restore(_redo.removeLast());
  }

  void addStroke(StudyStroke s) {
    if (s.points.isEmpty) return;
    checkpoint();
    strokes.addAll(chunks(s));
    changed();
  }

  /// Checks a plugin's `CanvasBatch` against the current objects and
  /// [pluginRevision] without changing anything; see [commitPluginBatch].
  StudyPluginBatch checkPluginBatch(String pluginId, Object? raw) {
    final batch = StudyPluginBatch.check(
      pluginId,
      raw,
      {for (final o in pluginObjects) o.id: o},
      pluginRevision,
      taken: (id) => cards.any((c) => c.id == id),
    );
    if (cards.length +
            pluginObjects.length +
            batch.created.length -
            batch.deleted.length >
        2000)
      throw const FormatException('画布对象数量超过限制');
    return batch;
  }

  /// Commits a checked batch as one undo entry. A batch checked before
  /// another change to the plugin objects is refused whole.
  void commitPluginBatch(StudyPluginBatch batch) {
    if (batch.revision != pluginRevision)
      throw const FormatException('画布已改变，请重新读取后再试');
    final updated = {for (final o in batch.updated) o.id: o};
    final deleted = {for (final o in batch.deleted) o.id};
    checkpoint();
    pluginObjects = [
      for (final o in pluginObjects)
        if (!deleted.contains(o.id)) updated[o.id] ?? o,
      ...batch.created,
    ];
    connections.removeWhere(
      (c) => deleted.contains(c.from) || deleted.contains(c.to),
    );
    pluginRevision++;
    changed();
  }

  /// Removes one plugin object at the user's request, as one undo entry.
  void removePluginObject(String id) {
    if (!pluginObjects.any((o) => o.id == id)) return;
    checkpoint();
    pluginObjects = [
      for (final o in pluginObjects)
        if (o.id != id) o,
    ];
    connections.removeWhere((c) => c.from == id || c.to == id);
    pluginRevision++;
    changed();
  }

  /// Validate every staged operation before creating one undo entry.
  List<String> applyAgentBatch(
    Map result,
    int expectedEdit,
    Rect viewport, {
    bool allowUpdates = false,
    Map<String, String>? expectedContents,
  }) {
    if (appliedRuns.contains('${result['id']}')) return [];
    if (expectedContents != null) {
      for (final entry in expectedContents.entries) {
        if (entry.key.startsWith('ink/')) {
          final stroke = strokes
              .where((s) => s.id == entry.key.substring(4))
              .firstOrNull;
          if (stroke == null || jsonEncode(stroke.json()) != entry.value)
            throw const FormatException('引用的笔迹已改变，请重新生成');
          continue;
        }
        final card = cards.where((c) => c.id == entry.key).firstOrNull;
        if (card == null || card.contentStamp != entry.value)
          throw const FormatException("引用资料或待修改内容已改变，请重新生成");
      }
    }
    if (expectedContents == null && edit != expectedEdit)
      throw const FormatException('画布在生成期间已改变，请重新生成以免覆盖你的编辑');
    if (!editable(activeLayer)) throw const FormatException('请先选择可编辑图层');
    final ops = result['operations'] as List;
    if (ops.length > 20) throw const FormatException('单次最多 20 个画布操作');
    final anchors = {
      for (final a in result['sources'] as List)
        '${a['id']}': StudyAnchor.fromJson(a as Map),
    };
    final allowedEdits = (result['editable_ids'] as List? ?? [])
        .cast<String>()
        .toSet();
    final next = cards.map((c) => c.copy()).toList();
    final links = connections
        .map((c) => StudyConnection.fromJson(c.json()))
        .toList();
    final created = <String>[];
    List<StudyAnchor> citations(Map op) => [
      for (final id in op['source_ids'] as List? ?? [])
        anchors['$id'] ?? (throw const FormatException('生成物引用了本次没有提供的来源')),
    ];
    List<StudyBlock> blocks(Map op) {
      final values = op['blocks'] as List? ?? [];
      if (values.isEmpty || values.length > 32)
        throw const FormatException('卡片内容结构无效');
      final out = [for (final v in values) StudyBlock.fromJson(v as Map)];
      for (final b in out) {
        if (!const {
              'heading',
              'paragraph',
              'list',
              'steps',
              'table',
              'code',
              'math',
              'exercise',
            }.contains(b.type) ||
            b.plainText.length > 12000 ||
            b.items.length > 30 ||
            b.rows.length > 30 ||
            b.columns.length > 8 ||
            (b.type == 'table' &&
                (b.columns.isEmpty ||
                    b.rows.any((r) => r.length != b.columns.length)))) {
          throw const FormatException('卡片内容超过限制或结构无效');
        }
      }
      return out;
    }

    Rect place() {
      for (var i = 0; i < 150; i++) {
        final p =
            viewport.topLeft +
            Offset(30 + (i % 3) * 560.0, 30 + (i ~/ 3) * 480.0);
        final r = Rect.fromLTWH(p.dx, p.dy, 520, 440);
        if (!next.any(
          (c) => visible(c.layer) && c.rect.inflate(16).overlaps(r),
        ))
          return r;
      }
      throw const FormatException('附近空间不足，请移动到画布空白处重试');
    }

    for (final raw in ops) {
      final op = raw as Map, kind = op['op'];
      if (kind == 'create') {
        final id = '${op['id']}',
            title = '${op['title']}',
            type = '${op['kind']}';
        if (next.any((c) => c.id == id) ||
            !const {
              'note',
              'explanation',
              'knowledge',
              'practice',
            }.contains(type) ||
            title.length > 240)
          throw const FormatException('生成卡片标识或类型无效');
        next.add(
          StudyCard(
            id: id,
            type: type,
            layer: activeLayer,
            rect: place(),
            title: title,
            blocks: blocks(op),
            sources: citations(op),
            generation: '${result['id']}',
          ),
        );
        created.add(id);
      } else if (kind == 'update') {
        if (!allowUpdates) throw const FormatException('修改已有卡片前需要确认预览');
        final c = next.where((c) => c.id == op['object_id']).firstOrNull;
        if (c == null ||
            !c.generated ||
            !editable(c.layer) ||
            !allowedEdits.contains(c.id))
          throw const FormatException('只能修改本次选中的可编辑知识卡片');
        final title = '${op['title']}';
        if (title.length > 240) throw const FormatException('标题过长');
        final cited = citations(op)
            .expand((a) => a.objectId == c.id ? c.sources : [a])
            .toList();
        c
          ..title = title
          ..blocks = blocks(op)
          ..responses = {}
          ..sources = cited;
      } else if (kind == 'connect') {
        final from = '${op['from']}',
            to = '${op['to']}',
            label = '${op['label']}';
        if (from == to ||
            !next.any((c) => c.id == from && editable(c.layer)) ||
            !next.any((c) => c.id == to && editable(c.layer)) ||
            label.length > 240)
          throw const FormatException('关联对象或标签无效');
        links.add(StudyConnection('${op['id']}', from, to, label));
      } else if (kind != 'focus') {
        throw const FormatException('未知画布操作');
      }
    }
    if (next.length > 2000 || links.length > 4000)
      throw const FormatException('画布对象数量超过限制');
    if (ops.any((o) => o['op'] != 'focus')) {
      checkpoint();
      appliedRuns.add('${result['id']}');
      if (appliedRuns.length > 256) appliedRuns.remove(appliedRuns.first);
      cards = next;
      connections = links;
      changed();
    }
    return created;
  }

  void transformCards(Set<String> ids, Rect from, Rect to) {
    final movable = cards
        .where((c) => ids.contains(c.id) && canTransformCard(c))
        .map((c) => c.id)
        .toSet();
    final sx = to.width / from.width, sy = to.height / from.height;
    Offset move(Offset p) => Offset(
      to.left + (p.dx - from.left) * sx,
      to.top + (p.dy - from.top) * sy,
    );
    for (final c in cards.where((a) => movable.contains(a.id))) {
      final tl = move(c.rect.topLeft), br = move(c.rect.bottomRight);
      c.rect = Rect.fromPoints(tl, br);
    }
    strokes = [
      for (final s in strokes)
        if (movable.contains(s.anchor))
          StudyStroke(
            id: s.id,
            layer: s.layer,
            points: [for (final p in s.points) InkPoint(move(p.p), p.pressure)],
            color: s.color,
            width: s.width * math.sqrt(sx * sy),
            tool: s.tool,
            anchor: s.anchor,
            page: s.page,
            gesture: s.gesture,
            brush: s.brush,
          )
        else
          s,
    ];
    changed();
  }

  /// Cards and attached ink share the selection pivot. A locked member blocks the group.
  bool canRotateSelection(Set<String> cardsIds, Set<String> inkIds) {
    final selected = cards.where((c) => cardsIds.contains(c.id)).toList();
    return (selected.isNotEmpty || inkIds.isNotEmpty) &&
        selected.every(
          (card) =>
              canTransformCard(card) &&
              (!layers.any((layer) => !layer.visible) ||
                  inkOnCard(card.id).every((stroke) => editable(stroke.layer))),
        ) &&
        (inkIds.isEmpty ||
            !layers.any((layer) => layer.locked || !layer.visible) ||
            strokes
                .where((stroke) => inkIds.contains(stroke.id))
                .every((stroke) => editable(stroke.layer)));
  }

  void rotateSelection(
    Set<String> cardIds,
    Set<String> inkIds,
    Offset pivot, {
    double? radians,
    bool recordUndo = true,
  }) {
    if (!canRotateSelection(cardIds, inkIds)) return;
    final quarter = radians == null;
    final angle = radians ?? math.pi / 2;
    if (!angle.isFinite || angle == 0) return;
    final cs = math.cos(angle), sn = math.sin(angle);
    Offset turn(Offset p) {
      final v = p - pivot;
      return pivot +
          (quarter
              ? Offset(-v.dy, v.dx)
              : Offset(cs * v.dx - sn * v.dy, sn * v.dx + cs * v.dy));
    }

    if (recordUndo) checkpoint();
    for (final c in cards.where((c) => cardIds.contains(c.id))) {
      c.rect = Rect.fromCenter(
        center: turn(c.rect.center),
        width: quarter ? c.rect.height : c.rect.width,
        height: quarter ? c.rect.width : c.rect.height,
      );
      if (quarter) {
        c.quarterTurns = (c.quarterTurns + 1) % 4;
      } else {
        c.rotation = math.atan2(
          math.sin(c.rotation + angle),
          math.cos(c.rotation + angle),
        );
      }
      formatVersion = math.max(
        formatVersion,
        quarter && c.type == 'image' ? 4 : 6,
      );
    }
    strokes = [
      for (final s in strokes)
        if (inkIds.contains(s.id) || cardIds.contains(s.anchor))
          StudyStroke(
            id: s.id,
            layer: s.layer,
            anchor: s.anchor,
            page: s.page,
            gesture: s.gesture,
            tool: s.tool,
            brush: s.brush,
            color: s.color,
            width: s.width,
            points: [for (final p in s.points) InkPoint(turn(p.p), p.pressure)],
          )
        else
          s,
    ];
    changed();
  }

  bool erase(Offset a, Offset b, double radius) {
    final hit = Rect.fromPoints(a, b).inflate(radius);
    final replacements = <(int, List<StudyStroke>)>[];
    for (final i in _ink.intersecting(hit)) {
      final s = strokes[i];
      if (!editable(s.layer) || !strokeVisible(s)) continue;
      final eraser = InkEraser(
        [a, b],
        const Size(1, 1),
        radius + s.width / 2,
        'partial',
      );
      final fragments = eraser.clipSamples(
        s.points,
        position: (sample) => sample.p,
        interpolate: InkPoint.lerp,
        bounds: s.bounds,
      );
      if (fragments.length == 1 && identical(fragments.first, s.points)) {
        continue;
      }
      replacements.add((i, [
        for (final fragment in fragments)
          StudyStroke(
            id: studyId(),
            layer: s.layer,
            points: fragment,
            color: s.color,
            width: s.width,
            tool: s.tool,
            anchor: s.anchor,
            page: s.page,
            gesture: s.gesture,
            brush: s.brush,
          ),
      ]));
    }
    if (replacements.isEmpty) return false;
    // Splits stay at their original paint position.
    if (replacements.length == 1) {
      final (i, pieces) = replacements.single;
      _ink.replaceRange(i, i + 1, pieces);
    } else {
      // A sweep through stacked ink can replace many strokes at once.
      final next = <StudyStroke>[];
      var start = 0;
      for (final (i, pieces) in replacements) {
        next.addAll(strokes.getRange(start, i));
        next.addAll(pieces);
        start = i + 1;
      }
      next.addAll(strokes.getRange(start, strokes.length));
      strokes = next;
    }
    changed();
    return true;
  }

  bool eraseWhole(Offset a, Offset b, double radius) {
    final hit = Rect.fromPoints(a, b).inflate(radius), gestures = <String>{};
    for (final s in inkIn(hit)) {
      if (!editable(s.layer) || !strokeVisible(s))
        continue;
      if (s.points.length == 1 &&
          _distanceToSegment(s.points.first.p, a, b) <= radius + s.width / 2)
        gestures.add(s.gesture);
      for (var i = 1; i < s.points.length; i++) {
        if (_segmentsNear(
          s.points[i - 1].p,
          s.points[i].p,
          a,
          b,
          radius + s.width / 2,
        )) {
          gestures.add(s.gesture);
          break;
        }
      }
    }
    if (gestures.isEmpty) return false;
    strokes.removeWhere(
      (s) => editable(s.layer) && gestures.contains(s.gesture),
    );
    changed();
    return true;
  }

  bool eraseLasso(List<Offset> polygon) {
    if (polygon.length < 3) return false;
    var bounds = Rect.fromPoints(polygon.first, polygon.first);
    for (final p in polygon)
      bounds = bounds.expandToInclude(Rect.fromPoints(p, p));
    bool inside(Offset p) {
      var yes = false;
      for (var i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
        final a = polygon[i], b = polygon[j];
        if ((a.dy > p.dy) != (b.dy > p.dy) &&
            p.dx < (b.dx - a.dx) * (p.dy - a.dy) / (b.dy - a.dy) + a.dx)
          yes = !yes;
      }
      return yes;
    }

    final hit = <String>{};
    for (final s in inkIn(bounds)) {
      if (editable(s.layer) &&
          strokeVisible(s) &&
          s.points.any((p) => inside(p.p)))
        hit.add(s.gesture);
    }
    if (hit.isEmpty) return false;
    strokes.removeWhere((s) => editable(s.layer) && hit.contains(s.gesture));
    changed();
    return true;
  }

  /// Select whole physical strokes, including sparse segments crossing the loop.
  Set<String> strokesInPolygon(List<Offset> polygon) {
    if (polygon.length < 3) return {};
    var region = Rect.fromPoints(polygon.first, polygon.first);
    for (final p in polygon)
      region = region.expandToInclude(Rect.fromPoints(p, p));
    final loop = Path()..addPolygon(polygon, true);
    final hit = <String>{};
    for (final s in inkIn(region).where(strokeVisible)) {
      final intersection = Path.combine(
        PathOperation.intersect,
        loop,
        studyStrokeOutline(s),
      );
      if (!intersection.getBounds().isEmpty) hit.add(s.gesture);
    }
    if (hit.isEmpty) return {};
    return strokes
        .where((s) => hit.contains(s.gesture) && strokeVisible(s))
        .map((s) => s.id)
        .toSet();
  }

  /// Caller owns the gesture checkpoint; source IDs survive edits and undo.
  void transformInk(
    Set<String> ids,
    Rect from,
    Rect to, {
    int? color,
    double? width,
  }) {
    final sx = to.width / math.max(.001, from.width);
    final sy = to.height / math.max(.001, from.height);
    var touched = false;
    strokes = [
      for (final s in strokes)
        if (ids.contains(s.id) && editable(s.layer))
          (() {
            touched = true;
            return StudyStroke(
              id: s.id,
              layer: s.layer,
              anchor: s.anchor,
              page: s.page,
              gesture: s.gesture,
              tool: s.tool,
              brush: s.brush,
              color: color ?? s.color,
              width: (width ?? s.width * math.sqrt(sx * sy)).clamp(.2, 80),
              points: [
                for (final p in s.points)
                  InkPoint(
                    Offset(
                      to.left + (p.p.dx - from.left) * sx,
                      to.top + (p.p.dy - from.top) * sy,
                    ),
                    p.pressure,
                  ),
              ],
            );
          })()
        else
          s,
    ];
    if (touched) changed();
  }

  String contextText(Set<String> ids) {
    final out = StringBuffer('画布材料（文字内容，不含手写识别）：\n');
    for (final c in cards.where(
      (a) => visible(a.layer) && (ids.isEmpty || ids.contains(a.id)),
    )) {
      out.writeln('${c.title}\n${c.text}');
      if (out.length > 12000) break;
    }
    final value = out.toString();
    return value.length <= 12000 ? value : value.substring(0, 12000);
  }
}

double _distanceToSegment(Offset p, Offset a, Offset b) {
  final v = b - a;
  final t = v.distanceSquared == 0
      ? 0.0
      : (((p - a).dx * v.dx + (p - a).dy * v.dy) / v.distanceSquared).clamp(
          0.0,
          1.0,
        );
  return (p - a - v * t).distance;
}

bool _segmentsNear(Offset a, Offset b, Offset c, Offset d, double r) {
  double cross(Offset p, Offset q) => p.dx * q.dy - p.dy * q.dx;
  final v = b - a, w = d - c, denominator = cross(v, w);
  if (denominator.abs() > 1e-8) {
    final t = cross(c - a, w) / denominator, u = cross(c - a, v) / denominator;
    if (t >= 0 && t <= 1 && u >= 0 && u <= 1) return true;
  }
  return math.min(
        math.min(_distanceToSegment(a, c, d), _distanceToSegment(b, c, d)),
        math.min(_distanceToSegment(c, a, b), _distanceToSegment(d, a, b)),
      ) <=
      r;
}

/// Filled hit geometry matches the segment widths and caps used by paintInk.
/// Chunk grouping remains a separate selection policy in strokesInPolygon.
Path studyStrokeOutline(StudyStroke stroke) {
  final outline = Path();
  final points = stroke.points;
  if (points.isEmpty) return outline;
  if (points.length == 1) {
    return outline..addOval(
      Rect.fromCircle(center: points.first.p, radius: stroke.width / 2),
    );
  }
  for (var i = 1; i < points.length; i++) {
    final a = points[i - 1], b = points[i];
    final pressure = (a.pressure + b.pressure) / 2;
    final radius =
        stroke.width /
        2 *
        (stroke.tool == 'highlighter'
            ? 1
            : stroke.brush?.radiusScale(pressure) ?? (.35 + .65 * pressure));
    final delta = b.p - a.p;
    if (delta.distance == 0) {
      outline.addOval(Rect.fromCircle(center: a.p, radius: radius));
      continue;
    }
    final tangent = delta / delta.distance;
    final normal = Offset(-tangent.dy, tangent.dx) * radius;
    final square = stroke.brush?.squareTip == true;
    final start =
        a.p -
        (square && (stroke.tool != 'highlighter' || i == 1)
            ? tangent * radius
            : Offset.zero);
    final end =
        b.p +
        (square && (stroke.tool != 'highlighter' || i == points.length - 1)
            ? tangent * radius
            : Offset.zero);
    // Clockwise contours share a winding with addOval.
    outline.addPolygon([
      start - normal,
      end - normal,
      end + normal,
      start + normal,
    ], true);
    if (!square || stroke.tool == 'highlighter' && i > 1)
      outline.addOval(Rect.fromCircle(center: a.p, radius: radius));
    if (!square || stroke.tool == 'highlighter' && i < points.length - 1)
      outline.addOval(Rect.fromCircle(center: b.p, radius: radius));
  }
  return outline;
}

enum StudySelectionKind { ink, text, document, video, other }

StudySelectionKind studyCardSelectionKind(StudyCard card) =>
    switch (card.type) {
      'text' ||
      'note' ||
      'explanation' ||
      'knowledge' ||
      'practice' => StudySelectionKind.text,
      'pdfdoc' || 'pdf' || 'image' => StudySelectionKind.document,
      'lecture' || 'video' => StudySelectionKind.video,
      _ => StudySelectionKind.other,
    };
