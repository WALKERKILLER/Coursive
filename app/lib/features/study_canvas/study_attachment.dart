import 'dart:convert';
import 'dart:typed_data';

import '../../core/api.dart';

/// A captured raster is independent of the mutable canvas. Failed uploads keep
/// these exact bytes, so retrying never silently takes a different screenshot.
class StudySnapshot {
  StudySnapshot(this.bytes, Map<String, dynamic> selection)
    : selection = jsonDecode(jsonEncode(selection)) as Map<String, dynamic>;
  final Uint8List bytes;
  final Map<String, dynamic> selection;
  Map<String, dynamic>? attachment;
  Object? error;
  bool uploading = false;
  Future<Map<String, dynamic>> upload(Api api, String lecture) async {
    if (attachment != null) return attachment!;
    uploading = true;
    error = null;
    try {
      attachment = Map<String, dynamic>.from(
        await api.uploadStudyAttachment(lecture, bytes, selection) as Map,
      );
      return attachment!;
    } catch (e) {
      error = e;
      rethrow;
    } finally {
      uploading = false;
    }
  }
}

class StudySelectionDrop {
  Future<StudySnapshot>? snapshot;
}
