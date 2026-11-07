import 'package:flutter_test/flutter_test.dart';
import 'package:coursive/core/daemon_service.dart';

void main() {
  test('Finder PATH includes tools without overriding explicit tool order', () {
    final finder = macOSDaemonPath('/usr/bin:/bin').split(':');
    expect(finder, containsAll(['/opt/homebrew/bin', '/usr/local/bin']));
    final custom = macOSDaemonPath('/custom/bin::/usr/bin:/opt/homebrew/bin')
        .split(':');
    expect(custom.first, '/custom/bin');
    expect(custom.where((p) => p == '/opt/homebrew/bin'), hasLength(1));
    expect(custom, isNot(contains('')));
    expect(macOSDaemonPath(null).split(':'), contains('/usr/bin'));
  });
}
