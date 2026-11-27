import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import '../lib/features/player/playback_focus.dart';

void main() {
  test('a slow pause must finish, and only the latest ready request starts', () async {
    final gate = PlaybackFocus();
    final paused = Completer<void>(), pausing = Completer<void>();
    final audible = <String>{};
    Future<void> start(String id, int ticket) => gate.start(ticket, id,
      pause: () async {
        if (id == 'a') {
          pausing.complete();
          await paused.future;
        }
        audible.remove(id);
      },
      play: () async {
        expect(audible, isEmpty);
        audible.add(id);
      },
    );
    await start('a', gate.request());
    final delayed = gate.request();
    final b = start('b', gate.request());
    await pausing.future;
    final c = start('c', gate.request());
    expect(audible, {'a'});
    paused.complete();
    await Future.wait([b, c]);
    await start('slow asset', delayed);
    expect(audible, {'c'});
  });

  test('cancellation and failed handoff cannot start another video early', () async {
    final gate = PlaybackFocus();
    var canPause = false, starts = 0;
    await gate.start(gate.request(), 'a', pause: () async {
      if (!canPause) throw StateError('decoder busy');
    }, play: () async => starts++);
    Future<void> next(int ticket) => gate.start(ticket, 'b',
        pause: () async {}, play: () async => starts++);
    await expectLater(next(gate.request()), throwsStateError);
    final cancelled = gate.request();
    gate.cancel(cancelled);
    await next(cancelled);
    expect(starts, 1);
    canPause = true;
    await next(gate.request());
    await gate.stop('b', () async {});
    await next(gate.request());
    expect(starts, 3);
  });
}
