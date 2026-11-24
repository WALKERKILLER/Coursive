import 'dart:convert';
import 'dart:io';

/// Read Finder's file URLs only when the user invokes Paste. AppKit resolves
/// percent escapes and preserves multiple files and non-ASCII names.
Future<List<String>> readMacClipboardFiles() async {
  final result = await Process.run('/usr/bin/osascript', [
    '-l',
    'JavaScript',
    '-e',
    r'''
      ObjC.import('AppKit');
      function run() {
        const items = $.NSPasteboard.generalPasteboard.pasteboardItems;
        const paths = [];
        for (let i = 0; i < items.count; i++) {
          const raw = ObjC.unwrap(items.objectAtIndex(i).stringForType('public.file-url'));
          if (typeof raw !== 'string') continue;
          const url = $.NSURL.URLWithString(raw);
          if (url.isFileURL) paths.push(ObjC.unwrap(url.path));
        }
        return JSON.stringify(paths);
      }
    ''',
  ]);
  if (result.exitCode != 0) {
    throw ProcessException(
      'osascript',
      const [],
      result.stderr.toString(),
      result.exitCode,
    );
  }
  return (jsonDecode(result.stdout as String) as List).cast<String>();
}
