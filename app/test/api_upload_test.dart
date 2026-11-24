import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:coursive/core/api.dart';
import 'package:file_selector/file_selector.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('upload preserves Finder filename and XFile byte stream', () async {
    final dir = await Directory.systemTemp.createTemp('coursive-upload-');
    addTearDown(() => dir.delete(recursive: true));
    final file = File('${dir.path}/复习 讲义 #1.md');
    final bytes = utf8.encode('# 操作系统\n' * 12500);
    await file.writeAsBytes(bytes);
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    final received = server.first.then((request) async {
      final body = await request.fold<List<int>>(
        [],
        (all, chunk) => all..addAll(chunk),
      );
      final name = request.uri.queryParameters['name'];
      request.response.write('{"ok":true}');
      await request.response.close();
      return (body, name, request.contentLength);
    });
    final api = Api('http://127.0.0.1:${server.port}', 'test');
    final x = XFile(file.path);
    final result = await api.upload(
      '/assets',
      {'name': x.name},
      x.openRead(),
      await x.length(),
    );
    expect(result['ok'], true);
    final (body, name, length) = await received;
    expect(name, '复习 讲义 #1.md');
    expect(length, bytes.length);
    expect(body, bytes);
  });
  test('a file read error completes the upload with an error', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    server.listen((r) => r.drain<void>().catchError((Object _) {}));
    final api = Api('http://127.0.0.1:${server.port}', 'test');
    Stream<Uint8List> broken() async* {
      yield Uint8List.fromList([1, 2]);
      throw const FileSystemException('read interrupted');
    }

    await expectLater(
      api
          .upload('/assets', {}, broken(), 100)
          .timeout(const Duration(seconds: 5)),
      throwsA(isNot(isA<TimeoutException>())),
    );
  });
}
