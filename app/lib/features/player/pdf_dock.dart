import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/foundation.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:path/path.dart' as path;

import '../../l10n/l10n.dart';
import '../../core/android_bridge.dart';
import '../../core/ink_erase.dart';
import '../study_canvas/study_shape.dart';

/// A local PDF working copy. The opening revision is rendered throughout this
/// session; ink added since opening is painted as an overlay and persisted once.
class DockPDF {
  DockPDF._(
    this.source,
    this.output,
    this.cache,
    this.helper,
    this.helperTimeout,
  );
  final String source, output, helper;
  final Directory cache;
  final Duration helperTimeout;
  final _renders = <void Function()>{};
  bool _closing = false;
  Future<void>? _closeFuture;
  static int _requestId = 0;

  late List<Size> sizes;
  late String revision;
  late bool canExpand;
  late int baseMargin, margin;
  String get snapshot => path.join(cache.path, 'opening.pdf');
  final strokes = <DockStroke>[];
  int savedCount = 0, savedMargin = 0;
  List<DockStroke> _saved = [];
  final _hidden = <int, Set<int>>{};
  bool get dirty => !listEquals(_saved, strokes) || margin != savedMargin;

  static Future<DockPDF> open(
    String source,
    String output, {
    String? executable,
    Duration helperTimeout = const Duration(seconds: 45),
  }) async {
    final helper =
        executable ??
        path.join(
          path.dirname(path.dirname(Platform.resolvedExecutable)),
          'Resources',
          'pdf-notes',
          'coursive-pdf-notes',
        );
    final cache = await Directory.systemTemp.createTemp('coursive-pdf-view-');
    final pdf = DockPDF._(source, output, cache, helper, helperTimeout);
    try {
      // A single copy fixes the render revision while asynchronous saves replace
      // the working file. Verify it matches the metadata we opened.
      final state = await pdf.command('info');
      pdf.revision = state['revision'] as String;
      pdf.canExpand = state['canExpand'] == true;
      await File(pdf.revision.isEmpty ? source : output).copy(pdf.snapshot);
      if (pdf.revision.isNotEmpty &&
          (await pdf.command('info'))['revision'] != pdf.revision) {
        throw StateError(l10n.pdfDockChanged);
      }
      pdf.baseMargin = pdf.margin = pdf.savedMargin = state['margin'] as int;
      pdf.sizes = [
        for (final s in state['sizes'] as List)
          Size((s[0] as num).toDouble(), (s[1] as num).toDouble()),
      ];
      for (final mark in state['ink'] as List? ?? []) {
        final page = mark['page'] as int;
        pdf._hidden.putIfAbsent(page, () => {}).add(mark['index'] as int);
        final points = [
          for (final pair in mark['points'])
            Offset((pair[0] as num).toDouble(), (pair[1] as num).toDouble()),
        ];
        if (points.isNotEmpty)
          pdf.strokes.add(
            DockStroke(
              page,
              points,
              argb: (mark['argb'] as num?)?.toInt() ?? DockStroke.defaultColor,
              width: (mark['width'] as num?)?.toDouble() ?? .003,
              square: mark['square'] == true,
              marker: mark['marker'] == true,
            ),
          );
      }
      pdf._saved = List.of(pdf.strokes);
      pdf.savedCount = pdf.strokes.length;
      return pdf;
    } catch (_) {
      await cache.delete(recursive: true);
      rethrow;
    }
  }

  Future<void> close() => _closeFuture ??= _close();
  Future<void> _close() async {
    _closing = true;
    for (final cancel in _renders.toList()) {
      cancel();
    }
    try {
      // A final flush may already be in flight when the owner leaves.
      await _pendingSave;
      if (dirty) await save();
    } finally {
      await cache.delete(recursive: true);
    }
  }

  Future<T> _bounded<T>(
    bool render,
    Future<T> work,
    void Function() abort,
  ) async {
    final stopped = Completer<T>();
    void cancel() {
      abort();
      if (!stopped.isCompleted) {
        stopped.completeError(StateError('PDF reader closed'));
      }
    }

    if (render) _renders.add(cancel);
    try {
      return await Future.any([work, stopped.future]).timeout(
        helperTimeout,
        onTimeout: () {
          abort();
          throw TimeoutException('PDF operation timed out', helperTimeout);
        },
      );
    } finally {
      _renders.remove(cancel);
    }
  }

  Future<T> _android<T>(String op, String input, {int? page, Map? edit}) {
    final id = ++_requestId;
    final work = AndroidBridge.pdfNotes<T>(
      op,
      input,
      output,
      page: page,
      edit: edit,
      requestId: id,
    );
    return _bounded(op == 'render', work.then((v) => v!), () {
      unawaited(AndroidBridge.cancelPDFNotes(id));
    });
  }

  Future<Uint8List> _native(List<String> args, {Map? payload}) async {
    final render = args.first == 'render';
    final process = await Process.start(helper, args);
    void abort() {
      process.kill(ProcessSignal.sigkill);
    }

    if (render && _closing) {
      abort();
      throw StateError('PDF reader closed');
    }
    Future<Uint8List> collect() async {
      final buffer = BytesBuilder(copy: false);
      final errors = StringBuffer();
      Future<void> write() async {
        if (payload != null) process.stdin.write(jsonEncode(payload));
        await process.stdin.close();
      }

      final exit = process.exitCode;
      await Future.wait<void>([
        process.stdout.forEach(buffer.add),
        process.stderr.transform(utf8.decoder).forEach(errors.write),
        write(),
        exit.then((_) {}),
      ]);
      if (await exit != 0) throw FormatException(errors.toString());
      return buffer.takeBytes();
    }

    return _bounded(render, collect(), abort);
  }

  Future<Map> command(String op, [Map? payload]) async {
    if (Platform.isAndroid) return _android<Map>(op, source, edit: payload);
    return jsonDecode(
      utf8.decode(await _native([op, source, output], payload: payload)),
    ) as Map;
  }

  final _rendering = <int, Future<Uint8List>>{};
  Future<Uint8List> render(int page) => _rendering.putIfAbsent(
    page,
    () => _renderPage(page).whenComplete(() {
      _rendering.remove(page);
    }),
  );
  Future<Uint8List> _renderPage(int page) async {
    final file = File(path.join(cache.path, '$page.png'));
    if (await file.exists()) return file.readAsBytes();
    if (_closing) throw StateError('PDF reader closed');
    final data = Platform.isAndroid
        ? await _android<Uint8List>(
            'render',
            snapshot,
            page: page,
            edit: {'hidden': _hidden[page]?.toList() ?? []},
          )
        : await _native([
            'render',
            snapshot,
            snapshot,
            '$page',
            if (_hidden.containsKey(page)) _hidden[page]!.join(','),
          ]);
    if (!_closing && await cache.exists()) await file.writeAsBytes(data);
    return data;
  }

  Future<void>? _pendingSave;
  Future<void> save() =>
      _pendingSave ??= _save().whenComplete(() => _pendingSave = null);
  Future<void> _save() async {
    if (!dirty) return;
    final snapshot = List.of(strokes), extension = margin;
    final update = await command('save', {
      'revision': revision,
      'margin': extension,
      'replaceInk': true,
      'strokes': [for (final s in snapshot) s.json()],
    });
    revision = update['revision'] as String;
    _saved = snapshot;
    savedCount = snapshot.length;
    savedMargin = extension;
  }
}

class DockStroke {
  DockStroke(
    this.page,
    this.points, {
    this.argb = defaultColor,
    this.width = .003,
    this.square = false,
    this.marker = false,
  });
  static const defaultColor = 0xff1f3b91;
  final int argb;
  // Fraction of the original paper width, independent of viewport and margin.
  final double width;
  final bool square, marker;
  DockStroke withPoints(List<Offset> fragment) => DockStroke(
    page,
    fragment,
    argb: argb,
    width: width,
    square: square,
    marker: marker,
  );
  final int page;
  final List<Offset> points;
  late final Rect bounds = inkBounds(points);
  Path? _path;
  Size? _pathSize;
  int _builtPoints = 0;
  Path pathFor(Size size, {bool live = false}) {
    // A stroke is append-only. Shape fitting and erasing create new strokes.
    if (_path == null || size != _pathSize || points.length < _builtPoints) {
      _pathSize = size;
      _builtPoints = 0;
      _path = Path();
    }
    final path = _path!;
    if (_builtPoints == 0) {
      final first = points.first;
      path.moveTo(first.dx * size.width, first.dy * size.height);
      path.relativeLineTo(.05, .05);
      _builtPoints = 1;
    }
    for (; _builtPoints < points.length; _builtPoints++) {
      final p = points[_builtPoints];
      path.lineTo(p.dx * size.width, p.dy * size.height);
    }
    return path;
  }

  Map json() => {
    'page': page,
    'argb': argb,
    'width': width,
    'square': square,
    'marker': marker,
    'points': [
      for (final p in points) [p.dx, p.dy],
    ],
  };
}

/// Preview changes locally until pointer-up. A cancelled gesture leaves the
/// document and any concurrent save untouched.
class _PageErase {
  _PageErase(Iterable<DockStroke> page) : visible = page.toList();
  List<DockStroke> visible;
  final _original = <DockStroke, DockStroke>{};
  final replacements = <DockStroke, List<DockStroke>>{};

  bool apply(InkEraser eraser) {
    final next = <DockStroke>[];
    var changed = false;
    for (final stroke in visible) {
      if (!eraser.touchesBounds(stroke.bounds)) {
        next.add(stroke);
        continue;
      }
      final fragments = eraser.clip(stroke.points, bounds: stroke.bounds);
      if (fragments.length == 1 && identical(fragments.first, stroke.points)) {
        next.add(stroke);
        continue;
      }
      changed = true;
      final original = _original.remove(stroke) ?? stroke;
      final pieces = [
        for (final points in fragments) stroke.withPoints(points),
      ];
      final group = replacements.putIfAbsent(original, () => [original]);
      final index = group.indexOf(stroke);
      group.replaceRange(index, index + 1, pieces);
      for (final piece in pieces) {
        _original[piece] = original;
      }
      next.addAll(pieces);
    }
    if (changed) visible = next;
    return changed;
  }

  void commit(List<DockStroke> all) {
    if (replacements.isEmpty) return;
    final updated = <DockStroke>[
      for (final stroke in all)
        if (replacements.containsKey(stroke))
          ...replacements[stroke]!
        else
          stroke,
    ];
    all
      ..clear()
      ..addAll(updated);
  }
}

typedef PDFEraseGesture = void Function(
  int page,
  List<Offset> path,
  Size paper,
  double radius,
  String mode,
);

class PDFDock extends StatefulWidget {
  const PDFDock({
    super.key,
    required this.pdf,
    this.onClose,
    this.legacyInk,
    this.onEraseLegacy,
    this.allowMouseInk = true,
    this.freezeScroll = false,
    this.shapeAssist = false,
    this.pressureThreshold,
    this.canvasZoom = 1,
    this.erasing = false,
    this.inkColor = const Color(DockStroke.defaultColor),
    this.penWidth,
    this.squareTip = false,
    this.marker = false,
    this.eraseMode = "stroke",
    this.eraseRadius = 12,
    this.horizontal = false,
    this.firstPage = 1,
    this.onPage,
    this.onTool,
    this.singlePage,
  });
  final DockPDF pdf;
  final VoidCallback? onClose;
  final bool freezeScroll, shapeAssist;
  final double? pressureThreshold;
  final double canvasZoom;
  final bool horizontal, erasing, allowMouseInk, squareTip, marker;
  final Color inkColor;
  final double? penWidth;
  final String eraseMode;
  final double eraseRadius;
  final int firstPage;
  final int? singlePage;
  final ValueChanged<int>? onPage;
  final ValueChanged<bool>? onTool;
  final PDFEraseGesture? onEraseLegacy;
  final CustomPainter? Function(int page, Size original)? legacyInk;
  @override
  State<PDFDock> createState() => PDFDockState();
}

class PDFDockState extends State<PDFDock> {
  DockPDF get pdf => widget.pdf;
  Future<bool>? _saving;
  Timer? _eraseSave;
  String? _error;
  bool _mousePen = true, _localErase = false;
  late final PageController _pages = PageController(
    initialPage: (widget.firstPage - 1).clamp(0, pdf.sizes.length - 1),
  );
  late int _page = _pages.initialPage;
  final _vertical = ScrollController();
  bool get writing =>
      _sheets.values.any((key) => key.currentState?.pointer != null);
  final _sheets = <int, GlobalKey<_PDFSheetState>>{};
  bool jumpToPage(int number) {
    if (widget.singlePage != null ||
        number < 1 ||
        number > pdf.sizes.length ||
        writing)
      return false;
    if (widget.horizontal) {
      if (!_pages.hasClients) return false;
      _pages.jumpToPage(number - 1);
    } else {
      if (!_vertical.hasClients) return false;
      final width =
          _vertical.position.context.storageContext.findRenderObject()
              as RenderBox?;
      if (width == null || !width.hasSize) return false;
      double offset = 0;
      for (final page in pdf.sizes.take(number - 1)) {
        offset +=
            (width.size.width - 12) /
                (page.aspectRatio * (1 + pdf.margin / 100)) +
            10;
      }
      // The lazy list's scroll extent estimate can be low for mixed page sizes.
      _vertical.jumpTo(offset);
    }
    return true;
  }

  @override
  void dispose() {
    if (_eraseSave != null) unawaited(flush());
    _vertical.dispose();
    _pages.dispose();
    super.dispose();
  }

  Widget _sheet(int i) => _PDFSheet(
    key: _sheets.putIfAbsent(i, () => GlobalKey<_PDFSheetState>()),
    pdf: pdf,
    number: i + 1,
    inkColor: widget.inkColor,
    penWidth: widget.penWidth,
    squareTip: widget.squareTip,
    marker: widget.marker,
    shapeAssist: widget.shapeAssist,
    pressureThreshold: widget.pressureThreshold,
    zoom: widget.canvasZoom,
    mousePen: widget.allowMouseInk && _mousePen,
    erasing: widget.erasing || _localErase,
    eraseMode: widget.eraseMode,
    eraseRadius: widget.eraseRadius,
    legacyInk: widget.legacyInk?.call(i + 1, pdf.sizes[i]),
    onEraseLegacy: widget.onEraseLegacy,
    onChanged: _changed,
  );

  Future<bool> flush() {
    _eraseSave?.cancel();
    _eraseSave = null;
    return _saving ??= _drain().whenComplete(() => _saving = null);
  }

  Future<bool> _drain() async {
    try {
      while (pdf.dirty) {
        await pdf.save();
      }
      _error = null;
      return true;
    } catch (e) {
      _error = '$e';
      return false;
    } finally {
      if (mounted) setState(() {});
    }
  }

  void _changed({bool erased = false}) {
    setState(() {});
    if (erased) {
      _eraseSave?.cancel();
      _eraseSave = Timer(
        const Duration(milliseconds: 220),
        () => unawaited(flush()),
      );
    } else {
      unawaited(flush());
    }
  }

  @override
  Widget build(BuildContext context) {
    if (widget.singlePage case final int page)
      return Center(child: _sheet(page - 1));
    return Material(
      color: Theme.of(context).colorScheme.surface,
      child: Column(
        children: [
          Row(
            children: [
              const SizedBox(width: 8),
              Expanded(
                child: Tooltip(
                  message: pdf.output,
                  child: Text(
                    path
                            .basenameWithoutExtension(pdf.output)
                            .replaceFirst(
                              RegExp('${RegExp.escape(l10n.pdfDockSuffix)}\$'),
                              '',
                            ) +
                        '.pdf',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ),
              IconButton(
                tooltip: l10n.pdfDockPen,
                isSelected: _mousePen,
                onPressed: () {
                  if (widget.erasing || _localErase) {
                    setState(() {
                      _localErase = false;
                      _mousePen = true;
                    });
                    widget.onTool?.call(false);
                  } else {
                    setState(() => _mousePen = !_mousePen);
                  }
                },
                icon: const Icon(Icons.edit_outlined),
              ),
              IconButton(
                tooltip: l10n.pdfCardEraser,
                icon: const Icon(Icons.auto_fix_normal),
                isSelected: widget.erasing || _localErase,
                onPressed: () {
                  final next = !(widget.erasing || _localErase);
                  if (widget.onTool != null) {
                    widget.onTool!(next);
                  } else {
                    setState(() => _localErase = next);
                  }
                },
              ),
              if (!widget.horizontal)
                PopupMenuButton<int>(
                  enabled: pdf.canExpand,
                  tooltip: pdf.canExpand
                      ? l10n.pdfDockMargin
                      : l10n.pdfDockCropped,
                  icon: const Icon(Icons.border_right),
                  itemBuilder: (_) => [
                    for (final m in [0, 25, 50])
                      CheckedPopupMenuItem(
                        value: m,
                        checked: pdf.margin == m,
                        enabled: m >= pdf.margin,
                        child: Text('${l10n.pdfDockMargin} $m%'),
                      ),
                  ],
                  onSelected: (m) {
                    if (m != pdf.margin && m >= pdf.margin) {
                      pdf.margin = m;
                      _changed();
                    }
                  },
                ),
              if (widget.onClose != null)
                IconButton(
                  tooltip: l10n.commonClose,
                  onPressed: widget.onClose,
                  icon: const Icon(Icons.close),
                ),
            ],
          ),
          if (_error != null)
            Row(
              children: [
                Expanded(child: SelectableText(_error!)),
                TextButton(onPressed: flush, child: Text(l10n.commonRetry)),
              ],
            )
          else
            Align(
              alignment: Alignment.centerLeft,
              child: Padding(
                padding: const EdgeInsets.only(left: 10),
                child: Text(
                  pdf.dirty
                      ? l10n.pdfDockSaving
                      : pdf.revision.isEmpty
                      ? l10n.pdfDockHint
                      : l10n.pdfDockSaved,
                  style: Theme.of(context).textTheme.labelSmall,
                ),
              ),
            ),
          const Divider(height: 1),
          Expanded(
            child: widget.horizontal
                ? PageView.builder(
                    physics: widget.freezeScroll
                        ? const NeverScrollableScrollPhysics()
                        : null,
                    controller: _pages,
                    itemCount: pdf.sizes.length,
                    onPageChanged: (i) {
                      setState(() => _page = i);
                      widget.onPage?.call(i + 1);
                    },
                    itemBuilder: (_, i) => Center(child: _sheet(i)),
                  )
                : LayoutBuilder(
                    builder: (_, viewport) => ListView.builder(
                      controller: _vertical,
                      // A distant video seek must skip intervening pages without
                      // mounting their sheets and starting a render for each one.
                      itemExtentBuilder: (i, _) =>
                          (viewport.maxWidth - 12) /
                              (pdf.sizes[i].aspectRatio *
                                  (1 + pdf.margin / 100)) +
                          10,
                      physics: widget.freezeScroll
                          ? const NeverScrollableScrollPhysics()
                          : null,
                      itemCount: pdf.sizes.length,
                      itemBuilder: (_, i) => Padding(
                        padding: const EdgeInsets.fromLTRB(6, 6, 6, 4),
                        child: _sheet(i),
                      ),
                    ),
                  ),
          ),
          if (widget.horizontal)
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                IconButton(
                  tooltip: l10n.pdfCardPrevious,
                  icon: const Icon(Icons.chevron_left),
                  onPressed: _page > 0
                      ? () => _pages.previousPage(
                          duration: const Duration(milliseconds: 180),
                          curve: Curves.easeOut,
                        )
                      : null,
                ),
                Text('${_page + 1} / ${pdf.sizes.length}'),
                IconButton(
                  tooltip: l10n.pdfCardNext,
                  icon: const Icon(Icons.chevron_right),
                  onPressed: _page + 1 < pdf.sizes.length
                      ? () => _pages.nextPage(
                          duration: const Duration(milliseconds: 180),
                          curve: Curves.easeOut,
                        )
                      : null,
                ),
              ],
            ),
        ],
      ),
    );
  }
}

class _PDFSheet extends StatefulWidget {
  const _PDFSheet({
    super.key,
    required this.pdf,
    required this.number,
    required this.mousePen,
    required this.shapeAssist,
    required this.pressureThreshold,
    required this.zoom,
    required this.inkColor,
    required this.penWidth,
    required this.squareTip,
    required this.marker,
    this.legacyInk,
    this.onEraseLegacy,
    required this.erasing,
    required this.eraseMode,
    required this.eraseRadius,
    required this.onChanged,
  });
  final DockPDF pdf;
  final int number;
  final bool shapeAssist;
  final double? pressureThreshold;
  final double zoom;
  final bool mousePen, erasing, squareTip, marker;
  final Color inkColor;
  final double? penWidth;
  final double eraseRadius;
  final String eraseMode;
  final PDFEraseGesture? onEraseLegacy;
  final CustomPainter? legacyInk;
  final void Function({bool erased}) onChanged;
  @override
  State<_PDFSheet> createState() => _PDFSheetState();
}

class _PDFSheetState extends State<_PDFSheet> {
  late Future<Uint8List> image = widget.pdf.render(widget.number);
  int? pointer;
  bool erase = false;
  DockStroke? active;
  List<Offset> live = [];
  _PageErase? _preview;
  int _processed = 0;
  int? _frame;
  final _motion = ValueNotifier<int>(0);
  final _marks = ValueNotifier<int>(0);
  late Size _paper;
  late double _radius;
  late String _mode;

  Timer? _pressureHold, _shapeHold;
  Offset? _downAt, _holdAt;
  StudyShape? _shape;
  Offset? _shapeTip;
  Offset _local(Offset p) => Offset(p.dx * _paper.width, p.dy * _paper.height);
  Offset _normal(Offset p) => Offset(p.dx / _paper.width, p.dy / _paper.height);
  void _updateIntent(PointerEvent e) {
    if (erase || active == null) return;
    final screen = e.position;
    if (_downAt != null && (screen - _downAt!).distance > 6) _downAt = null;
    final threshold = widget.pressureThreshold;
    if (_downAt != null &&
        threshold != null &&
        e.kind == PointerDeviceKind.stylus &&
        e.pressureMax > e.pressureMin &&
        (e.pressure - e.pressureMin) / (e.pressureMax - e.pressureMin) >=
            threshold) {
      _shapeHold?.cancel();
      _holdAt = null;
      _pressureHold ??= Timer(const Duration(milliseconds: 380), () {
        if (active == null) return;
        final at = live.last;
        erase = true;
        active = null;
        live = [at];
        _processed = 0;
        _preview = _PageErase(
          widget.pdf.strokes.where((s) => s.page == widget.number),
        );
        _queueFrame();
      });
      return;
    }
    _pressureHold?.cancel();
    _pressureHold = null;
    if (!widget.shapeAssist || _shape != null || live.length < 3) return;
    if (_holdAt != null && (screen - _holdAt!).distance <= 6) return;
    _holdAt = screen;
    _shapeHold?.cancel();
    _shapeHold = Timer(const Duration(milliseconds: 700), () {
      final fitted = recognizeStudyShape(
        live.map(_local).toList(),
        zoom: widget.zoom,
      );
      if (fitted == null || active == null) return;
      _shape = fitted;
      _shapeTip = _local(live.last);
      _fit(fitted.points);
    });
  }

  void _fit(List<Offset> points) {
    live = points.map(_normal).toList();
    active = active!.withPoints(live);
    _queueFrame();
  }

  void _append(Offset p) {
    if (_shape case final fitted?) {
      final pos = _local(p);
      _fit(
        (pos - _shapeTip!).distance * widget.zoom <= 6
            ? fitted.points
            : fitted.adjust(_shapeTip!, pos),
      );
    } else if (p != live.last) {
      live.add(p);
    }
  }

  void _applyTrail({bool end = false}) {
    final preview = _preview;
    if (preview == null ||
        _processed == live.length ||
        (_mode == 'lasso' && !end))
      return;
    final section = _processed == 0 ? live : live.sublist(_processed - 1);
    _processed = live.length;
    if (preview.apply(InkEraser(section, _paper, _radius, _mode)))
      _marks.value++;
  }

  void _queueFrame() {
    if (!erase) {
      _motion.value++;
      return;
    }
    _frame ??= SchedulerBinding.instance.scheduleFrameCallback((_) {
      _frame = null;
      _applyTrail();
      _motion.value++;
    });
  }

  void _finish() {
    _pressureHold?.cancel();
    _pressureHold = null;
    _shapeHold?.cancel();
    _shapeHold = null;
    _shape = null;
    _shapeTip = _holdAt = _downAt = null;
    if (_frame != null)
      SchedulerBinding.instance.cancelFrameCallbackWithId(_frame!);
    _frame = null;
    pointer = null;
    _preview = null;
    active = null;
    live = [];
    _motion.value++;
    _marks.value++;
  }

  @override
  void dispose() {
    if (_frame != null)
      SchedulerBinding.instance.cancelFrameCallbackWithId(_frame!);
    _pressureHold?.cancel();
    _shapeHold?.cancel();
    _motion.dispose();
    _marks.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final pdf = widget.pdf, size = pdf.sizes[widget.number - 1];
    final factor = 1 + pdf.margin / 100;
    return AspectRatio(
      aspectRatio: size.aspectRatio * factor,
      child: LayoutBuilder(
        builder: (context, b) {
          final w = b.maxWidth / factor, h = b.maxHeight;
          Offset point(Offset p) =>
              Offset((p.dx / w).clamp(0.0, factor), (p.dy / h).clamp(0.0, 1.0));
          return ColoredBox(
            color: Colors.white,
            child: Stack(
              fit: StackFit.expand,
              children: [
                Align(
                  alignment: Alignment.centerLeft,
                  child: SizedBox(
                    width: w * (1 + pdf.baseMargin / 100),
                    height: h,
                    child: FutureBuilder<Uint8List>(
                      future: image,
                      builder: (ctx, s) {
                        if (s.hasError)
                          return TextButton(
                            onPressed: () => setState(
                              () => image = pdf.render(widget.number),
                            ),
                            child: Text(l10n.commonRetry),
                          );
                        if (!s.hasData)
                          return const Center(
                            child: CircularProgressIndicator.adaptive(),
                          );
                        return Image.memory(
                          s.data!,
                          fit: BoxFit.fill,
                          gaplessPlayback: true,
                        );
                      },
                    ),
                  ),
                ),
                if (widget.legacyInk case final painter?)
                  Align(
                    alignment: Alignment.centerLeft,
                    child: SizedBox(
                      width: w,
                      height: h,
                      child: IgnorePointer(
                        child: CustomPaint(painter: painter),
                      ),
                    ),
                  ),
                // The enclosing scroll view handles fingers and the wheel; the ink
                // recognizer claims only pen/mouse gestures before the scroll drag.
                RawGestureDetector(
                  behavior: HitTestBehavior.opaque,
                  gestures: {
                    EagerGestureRecognizer:
                        GestureRecognizerFactoryWithHandlers<
                          EagerGestureRecognizer
                        >(
                          () => EagerGestureRecognizer(
                            supportedDevices: {
                              PointerDeviceKind.stylus,
                              PointerDeviceKind.invertedStylus,
                              if (widget.mousePen || widget.erasing)
                                PointerDeviceKind.mouse,
                            },
                          ),
                          (r) {
                            r.supportedDevices = {
                              PointerDeviceKind.stylus,
                              PointerDeviceKind.invertedStylus,
                              if (widget.mousePen || widget.erasing)
                                PointerDeviceKind.mouse,
                            };
                          },
                        ),
                  },
                  child: Listener(
                    behavior: HitTestBehavior.opaque,
                    onPointerDown: (e) {
                      if (pointer != null ||
                          (e.kind != PointerDeviceKind.stylus &&
                              e.kind != PointerDeviceKind.invertedStylus &&
                              !((widget.mousePen || widget.erasing) &&
                                  e.kind == PointerDeviceKind.mouse &&
                                  e.buttons == kPrimaryButton)))
                        return;
                      pointer = e.pointer;
                      erase =
                          widget.erasing ||
                          e.kind == PointerDeviceKind.invertedStylus;
                      live = [point(e.localPosition)];
                      active = erase
                          ? null
                          : DockStroke(
                              widget.number,
                              live,
                              argb: widget.inkColor.toARGB32(),
                              width: widget.penWidth == null
                                  ? .003
                                  : widget.penWidth! / w,
                              square: widget.squareTip,
                              marker: widget.marker,
                            );
                      _paper = Size(w, h);
                      _radius = widget.eraseRadius;
                      _mode = widget.eraseMode;
                      if (erase) {
                        _processed = 0;
                        _preview = _PageErase(
                          pdf.strokes.where((s) => s.page == widget.number),
                        );
                      }
                      _downAt = e.position;
                      _updateIntent(e);
                      _queueFrame();
                    },
                    onPointerMove: (e) {
                      if (e.pointer != pointer) return;
                      final p = point(e.localPosition);
                      _append(p);
                      _updateIntent(e);
                      _queueFrame();
                    },
                    onPointerCancel: (e) {
                      if (e.pointer == pointer) _finish();
                    },
                    onPointerUp: (e) {
                      if (e.pointer != pointer) return;
                      final p = point(e.localPosition);
                      _append(p);
                      var changed = true;
                      if (erase) {
                        _applyTrail(end: true);
                        changed = _preview!.replacements.isNotEmpty;
                        _preview!.commit(pdf.strokes);
                        widget.onEraseLegacy?.call(
                          widget.number,
                          live,
                          _paper,
                          _radius,
                          _mode,
                        );
                      } else {
                        pdf.strokes.add(active!);
                      }
                      _finish();
                      if (changed) widget.onChanged(erased: erase);
                    },
                    child: Stack(
                      fit: StackFit.expand,
                      children: [
                        RepaintBoundary(
                          child: ValueListenableBuilder<int>(
                            valueListenable: _marks,
                            builder: (_, tick, child) => CustomPaint(
                              key: const ValueKey('pdf-committed-ink'),
                              painter: _DockInk(
                                _preview?.visible ??
                                    pdf.strokes
                                        .where((s) => s.page == widget.number)
                                        .toList(),
                                w,
                                h,
                                null,
                                0,
                              ),
                            ),
                          ),
                        ),
                        RepaintBoundary(
                          child: CustomPaint(
                            key: const ValueKey('pdf-live-ink'),
                            painter: _LivePageInk(this, w, h),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}

class _LivePageInk extends CustomPainter {
  _LivePageInk(this.sheet, this.w, this.h) : super(repaint: sheet._motion);
  final _PDFSheetState sheet;
  final double w, h;
  Offset? get cursor =>
      sheet.erase && sheet.pointer != null ? sheet.live.last : null;
  @override
  void paint(Canvas canvas, Size size) {
    _DockInk(
      [if (sheet.active case final mark?) mark],
      w,
      h,
      cursor,
      sheet.widget.eraseRadius,
      live: true,
      lasso: cursor != null && sheet._mode == 'lasso' ? sheet.live : null,
    ).paint(canvas, size);
  }

  @override
  bool shouldRepaint(_LivePageInk old) =>
      old.w != w || old.h != h || old.sheet != sheet;
}

class _DockInk extends CustomPainter {
  _DockInk(
    this.lines,
    this.w,
    this.h,
    this.cursor,
    this.radius, {
    this.live = false,
    this.lasso,
  });
  final bool live;
  final List<Offset>? lasso;
  final Offset? cursor;
  final double radius;
  final List<DockStroke> lines;
  final double w, h;
  @override
  void paint(Canvas canvas, Size size) {
    canvas.clipRect(Offset.zero & size);
    final paint = Paint()
      ..strokeJoin = StrokeJoin.round
      ..style = PaintingStyle.stroke;
    for (final mark in lines) {
      final line = mark.points;
      paint
        ..strokeWidth = w * mark.width
        ..strokeCap = mark.square ? StrokeCap.square : StrokeCap.round
        ..blendMode = mark.marker ? BlendMode.darken : BlendMode.srcOver
        ..color = Color(mark.argb);
      if (line.isEmpty) continue;
      canvas.drawPath(mark.pathFor(Size(w, h), live: live), paint);
    }
    if (lasso case final trail? when trail.length > 1) {
      final path = Path()
        ..addPolygon([
          for (final p in trail) Offset(p.dx * w, p.dy * h),
        ], false);
      canvas.drawPath(
        path,
        Paint()
          ..style = PaintingStyle.stroke
          ..color = Colors.grey
          ..strokeWidth = 1,
      );
    }
    if (cursor case final Offset p)
      canvas.drawCircle(
        Offset(p.dx * w, p.dy * h),
        radius,
        Paint()
          ..style = PaintingStyle.stroke
          ..color = Colors.grey,
      );
  }

  @override
  bool shouldRepaint(_DockInk old) =>
      live || old.w != w || old.h != h || !listEquals(lines, old.lines);
}
