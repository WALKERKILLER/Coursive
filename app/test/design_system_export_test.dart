import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// The MyGo client draws its own window, so it cannot inherit this app's
/// theme. It computes ColorScheme.fromSeed with a Go port of
/// material_color_utilities (app-mygo/internal/design/mcu), and this test
/// writes what Flutter itself computes for the seeds below, in every variant,
/// contrast and brightness the settings offer. The Go tests compare their
/// schemes with this file, so the two clients cannot drift apart in colour.
///
///   (cd app && flutter test test/design_system_export_test.dart)
///   COURSIVE_EXPORT_DESIGN=1 flutter test test/design_system_export_test.dart
const _path = '../app-mygo/internal/design/mcu/testdata/flutter_schemes.json';

/// The appearance picker's seeds (lib/theme/prefs.dart), and others that
/// reach the solver's edges: greys, a disliked yellow, pure primaries.
const _seeds = <int>[
  0xFF3F6CB4, 0xFF2E6E6A, 0xFF4F7A3A, 0xFFB0782A,
  0xFFB4443A, 0xFFB0507A, 0xFF6E5AA8, 0xFF5D6670,
  0xFF000000, 0xFFFFFFFF, 0xFF808080, 0xFFFF0000,
  0xFF00FF00, 0xFF0000FF, 0xFF8A8A00, 0xFFFFEB3B,
];

/// The variants the settings offer (kVariantLabels in lib/theme/prefs.dart).
const _variants = [
  DynamicSchemeVariant.tonalSpot,
  DynamicSchemeVariant.neutral,
  DynamicSchemeVariant.vibrant,
  DynamicSchemeVariant.expressive,
  DynamicSchemeVariant.fidelity,
  DynamicSchemeVariant.content,
  DynamicSchemeVariant.monochrome,
];

const _contrasts = <String, double>{'standard': 0, 'medium': 0.5, 'high': 1};

Map<String, Color> _scheme(ColorScheme cs) {
  return {
    'primary': cs.primary,
    'onPrimary': cs.onPrimary,
    'primaryContainer': cs.primaryContainer,
    'onPrimaryContainer': cs.onPrimaryContainer,
    'primaryFixed': cs.primaryFixed,
    'primaryFixedDim': cs.primaryFixedDim,
    'onPrimaryFixed': cs.onPrimaryFixed,
    'onPrimaryFixedVariant': cs.onPrimaryFixedVariant,
    'secondary': cs.secondary,
    'onSecondary': cs.onSecondary,
    'secondaryContainer': cs.secondaryContainer,
    'onSecondaryContainer': cs.onSecondaryContainer,
    'secondaryFixed': cs.secondaryFixed,
    'secondaryFixedDim': cs.secondaryFixedDim,
    'onSecondaryFixed': cs.onSecondaryFixed,
    'onSecondaryFixedVariant': cs.onSecondaryFixedVariant,
    'tertiary': cs.tertiary,
    'onTertiary': cs.onTertiary,
    'tertiaryContainer': cs.tertiaryContainer,
    'onTertiaryContainer': cs.onTertiaryContainer,
    'tertiaryFixed': cs.tertiaryFixed,
    'tertiaryFixedDim': cs.tertiaryFixedDim,
    'onTertiaryFixed': cs.onTertiaryFixed,
    'onTertiaryFixedVariant': cs.onTertiaryFixedVariant,
    'error': cs.error,
    'onError': cs.onError,
    'errorContainer': cs.errorContainer,
    'onErrorContainer': cs.onErrorContainer,
    'outline': cs.outline,
    'outlineVariant': cs.outlineVariant,
    'surface': cs.surface,
    'onSurface': cs.onSurface,
    'surfaceDim': cs.surfaceDim,
    'surfaceBright': cs.surfaceBright,
    'surfaceContainerLowest': cs.surfaceContainerLowest,
    'surfaceContainerLow': cs.surfaceContainerLow,
    'surfaceContainer': cs.surfaceContainer,
    'surfaceContainerHigh': cs.surfaceContainerHigh,
    'surfaceContainerHighest': cs.surfaceContainerHighest,
    'onSurfaceVariant': cs.onSurfaceVariant,
    'inverseSurface': cs.inverseSurface,
    'onInverseSurface': cs.onInverseSurface,
    'inversePrimary': cs.inversePrimary,
    'shadow': cs.shadow,
    'scrim': cs.scrim,
    'surfaceTint': cs.surfaceTint,
  };
}

String _hex(int argb) => '#${argb.toRadixString(16).padLeft(8, '0').toUpperCase()}';

/// One scheme a line, its roles in the order of the header, so the file
/// stays small enough to keep in the repository.
String _document() {
  final names = _scheme(ColorScheme.fromSeed(seedColor: Color(_seeds.first))).keys.toList();
  final lines = <String>[];
  for (final seed in _seeds) {
    for (final variant in _variants) {
      for (final contrast in _contrasts.entries) {
        for (final brightness in Brightness.values) {
          final cs = ColorScheme.fromSeed(
            seedColor: Color(seed),
            brightness: brightness,
            dynamicSchemeVariant: variant,
            contrastLevel: contrast.value,
          );
          final roles = _scheme(cs);
          lines.add(jsonEncode({
            'seed': _hex(seed),
            'variant': variant.name,
            'contrast': contrast.value,
            'dark': brightness == Brightness.dark,
            'argb': [for (final n in names) _hex(roles[n]!.toARGB32())],
          }));
        }
      }
    }
  }
  return '{"roles": ${jsonEncode(names)},\n"schemes": [\n${lines.join(',\n')}\n]}\n';
}

void main() {
  test('the MyGo colour schemes match Flutter', () {
    final file = File(_path);
    final generated = _document();
    if (Platform.environment['COURSIVE_EXPORT_DESIGN'] == '1') {
      file.parent.createSync(recursive: true);
      file.writeAsStringSync(generated);
      return;
    }
    expect(file.existsSync(), isTrue, reason: 'run with COURSIVE_EXPORT_DESIGN=1 to write $_path');
    expect(file.readAsStringSync(), generated, reason: 'the MyGo schemes are stale; run COURSIVE_EXPORT_DESIGN=1');
  });
}
