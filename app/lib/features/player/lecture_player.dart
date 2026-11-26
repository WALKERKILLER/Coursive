import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../../core/activity_tracker.dart';
import '../../core/api.dart';
import '../../core/app_state.dart';
import '../../core/models.dart';
import '../../theme/prefs.dart';
import '../../l10n/l10n.dart';

/// One video source: a remote stream or the downloaded file.
class Source {
  Source({
    required this.label,
    required this.uri,
    this.local = false,
    this.headers = const {},
  });
  final String label;
  final String uri;
  final bool local;
  final Map<String, String> headers;
}

/// Decide the initial media offset before opening the native player.
int playbackStartMs({
  int? requested,
  required int saved,
  required int duration,
  required bool autoResume,
}) {
  if (requested != null) {
    if (requested < 0) return 0;
    return duration > 0 && requested > duration ? duration : requested;
  }
  return autoResume && saved > 10000 && (duration == 0 || saved < duration - 30000)
      ? saved
      : 0;
}

/// Owns the media_kit players and everything the player UI shows.
class LecturePlayer extends ChangeNotifier implements PlayerHandle {
  Future<bool> Function(Map<String, dynamic>)? canvasCommand;
  LecturePlayer({
    required this.api,
    required this.lectureId,
    required this.prefs,
    this.initialPositionMs,
    this.opensInCanvas = false,
  }) {
    _subs.addAll([
      player.stream.position.listen((p) {
        position.value = p;
        _syncPip(p);
        if (_recoverableCodecError && playing && !_checkingDecoder) {
          unawaited(_clearRecoveredCodecError(_openRevision));
        }
      }),
      player.stream.duration.listen((d) {
        if (d > Duration.zero) duration = d;
        notifyListeners();
      }),
      player.stream.buffer.listen((b) => buffer.value = b),
      player.stream.playing.listen((v) {
        playing = v;
        v ? pip?.play() : pip?.pause();
        v ? _watch?.start() : _watch?.stop();
        notifyListeners();
      }),
      player.stream.buffering.listen((v) {
        buffering = v;
        notifyListeners();
      }),
      player.stream.rate.listen((v) {
        if (!boosting) rate = v;
        pip?.setRate(v);
        notifyListeners();
      }),
      player.stream.volume.listen((v) {
        volume = v;
        notifyListeners();
      }),
      player.stream.error.listen((e) {
        _codecCheck?.cancel();
        if (e.trim().toLowerCase() == 'could not open codec.') {
          // mpv reports each failed decoder candidate, even if the next one works.
          final revision = _openRevision;
          final at = player.state.position;
          _codecCheck = Timer(const Duration(milliseconds: 1600), () async {
            if (_disposed || revision != _openRevision) return;
            final recovered = await _hasDecodedTracks();
            if (_disposed || revision != _openRevision) return;
            if (recovered && player.state.position != at) return;
            error = e;
            _recoverableCodecError = true;
            notifyListeners();
          });
        } else {
          _openRevision++;
          _recoverableCodecError = false;
          error = e;
          notifyListeners();
        }
      }),
      player.stream.completed.listen((done) {
        if (done) save();
      }),
    ]);
    if (lectureId.isNotEmpty) _saver = Timer.periodic(const Duration(seconds: 10), (_) => save());
  }

  final Api api;
  @override
  final String lectureId;
  final UiPrefs prefs;
  final int? initialPositionMs;
  final bool opensInCanvas;

  late final _watch = lectureId.isEmpty ? null : ActivityTracker.instance?.clock(
    'watch',
    lectureId: lectureId,
  );

  final player = Player(
    configuration: const PlayerConfiguration(
      title: 'Coursive',
      bufferSize: 64 * 1024 * 1024,
    ),
  );
  late final video = VideoController(player);
  Player? pip;
  VideoController? pipVideo;

  final position = ValueNotifier(Duration.zero);
  final seekEvents = ValueNotifier<int>(0);
  final buffer = ValueNotifier(Duration.zero);

  /// Asks the side panel to show a tab ('outline', 'transcript', …).
  final panelTab = ValueNotifier<String?>(null);
  Duration duration = Duration.zero;
  bool playing = false, buffering = true, boosting = false;
  double rate = 1, volume = 100;
  String? error;

  Lecture? lecture;
  Course? course;
  LectureMedia? media;
  List<Source> sources = [];
  int sourceIdx = 0;
  int? pipIdx;
  List<Slide> slides = [];
  List<Segment> segments = [];
  List<Bookmark> bookmarks = [];
  List<Note> notes = [];
  List<Chapter> chapters = [];
  List<CoursewareFile> courseware = [];
  bool outlineBusy = false;
  String? outlineError;
  bool loading = true;
  Object? loadError, mediaError;
  bool mediaLoading = false;
  int resumedFrom = 0;

  final _subs = <StreamSubscription>[];
  Timer? _saver;
  int _lastSaved = -1;
  int _playRequest = 0;
  bool _disposed = false;

  Source? get source => sources.isEmpty ? null : sources[sourceIdx];
  @override
  int get positionMs => position.value.inMilliseconds;

  Future<void> load() async {
    loading = true;
    loadError = mediaError = null;
    mediaLoading = true;
    notifyListeners();
    try {
      final d = (await api.get('/api/lectures/$lectureId') as Map)
          .cast<String, dynamic>();
      lecture = Lecture((d['lecture'] as Map).cast<String, dynamic>());
      if (d['course'] is Map)
        course = Course((d['course'] as Map).cast<String, dynamic>());
      bookmarks = jsonList(d['bookmarks'])
          .map((e) => Bookmark((e as Map).cast<String, dynamic>()))
          .toList();
      notes = jsonList(d['notes'])
          .map((e) => Note((e as Map).cast<String, dynamic>()))
          .toList();
      chapters = jsonList(d['timeline'])
          .map((e) => Chapter((e as Map).cast<String, dynamic>()))
          .toList();
      var localLabel = '';
      for (final a in jsonList(d['assets'])) {
        if (a is Map && a['kind'] == 'video') {
          final meta = a['meta'];
          localLabel =
              (meta is Map
                      ? meta['stream']
                      : meta is String
                      ? (jsonDecode(meta) as Map)['stream']
                      : null)
                  ?.toString() ??
              '';
        }
      }
      final pb = d['playback'];
      final resumeMs = pb is Map ? (pb['position_ms'] as num? ?? 0).toInt() : 0;

      if (_disposed) return;
      if (opensInCanvas) {
        loading = false;
        notifyListeners();
        unawaited(_loadSide());
      }
      try {
        await _loadMedia(localLabel, resumeMs);
      } catch (e) {
        if (!opensInCanvas) rethrow;
        mediaError = e;
      } finally {
        mediaLoading = false;
        if (!_disposed) notifyListeners();
      }
      if (!opensInCanvas && !_disposed) unawaited(_loadSide());
    } catch (e) {
      if (_disposed) return;
      loading = false;
      loadError = e;
      notifyListeners();
    }
  }

  Future<void> _loadMedia(String localLabel, int resumeMs) async {
    media = LectureMedia(
      (await api.get('/api/lectures/$lectureId/media') as Map)
          .cast<String, dynamic>(),
    );
    sources = [
      for (final s in media!.streams)
        Source(
          label: s.label,
          uri: media!.relayed
              ? api.uri(s.url, {'token': api.token}).toString()
              : s.url,
          headers: media!.headers,
        ),
    ];
    if (media!.localPath.isNotEmpty) {
      sources.insert(
        0,
        Source(
          label: localLabel.isEmpty ? l10n.commonLocal : localLabel,
          uri: Uri.file(media!.localPath).toString(),
          local: true,
        ),
      );
    }
    if (_disposed) return;
    if (sources.isEmpty) throw StateError(l10n.playerNoVideo);
    sourceIdx = 0;
    if (media!.duration > 0) duration = Duration(seconds: media!.duration);
    loading = false;
    notifyListeners();

    final start = playbackStartMs(
      requested: initialPositionMs,
      saved: resumeMs,
      duration: duration.inMilliseconds,
      autoResume: prefs.autoResume,
    );
    resumedFrom = initialPositionMs == null ? start : 0;
    await _open(source!, Duration(milliseconds: start), play: !opensInCanvas);
    if (!_disposed) await player.setRate(prefs.defaultSpeed);
  }

  Future<void> _loadSide() async {
    try {
      final sl = jsonList(await api.get('/api/lectures/$lectureId/slides'));
      slides = sl.map((e) => Slide((e as Map).cast<String, dynamic>())).toList()
        ..sort((a, b) => a.tSec - b.tSec);
      notifyListeners();
    } catch (_) {}
    await reloadTranscript();
    await reloadCourseware();
  }

  Future<void> reloadTimeline() async {
    try {
      final r = jsonList(await api.get('/api/lectures/$lectureId/timeline'));
      chapters = r
          .map((e) => Chapter((e as Map).cast<String, dynamic>()))
          .toList();
      notifyListeners();
    } catch (_) {}
  }

  Future<void> reloadCourseware() async {
    try {
      final r = jsonList(await api.get('/api/lectures/$lectureId/courseware'));
      courseware = r
          .map((e) => CoursewareFile((e as Map).cast<String, dynamic>()))
          .toList();
      notifyListeners();
    } catch (_) {}
  }

  /// Asks the assistant host to write the outline (one model call over the
  /// whole transcript), by main stage or, when [detailed], by topic. Errors
  /// land in [outlineError].
  Future<void> generateOutline({bool detailed = false}) async {
    if (outlineBusy) return;
    outlineBusy = true;
    outlineError = null;
    notifyListeners();
    try {
      final r = (await api.post('/agent/timeline', {
        'lecture_id': lectureId,
        'detail': detailed ? 'detailed' : 'concise',
      }, const Duration(minutes: 4)) as Map).cast<String, dynamic>();
      chapters = jsonList(r['chapters'])
          .map((e) => Chapter((e as Map).cast<String, dynamic>()))
          .toList();
    } catch (e) {
      outlineError = '$e';
    } finally {
      outlineBusy = false;
      if (!_disposed) notifyListeners();
    }
  }

  Future<void> deleteOutline() async {
    chapters = [];
    notifyListeners();
    await api.delete('/api/lectures/$lectureId/timeline');
  }

  int chapterAt(int ms) {
    for (var i = chapters.length - 1; i >= 0; i--) {
      if (chapters[i].startMs <= ms) return i;
    }
    return chapters.isEmpty ? -1 : 0;
  }

  Future<void> reloadTranscript() async {
    try {
      final sg = jsonList(await api.get('/api/lectures/$lectureId/transcript'));
      segments = sg
          .map((e) => Segment((e as Map).cast<String, dynamic>()))
          .toList();
      notifyListeners();
    } catch (_) {}
  }

  Future<void> reloadNotes() async {
    try {
      final ns = jsonList(
        await api.get('/api/notes', {'lecture_id': lectureId}),
      );
      notes = ns.map((e) => Note((e as Map).cast<String, dynamic>())).toList();
      notifyListeners();
    } catch (_) {}
  }

  Timer? _codecCheck;
  int _openRevision = 0;
  bool _recoverableCodecError = false, _checkingDecoder = false;

  Future<bool> _hasDecodedTracks() async {
    final native = player.platform;
    if (native is! NativePlayer) return false;
    try {
      final tracks = player.state.tracks;
      final hasVideo = tracks.video.any((t) => t.id != 'auto' && t.id != 'no');
      final hasAudio = tracks.audio.any((t) => t.id != 'auto' && t.id != 'no');
      if (!hasVideo && !hasAudio) return false;
      final videoWidth = hasVideo ? await native.getProperty('video-dec-params/w') : '';
      final sampleRate = hasAudio ? await native.getProperty('audio-params/samplerate') : '';
      return (!hasVideo || (int.tryParse(videoWidth) ?? 0) > 0) &&
          (!hasAudio || (int.tryParse(sampleRate) ?? 0) > 0);
    } catch (_) {
      return false;
    }
  }

  Future<void> _clearRecoveredCodecError(int revision) async {
    _checkingDecoder = true;
    try {
      if (await _hasDecodedTracks() && !_disposed && revision == _openRevision && _recoverableCodecError) {
        _recoverableCodecError = false;
        error = null;
        notifyListeners();
      }
    } finally {
      _checkingDecoder = false;
    }
  }

  /// An imported recording has its own playhead and no lecture progress record.
  Future<void> openMaterial(Source material, int at) async {
    assert(lectureId.isEmpty);
    sources = [material];
    await _open(material, Duration(milliseconds: at), play: false);
  }

  Future<void> retryPlayback() async {
    final current = source;
    if (current == null) return;
    try {
      final previousRate = rate;
      await _open(current, position.value);
      await player.setRate(previousRate);
    } catch (e) {
      error = e.toString();
      notifyListeners();
    }
  }

  Future<void> _open(Source s, Duration start, {bool play = true}) async {
    _codecCheck?.cancel();
    _openRevision++;
    _recoverableCodecError = false;
    error = null;
    notifyListeners();
    // Attach video output before opening a silent video (study canvas).
    await video.platform.future;
    if (_disposed) return;
    await player.open(
      Media(
        s.uri,
        httpHeaders: s.headers,
        start: start > Duration.zero ? start : null,
      ),
      play: play,
    );
  }

  Future<void> switchSource(int i) async {
    if (i == sourceIdx || i >= sources.length) return;
    final at = position.value;
    final wasPlaying = playing;
    final r = rate;
    sourceIdx = i;
    if (pipIdx == i) await setPip(null);
    notifyListeners();
    await _open(sources[i], at);
    await player.setRate(r);
    if (!wasPlaying) await player.pause();
  }

  /// Shows a second camera in a floating window, kept in sync with the main one.
  Future<void> setPip(int? i) async {
    if (i == pipIdx) return;
    await pip?.dispose();
    pip = null;
    pipVideo = null;
    pipIdx = i;
    if (i != null) {
      final p = Player(
        configuration: const PlayerConfiguration(
          title: 'Coursive PiP',
          muted: true,
        ),
      );
      pip = p;
      pipVideo = VideoController(p);
      final s = sources[i];
      await p.setVolume(0);
      await p.open(
        Media(s.uri, httpHeaders: s.headers, start: position.value),
        play: playing,
      );
      await p.setRate(rate);
    }
    notifyListeners();
  }

  DateTime _lastPipSync = DateTime(0);
  void _syncPip(Duration main) {
    final p = pip;
    if (p == null) return;
    final now = DateTime.now();
    if (now.difference(_lastPipSync) < const Duration(seconds: 2)) return;
    _lastPipSync = now;
    final drift = (p.state.position - main).inMilliseconds.abs();
    if (drift > 1200) p.seek(main);
  }

  @override
  void seekTo(int ms) {
    _playRequest++;
    unawaited(_seek(ms));
  }

  Future<void> pause() {
    _playRequest++;
    return player.pause();
  }

  Future<void> playFrom(int ms) async {
    final request = ++_playRequest;
    await _seek(ms);
    if (!_disposed && request == _playRequest) await player.play();
  }

  Future<void> _seek(int ms) async {
    final requested = ms < 0 ? 0 : ms;
    final d = Duration(
      milliseconds: requested.clamp(
        0,
        duration.inMilliseconds > 0 ? duration.inMilliseconds : requested,
      ),
    );
    position.value = d;
    seekEvents.value++;
    await Future.wait([player.seek(d), if (pip case final companion?) companion.seek(d)]);
  }

  void seekBy(int seconds) => seekTo(positionMs + seconds * 1000);

  Future<void> setRate(double r) async {
    rate = r;
    await player.setRate(r);
  }

  void startBoost() {
    if (boosting || !playing) return;
    boosting = true;
    player.setRate(prefs.boostSpeed);
    notifyListeners();
  }

  void endBoost() {
    if (!boosting) return;
    boosting = false;
    player.setRate(rate);
    notifyListeners();
  }

  void setVolume(double v) => player.setVolume(v.clamp(0, 100));

  // ---- derived lookups ----

  int slideAt(int ms) {
    if (slides.isEmpty) return -1;
    final s = ms ~/ 1000;
    var lo = 0, hi = slides.length - 1, ans = 0;
    while (lo <= hi) {
      final mid = (lo + hi) >> 1;
      if (slides[mid].tSec <= s) {
        ans = mid;
        lo = mid + 1;
      } else {
        hi = mid - 1;
      }
    }
    return ans;
  }

  int segmentAt(int ms) {
    if (segments.isEmpty) return -1;
    var lo = 0, hi = segments.length - 1, ans = -1;
    while (lo <= hi) {
      final mid = (lo + hi) >> 1;
      if (segments[mid].startMs <= ms) {
        ans = mid;
        lo = mid + 1;
      } else {
        hi = mid - 1;
      }
    }
    return ans;
  }

  String slideUrl(Slide s) => switch (s) {
    _ when s.path.isNotEmpty => api.fileUrl(s.path),
    // daemon fetches remote slides with any headers the source needs.
    _ => api.uri(
      '/api/lectures/$lectureId/slides/${s.idx}/image',
      {'token': api.token},
    ).toString(),
  };

  // ---- persistence ----

  Future<void> save() async {
    if (lectureId.isEmpty) return;
    final ms = positionMs;
    if (ms <= 0 || (ms - _lastSaved).abs() < 2000) return;
    _lastSaved = ms;
    try {
      await api.put('/api/lectures/$lectureId/playback', {
        'position_ms': ms,
        'duration_ms': duration.inMilliseconds,
        'speed': rate,
      });
    } catch (_) {}
  }

  Future<void> reloadSyncedState() async {
    try {
      final d = await api.get('/api/lectures/$lectureId') as Map;
      if (_disposed) return;
      bookmarks = jsonList(d['bookmarks'])
          .map((e) => Bookmark((e as Map).cast<String, dynamic>()))
          .toList();
      notifyListeners();
      await Future.wait([
        reloadNotes(),
        reloadTranscript(),
        reloadCourseware(),
      ]);
    } catch (_) {}
  }

  Future<Bookmark?> addBookmark([String label = '']) async {
    try {
      final r = await api.post('/api/lectures/$lectureId/bookmarks', {
        't_ms': positionMs,
        'label': label,
      });
      final b = Bookmark((r as Map).cast<String, dynamic>());
      bookmarks = [...bookmarks, b]..sort((a, b) => a.tMs - b.tMs);
      notifyListeners();
      return b;
    } catch (_) {
      return null;
    }
  }

  Future<void> deleteBookmark(Bookmark b) async {
    bookmarks = bookmarks.where((x) => x.id != b.id).toList();
    notifyListeners();
    try {
      await api.delete('/api/bookmarks/${b.id}');
    } catch (_) {}
  }

  @override
  void notifyListeners() {
    if (!_disposed) super.notifyListeners();
  }

  Future<void> close() async {
    _playRequest++;
    _watch?.stop();
    await save();
    _disposed = true;
    _codecCheck?.cancel();
    _saver?.cancel();
    for (final s in _subs) {
      s.cancel();
    }
    await pip?.dispose();
    await player.dispose();
    seekEvents.dispose();
    position.dispose();
    buffer.dispose();
    panelTab.dispose();
  }
}
