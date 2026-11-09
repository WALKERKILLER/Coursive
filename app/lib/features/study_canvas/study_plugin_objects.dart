import 'dart:convert';
import 'dart:math' as math;
import 'dart:ui';

import '../plugins/canvas_fallback.dart';

/// The object shape this build reads and writes
/// (`pluginfabric.CanvasSchemaVersion`).
const canvasSchemaVersion = 1;

/// The limits of `pluginfabric.CanvasObject`, kept equal so an object the
/// canvas commits is an object the daemon saves.
const _maxText = 800000,
    _maxData = 256 << 10,
    _maxAnchors = 32,
    _maxCoordinate = 1e9,
    _minSize = 24.0,
    _maxSize = 100000.0,
    _maxLectureMs = 7 * 24 * 3600 * 1000,
    _maxPages = 5000,
    _maxStrokes = 500,
    _fallbackSummary = 280;

/// The objects one plugin batch may write or delete.
const canvasMaxBatch = 200;

final _idRE = RegExp(r'^[A-Za-z0-9][A-Za-z0-9._:-]{0,99}$');
final _typeRE = RegExp(r'^[a-z][a-z0-9-]{0,31}(?:\.[a-z][a-z0-9-]{0,31})+$');
final _pluginRE = RegExp(r'^[a-z][a-z0-9-]{1,31}$');
final _versionRE = RegExp(r'^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$');

/// One rule a canvas object breaks, with the same code and path as the
/// daemon's `pluginfabric.Problem`.
class CanvasProblem {
  const CanvasProblem(this.code, this.path);
  final String code, path;
  @override
  String toString() => '$code at $path';
}

/// Every problem in a raw `CanvasObject`, in the order the daemon reports them
/// (`pluginfabric.CanvasObject.Validate`). The canvas commits only an object
/// with none, so a save never fails on what a plugin wrote. The shared cases
/// in daemon/internal/pluginfabric/testdata/canvas keep both sides equal.
List<CanvasProblem> validateCanvasObject(Object? raw, [String at = '']) {
  final out = <CanvasProblem>[];
  void add(String code, String name) =>
      out.add(CanvasProblem(code, at.isEmpty ? name : '$at.$name'));
  final o = raw is Map ? raw : const {};

  final id = o['id'];
  if (id == null || id == '') {
    add('id-missing', 'id');
  } else if (id is! String || !_idRE.hasMatch(id)) {
    add('id-invalid', 'id');
  }
  final type = o['type'];
  if (type == null || type == '') {
    add('type-missing', 'type');
  } else if (type is! String || !_typeRE.hasMatch(type)) {
    add('type-invalid', 'type');
  }
  final schema = o['schemaVersion'];
  if (schema == null || schema == 0) {
    add('schema-missing', 'schemaVersion');
  } else if (schema is! int || schema < 0 || schema > canvasSchemaVersion) {
    add('schema-unsupported', 'schemaVersion');
  }

  final g = o['geometry'] is Map ? o['geometry'] as Map : const {};
  for (final (name, lo, hi) in [
    ('x', -_maxCoordinate, _maxCoordinate),
    ('y', -_maxCoordinate, _maxCoordinate),
    ('w', _minSize, _maxSize),
    ('h', _minSize, _maxSize),
  ]) {
    final v = g[name] ?? 0;
    if (v is! num || !v.isFinite || v < lo || v > hi) {
      add('geometry-invalid', 'geometry.$name');
    }
  }
  final rotation = g['rotation'] ?? 0;
  if (rotation is! int || rotation < 0 || rotation > 3) {
    add('geometry-invalid', 'geometry.rotation');
  }

  final content = o['content'] is Map ? o['content'] as Map : const {};
  final title = content['title'];
  if (title is! String || title.trim().isEmpty) {
    add('content-invalid', 'content.title');
  } else if (title.runes.length > 240) {
    add('content-invalid', 'content.title');
  }
  final text = content['text'] ?? '';
  if (text is! String || utf8.encode(text).length > _maxText) {
    add('content-invalid', 'content.text');
  }
  final data = content['data'];
  if (data != null &&
      (data is! Map || utf8.encode(jsonEncode(data)).length > _maxData)) {
    add('content-invalid', 'content.data');
  }

  final origin = o['createdBy'] is Map ? o['createdBy'] as Map : const {};
  final plugin = origin['pluginId'];
  if (plugin == null || plugin == '') {
    add('origin-missing', 'createdBy.pluginId');
  } else if (plugin is! String || !_pluginRE.hasMatch(plugin)) {
    add('origin-invalid', 'createdBy.pluginId');
  }
  final version = origin['pluginVersion'] ?? '';
  if (version is! String ||
      (version.isNotEmpty && !_versionRE.hasMatch(version))) {
    add('origin-invalid', 'createdBy.pluginVersion');
  }

  final revision = o['revision'];
  if (revision is! int || revision < 1) add('revision-invalid', 'revision');

  final policy = o['interactionPolicy'] is Map
      ? o['interactionPolicy'] as Map
      : const {};
  final mode = policy['mode'];
  if (mode == null || mode == '') {
    add('interaction-missing', 'interactionPolicy.mode');
  } else if (!const {'fixed', 'select', 'move', 'edit'}.contains(mode)) {
    add('interaction-invalid', 'interactionPolicy.mode');
  }
  if (policy['stylusHitTest'] == true && mode == 'fixed') {
    add('interaction-invalid', 'interactionPolicy.stylusHitTest');
  }

  final anchors = o['sourceAnchors'] ?? const [];
  if (anchors is! List) {
    add('anchor-kind', 'sourceAnchors');
    return out;
  }
  if (anchors.length > _maxAnchors) add('anchor-limit', 'sourceAnchors');
  for (var i = 0; i < anchors.length; i++) {
    final a = anchors[i] is Map ? anchors[i] as Map : const {};
    _validateAnchor(a, (code, name) => add(code, 'sourceAnchors[$i].$name'));
  }
  return out;
}

void _validateAnchor(Map a, void Function(String, String) add) {
  // A field that belongs to another kind is reported rather than ignored.
  bool has(String name) => switch (a[name]) {
    null || '' || 0 => false,
    List l => l.isNotEmpty,
    _ => true,
  };
  void reject(List<String> names) {
    for (final n in names) {
      if (has(n)) add('anchor-invalid', n);
    }
  }

  bool validId(Object? v) => v is String && _idRE.hasMatch(v);
  void span(int minLength) {
    final start = a['startMs'] ?? 0, end = a['endMs'] ?? 0;
    if (start is! int ||
        end is! int ||
        start < 0 ||
        end < start ||
        end > _maxLectureMs ||
        end - start < minLength) {
      add('anchor-invalid', 'startMs');
    }
  }

  int page(String name) =>
      a[name] is int ? a[name] as int : (has(name) ? -1 : 0);
  switch (a['kind']) {
    case 'lecture':
      if (!validId(a['lectureId'])) add('anchor-invalid', 'lectureId');
      span(0);
      reject(['file', 'page', 'pageEnd', 'text', 'strokeIds']);
    case 'transcript':
      if (!validId(a['lectureId'])) add('anchor-invalid', 'lectureId');
      span(1);
      final text = a['text'];
      if (text is! String || text.trim().isEmpty || text.runes.length > 8000) {
        add('anchor-invalid', 'text');
      }
      reject(['file', 'page', 'pageEnd', 'strokeIds']);
    case 'pdf':
      if (!_validPath(a['file'])) add('anchor-invalid', 'file');
      final first = page('page'), last = page('pageEnd');
      if (first < 1 || first > _maxPages) add('anchor-invalid', 'page');
      if (last != 0 && (last < first || last > _maxPages)) {
        add('anchor-invalid', 'pageEnd');
      }
      reject(['lectureId', 'startMs', 'endMs', 'text', 'strokeIds']);
    case 'courseware':
      if (!_validPath(a['file'])) add('anchor-invalid', 'file');
      reject([
        'lectureId',
        'startMs',
        'endMs',
        'page',
        'pageEnd',
        'text',
        'strokeIds',
      ]);
    case 'stroke':
      if (has('lectureId') && !validId(a['lectureId'])) {
        add('anchor-invalid', 'lectureId');
      }
      final strokes = a['strokeIds'] is List
          ? a['strokeIds'] as List
          : const [];
      if (strokes.isEmpty || strokes.length > _maxStrokes) {
        add('anchor-invalid', 'strokeIds');
      }
      for (var i = 0; i < strokes.length; i++) {
        if (!validId(strokes[i])) add('anchor-invalid', 'strokeIds[$i]');
      }
      reject(['file', 'page', 'pageEnd', 'startMs', 'endMs', 'text']);
    default:
      add('anchor-kind', 'kind');
  }
}

/// A relative path inside the course folder.
bool _validPath(Object? p) {
  if (p is! String ||
      p.isEmpty ||
      p.length > 512 ||
      p.startsWith('/') ||
      p.contains(r'\') ||
      p.contains('\x00')) {
    return false;
  }
  return p.split('/').every((s) => s.isNotEmpty && s != '.' && s != '..');
}

/// A plugin object on the study canvas (`pluginfabric.CanvasObject`). It keeps
/// the JSON it was read from, so a field this build does not draw survives a
/// save, and it is immutable, so an undo snapshot can share it.
class StudyPluginObject {
  StudyPluginObject._(this._raw);

  /// Reads a stored or proposed object; a problem throws [FormatException]
  /// naming the rule.
  factory StudyPluginObject.fromJson(Object? raw) {
    final problems = validateCanvasObject(raw);
    if (problems.isNotEmpty) {
      throw FormatException('插件对象无效：${problems.first}');
    }
    return StudyPluginObject._(
      jsonDecode(jsonEncode(raw)) as Map<String, dynamic>,
    );
  }

  final Map<String, dynamic> _raw;

  String get id => _raw['id'] as String;
  String get type => _raw['type'] as String;
  int get revision => _raw['revision'] as int;
  Map get _content => _raw['content'] as Map;
  String get title => (_content['title'] as String).trim();
  String get text => (_content['text'] as String? ?? '').trim();
  String get pluginId => (_raw['createdBy'] as Map)['pluginId'] as String;
  List<Map> get anchors => [
    for (final a in _raw['sourceAnchors'] as List? ?? const []) a as Map,
  ];

  Rect get rect {
    final g = _raw['geometry'] as Map;
    num v(String k) => g[k] as num? ?? 0;
    return Rect.fromLTWH(
      v('x').toDouble(),
      v('y').toDouble(),
      v('w').toDouble(),
      v('h').toDouble(),
    );
  }

  int get quarterTurns => (_raw['geometry'] as Map)['rotation'] as int? ?? 0;

  /// Where the object came from, in one line (`pluginfabric.FallbackFor`).
  String get source {
    final parts = [for (final a in anchors.take(3)) _anchorText(a)];
    final text = parts.join('；');
    return anchors.length > 3 ? '$text 等 ${anchors.length} 处' : text;
  }

  /// The view a canvas shows when the owning plugin is missing.
  CanvasFallbackModel get fallback {
    final summary = text.runes.length > _fallbackSummary
        ? '${String.fromCharCodes(text.runes.take(_fallbackSummary))}…'
        : text;
    return CanvasFallbackModel(
      id: id,
      type: type,
      pluginId: pluginId,
      title: title,
      source: source,
      summary: summary,
      schemaVersion: _raw['schemaVersion'] as int,
      revision: revision,
      data: {...?(_content['data'] as Map?)?.cast<String, dynamic>()},
    );
  }

  /// The stored object with its source anchors, as the user exports it; the
  /// caller gets its own copy.
  Map<String, dynamic> json() =>
      jsonDecode(jsonEncode(_raw)) as Map<String, dynamic>;
}

String _anchorText(Map a) {
  String clock(int ms) {
    final s = math.max(0, ms ~/ 1000);
    String two(int v) => v.toString().padLeft(2, '0');
    return s >= 3600
        ? '${s ~/ 3600}:${two(s ~/ 60 % 60)}:${two(s % 60)}'
        : '${two(s ~/ 60)}:${two(s % 60)}';
  }

  String span() {
    final start = a['startMs'] as int? ?? 0, end = a['endMs'] as int? ?? 0;
    if (start == 0 && end == 0) return '';
    if (end == 0) return ' 从 ${clock(start)}';
    return ' ${clock(start)}–${clock(end)}';
  }

  final page = a['page'] as int? ?? 0, pageEnd = a['pageEnd'] as int? ?? 0;
  return switch (a['kind']) {
    'lecture' => '课程视频${span()}',
    'transcript' => '字幕${span()}',
    'pdf' => pageEnd > page ? 'PDF 第 $page–$pageEnd 页' : 'PDF 第 $page 页',
    'courseware' => '课件 ${'${a['file']}'.split('/').last}',
    'stroke' => '手写笔迹 ${(a['strokeIds'] as List).length} 笔',
    _ => '${a['kind']}',
  };
}

/// A plugin batch the canvas checked and can commit as one undo entry
/// (`pluginfabric.CanvasBatch`). [revision] is the plugin-object revision it
/// was checked against; a commit after another change is refused.
class StudyPluginBatch {
  StudyPluginBatch._({
    required this.pluginId,
    required this.revision,
    required this.label,
    required this.created,
    required this.updated,
    required this.deleted,
  });

  final String pluginId, label;
  final int revision;
  final List<StudyPluginObject> created, updated, deleted;

  /// Overwriting or deleting what is already on the canvas needs the user's
  /// approval after a preview; a create is undoable and commits directly.
  bool get needsApproval => updated.isNotEmpty || deleted.isNotEmpty;

  /// Reads [raw] for [pluginId] against the current [objects] and [revision].
  /// Any problem throws [FormatException] and nothing is staged.
  static StudyPluginBatch check(
    String pluginId,
    Object? raw,
    Map<String, StudyPluginObject> objects,
    int revision, {
    required bool Function(String id) taken,
  }) {
    if (raw is! Map) throw const FormatException('画布批次应为对象');
    final writes = raw['objects'] ?? const [],
        deletes = raw['delete'] ?? const [];
    if (writes is! List || deletes is! List) {
      throw const FormatException('画布批次的 objects 和 delete 应为列表');
    }
    if (writes.isEmpty && deletes.isEmpty) {
      throw const FormatException('批次没有任何对象改动');
    }
    if (writes.length > canvasMaxBatch || deletes.length > canvasMaxBatch) {
      throw const FormatException('画布对象超过限制');
    }
    if (raw['revision'] != revision) {
      throw FormatException(
        '画布已改变：批次读到版本 ${raw['revision']}，当前版本 $revision，请重新读取后再试',
      );
    }
    final seen = <String>{};
    final created = <StudyPluginObject>[], updated = <StudyPluginObject>[];
    for (final w in writes) {
      final o = StudyPluginObject.fromJson(w);
      if (!seen.add(o.id)) throw FormatException('对象 ${o.id} 在同一批次里重复');
      if (o.pluginId != pluginId) {
        throw FormatException('插件只能写入自己的对象：${o.id}');
      }
      final old = objects[o.id];
      if (old == null) {
        if (taken(o.id)) throw FormatException('对象标识 ${o.id} 已被画布占用');
        if (o.revision != 1) {
          throw FormatException('新对象 ${o.id} 的 revision 应为 1');
        }
        created.add(o);
      } else {
        if (old.pluginId != pluginId) {
          throw FormatException('插件只能修改自己的对象：${o.id}');
        }
        if (o.revision != old.revision + 1) {
          throw FormatException(
            '对象 ${o.id} 已改变：读到版本 ${o.revision - 1}，当前版本 ${old.revision}',
          );
        }
        updated.add(o);
      }
    }
    final deleted = <StudyPluginObject>[];
    for (final id in deletes) {
      final old = objects['$id'];
      if (!seen.add('$id')) throw FormatException('对象 $id 在同一批次里既写入又删除');
      if (old == null) throw FormatException('要删除的对象 $id 已不存在');
      if (old.pluginId != pluginId) {
        throw FormatException('插件只能删除自己的对象：$id');
      }
      deleted.add(old);
    }
    final label = raw['label'];
    return StudyPluginBatch._(
      pluginId: pluginId,
      revision: revision,
      label: label is String ? label : '',
      created: created,
      updated: updated,
      deleted: deleted,
    );
  }
}
