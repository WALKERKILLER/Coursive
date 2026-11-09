import 'dart:convert';
import 'dart:io';

import 'package:coursive/core/plugins.dart';
import 'package:flutter_test/flutter_test.dart';

/// The canonical plugin-manifest fixtures (docs/schemas/plugin-manifest.v3.schema.json).
/// The daemon and the assistant host validate the same files, so a manifest the
/// app can read is a manifest they accepted.
Directory fixturesDir() {
  for (final candidate in [
    '${Directory.current.path}/../daemon/internal/pluginfabric/testdata',
    '${Directory.current.path}/daemon/internal/pluginfabric/testdata',
  ]) {
    final dir = Directory(candidate);
    if (dir.existsSync()) return dir;
  }
  fail('找不到 daemon/internal/pluginfabric/testdata：请从仓库根目录或 app/ 运行 flutter test');
}

const _groups = {'navigation', 'pages', 'slots', 'tools', 'views', 'actions', 'settings', 'sources'};

List<Map<String, dynamic>> _cases(Directory dir) {
  final index = jsonDecode(File('${dir.path}/cases.json').readAsStringSync()) as Map;
  expect(index['schema'], 'coursive/plugin-manifest@3');
  return [for (final c in index['cases'] as List) (c as Map).cast<String, dynamic>()];
}

Map<String, dynamic> _manifest(Directory dir, String file) =>
    (jsonDecode(File('${dir.path}/$file').readAsStringSync()) as Map).cast<String, dynamic>();

bool _insidePackage(Object? path) {
  if (path is! String) return false;
  var p = path;
  while (p.startsWith('./')) {
    p = p.substring(2);
  }
  if (p.isEmpty || p.startsWith('/') || p.contains(r'\')) return false;
  if (p.length > 1 && p[1] == ':') return false;
  return p.split('/').every((s) => s.isNotEmpty && s != '.' && s != '..');
}

void main() {
  test('the app reads every shared manifest fixture', () {
    final dir = fixturesDir();
    final cases = _cases(dir);
    expect(cases.where((c) => c['code'] == null), isNotEmpty, reason: '至少有一个通过的清单');
    expect(cases.where((c) => c['code'] != null), isNotEmpty, reason: '至少有一个被拒绝的清单');
    for (final c in cases) {
      final manifest = _manifest(dir, '${c['file']}');
      expect(manifest.containsKey('coursive') || c['code'] == 'manifest-missing', isTrue, reason: '${c['file']}');
    }
  });

  test('a valid canonical manifest keeps the shape the plugin store consumes', () {
    final dir = fixturesDir();
    for (final c in _cases(dir).where((c) => c['code'] == null)) {
      final file = '${c['file']}';
      final manifest = _manifest(dir, file);
      final plugin = (manifest['coursive'] as Map).cast<String, dynamic>();
      expect(plugin['id'], isA<String>(), reason: file);
      expect(plugin['api'], isA<String>(), reason: file);
      expect(manifest['version'], isA<String>(), reason: file);

      for (final runtime in (plugin['runtime'] as List? ?? const [])) {
        final entry = (runtime as Map)['entry'];
        final paths = entry is Map ? entry.values : [entry];
        for (final path in paths) {
          expect(_insidePackage(path), isTrue, reason: '$file: $path');
        }
      }

      final contributes = (plugin['contributes'] as Map?)?.cast<String, dynamic>() ?? const {};
      expect(contributes.keys.toSet().difference(_groups), isEmpty, reason: file);

      final pluginMap = {'id': plugin['id'], 'title': plugin['title']};
      for (final e in (contributes['actions'] as List? ?? const [])) {
        final entry = PluginEntry(pluginMap, 'actions', (e as Map));
        expect(entry.id, isNotEmpty, reason: file);
        expect(entry.action, isNotEmpty, reason: file);
      }
    }
  });
}
