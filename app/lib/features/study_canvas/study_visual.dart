import 'dart:convert';
import 'dart:math' as math;
import 'dart:typed_data';
import 'dart:ui' as ui;

import '../../l10n/l10n.dart';

/// A scanned page's text hash can be empty on every page. Include its visual
/// identity so a crop cannot silently follow a different page or selection.
String studyVisualFingerprint(Map source) =>
    jsonEncode([source['image_path'], source['anchor']]);

bool studyVisualBindingChanged(String id, Map before, Map after) {
  if (id == 'selected-ink')
    return jsonEncode(before['selection']) != jsonEncode(after['selection']);
  final object = id == 'lecture-video'
      ? 'lecture'
      : id.replaceFirst(RegExp(r'^object-'), '');
  if (!(after['objects'] as List? ?? []).contains(object)) return true;
  Object? scope(Map q) {
    final selection = (q['selections'] as Map?)?[object] ?? q['selection'];
    return [
      selection is Map && selection['object_id'] == object ? selection : null,
      (q['object_pages'] as Map?)?[object],
      (q['video_ranges'] as Map?)?[object],
    ];
  }

  return jsonEncode(scope(before)) != jsonEncode(scope(after));
}

/// Rebuild a lost attachment from the approved source, independent of viewport,
/// sidebar occlusion and conversation lifetime. Never substitute the whole page.
Future<Map<String, dynamic>> restoreStudyVisual(
  Map source,
  Future<Uint8List> Function(String path) load, {
  int quarterTurns = 0,
  ui.Size? cardSize,
}) async {
  final path = source['image_path'];
  final region = (source['anchor'] as Map?)?['region'];
  if (path is! String ||
      path.isEmpty ||
      region is! List ||
      region.length != 4) {
    throw FormatException(l10n.studyVisualExpired);
  }
  if (region.any((v) => v is! num)) {
    throw FormatException(l10n.studyVisualRegionInvalid);
  }
  final r = region.map((v) => (v as num).toDouble()).toList();
  if (r.any((v) => !v.isFinite) ||
      r[0] < 0 ||
      r[1] < 0 ||
      r[2] <= 0 ||
      r[3] <= 0 ||
      r[0] + r[2] > 1.000001 ||
      r[1] + r[3] > 1.000001) {
    throw FormatException(l10n.studyVisualRegionInvalid);
  }
  final bytes = await load(path);
  final buffer = await ui.ImmutableBuffer.fromUint8List(bytes);
  ui.ImageDescriptor? descriptor;
  ui.Codec? codec;
  ui.Image? image;
  try {
    descriptor = await ui.ImageDescriptor.encoded(buffer);
    if (descriptor.width * descriptor.height > 24000000) {
      throw FormatException(l10n.studyVisualDimensionsTooLarge);
    }
    codec = await descriptor.instantiateCodec();
    image = (await codec.getNextFrame()).image;
    final turns = quarterTurns % 4;
    // Legacy PDF/image regions are normalized against the entire card, whose
    // title and contain-fit padding are not pixels in the original asset.
    final originalSize = cardSize == null
        ? ui.Size(image.width.toDouble(), image.height.toDouble())
        : turns.isOdd
        ? ui.Size(cardSize.height, cardSize.width)
        : cardSize;
    final area = cardSize == null
        ? ui.Offset.zero & originalSize
        : ui.Rect.fromLTWH(
            0,
            36,
            originalSize.width,
            math.max(1, originalSize.height - 36),
          );
    final fit = math.min(area.width / image.width, area.height / image.height);
    final destination = ui.Rect.fromCenter(
      center: area.center,
      width: image.width * fit,
      height: image.height * fit,
    );
    final width = turns.isOdd ? originalSize.height : originalSize.width;
    final height = turns.isOdd ? originalSize.width : originalSize.height;
    final crop = ui.Rect.fromLTWH(
      r[0] * width,
      r[1] * height,
      r[2] * width,
      r[3] * height,
    );
    for (final edge in [1200.0, 900.0, 600.0, 320.0]) {
      final scale = math.min(1.0, edge / math.max(crop.width, crop.height));
      final w = math.max(1, (crop.width * scale).round());
      final h = math.max(1, (crop.height * scale).round());
      final recorder = ui.PictureRecorder();
      final canvas = ui.Canvas(recorder);
      canvas.drawColor(const ui.Color(0xffffffff), ui.BlendMode.src);
      canvas.scale(w / crop.width, h / crop.height);
      canvas.translate(-crop.left, -crop.top);
      // Map the original image into the same clockwise orientation as its card.
      switch (turns) {
        case 1:
          canvas.translate(width, 0);
          canvas.rotate(math.pi / 2);
        case 2:
          canvas.translate(width, height);
          canvas.rotate(math.pi);
        case 3:
          canvas.translate(0, height);
          canvas.rotate(-math.pi / 2);
      }
      canvas.drawImageRect(
        image,
        ui.Rect.fromLTWH(0, 0, image.width.toDouble(), image.height.toDouble()),
        destination,
        ui.Paint()..filterQuality = ui.FilterQuality.medium,
      );
      final picture = recorder.endRecording();
      final cropped = await picture.toImage(w, h);
      ByteData? encoded;
      try {
        encoded = await cropped.toByteData(format: ui.ImageByteFormat.png);
      } finally {
        cropped.dispose();
        picture.dispose();
      }
      if (encoded != null && encoded.lengthInBytes <= 450000) {
        return {
          'source_id': source['id'],
          'mime_type': 'image/png',
          'data': base64Encode(
            encoded.buffer.asUint8List(
              encoded.offsetInBytes,
              encoded.lengthInBytes,
            ),
          ),
        };
      }
    }
    throw FormatException(l10n.studyVisualPayloadTooLarge);
  } finally {
    image?.dispose();
    codec?.dispose();
    descriptor?.dispose();
    buffer.dispose();
  }
}
