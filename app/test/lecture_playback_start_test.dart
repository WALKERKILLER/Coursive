import 'package:coursive/features/player/lecture_player.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('a handout timestamp takes priority when opening a lecture', () {
    int open({int? requested, bool autoResume = true, int duration = 9953000}) =>
        playbackStartMs(
          requested: requested,
          saved: 6086750,
          duration: duration,
          autoResume: autoResume,
        );

    expect(open(requested: 83000), 83000);
    expect(open(requested: 0), 0);
    expect(open(requested: 83000, autoResume: false), 83000);
    expect(open(requested: 83000, duration: 0), 83000);
    expect(open(requested: 83000, duration: 10000), 10000);
    expect(open(), 6086750);
    expect(open(autoResume: false), 0);
  });
}
