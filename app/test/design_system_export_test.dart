import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// The MyGo client draws its own window, so it cannot inherit this app's
/// theme: it reads the palette from `app-mygo/design/palette.json`, which this
/// test writes. The file is the contract, and this test is what keeps it
/// honest — it fails when the generated palette no longer matches the theme
/// here, so the two clients cannot drift apart in colour.
///
///   (cd app && flutter test test/design_system_export_test.dart)
///   COURSIVE_EXPORT_DESIGN=1 flutter test test/design_system_export_test.dart
const _palettePath = '../app-mygo/design/palette.json';

/// The seed and scheme settings the app starts with (lib/theme/prefs.dart).
const _seed = Color(0xFF3F6CB4);

Map<String, String> _scheme(Brightness brightness) {
  final cs = ColorScheme.fromSeed(
    seedColor: _seed,
    brightness: brightness,
    dynamicSchemeVariant: DynamicSchemeVariant.tonalSpot,
  );
  String hex(Color c) =>
      '#${c.toARGB32().toRadixString(16).padLeft(8, '0').toUpperCase()}';
  return {
    'primary': hex(cs.primary),
    'onPrimary': hex(cs.onPrimary),
    'primaryContainer': hex(cs.primaryContainer),
    'onPrimaryContainer': hex(cs.onPrimaryContainer),
    'secondary': hex(cs.secondary),
    'onSecondary': hex(cs.onSecondary),
    'secondaryContainer': hex(cs.secondaryContainer),
    'onSecondaryContainer': hex(cs.onSecondaryContainer),
    'tertiary': hex(cs.tertiary),
    'onTertiary': hex(cs.onTertiary),
    'tertiaryContainer': hex(cs.tertiaryContainer),
    'onTertiaryContainer': hex(cs.onTertiaryContainer),
    'error': hex(cs.error),
    'onError': hex(cs.onError),
    'errorContainer': hex(cs.errorContainer),
    'onErrorContainer': hex(cs.onErrorContainer),
    'background': hex(cs.surface),
    'onBackground': hex(cs.onSurface),
    'surface': hex(cs.surface),
    'onSurface': hex(cs.onSurface),
    'surfaceVariant': hex(cs.surfaceContainerHighest),
    'onSurfaceVariant': hex(cs.onSurfaceVariant),
    'outline': hex(cs.outline),
    'outlineVariant': hex(cs.outlineVariant),
    'shadow': hex(cs.shadow),
    'scrim': hex(cs.scrim),
    'inverseSurface': hex(cs.inverseSurface),
    'inverseOnSurface': hex(cs.onInverseSurface),
    'surfaceTint': hex(cs.surfaceTint),
    'surfaceContainerLowest': hex(cs.surfaceContainerLowest),
    'surfaceContainerLow': hex(cs.surfaceContainerLow),
    'surfaceContainer': hex(cs.surfaceContainer),
    'surfaceContainerHigh': hex(cs.surfaceContainerHigh),
    'surfaceContainerHighest': hex(cs.surfaceContainerHighest),
  };
}

String _document() {
  final doc = {
    'seed': '#${_seed.toARGB32().toRadixString(16).padLeft(8, '0').toUpperCase()}',
    'light': _scheme(Brightness.light),
    'dark': _scheme(Brightness.dark),
  };
  return '${const JsonEncoder.withIndent('  ').convert(doc)}\n';
}

void main() {
  test('the MyGo palette matches this theme', () {
    final file = File(_palettePath);
    final generated = _document();
    if (Platform.environment['COURSIVE_EXPORT_DESIGN'] == '1') {
      file.parent.createSync(recursive: true);
      file.writeAsStringSync(generated);
      return;
    }
    expect(file.existsSync(), isTrue,
        reason: 'run with COURSIVE_EXPORT_DESIGN=1 to write $_palettePath');
    expect(file.readAsStringSync(), generated,
        reason: 'the MyGo palette is stale; run COURSIVE_EXPORT_DESIGN=1');
  });
}
