// Playback belongs to a user action, even when opening a recording needs I/O.
class PlaybackFocus {
  static final instance = PlaybackFocus();

  int _latest = 0;
  Object? _owner;
  Future<void> Function()? _pauseOwner;
  Future<void> _tail = Future.value();

  int request() => ++_latest;
  bool current(int ticket) => ticket == _latest;
  void cancel(int? ticket) {
    if (ticket == _latest) _latest++;
  }

  Future<void> _serialize(Future<void> Function() action) {
    final next = _tail.then((_) => action());
    _tail = next.catchError((Object _) {});
    return next;
  }

  Future<void> start(int ticket, Object owner, {
    required Future<void> Function() pause,
    required Future<void> Function() play,
  }) => _serialize(() async {
    if (!current(ticket)) return;
    if (_owner != null && !identical(_owner, owner)) {
      await _pauseOwner!();
      _owner = _pauseOwner = null;
    }
    if (!current(ticket)) return;
    _owner = owner;
    _pauseOwner = pause;
    await play();
  });

  Future<void> stop(Object owner, Future<void> Function() stop) =>
      _serialize(() async {
        await stop();
        if (identical(_owner, owner)) _owner = _pauseOwner = null;
      });
}
