part of 'study_canvas.dart';

extension _StudyOutlineExport on StudyCanvasState {
  /// Draws [child] offscreen at [scale] and returns it as PNG, sized in
  /// logical pixels (points in the PDF).
  Future<OutlineImage?> _rasterize(Widget child, {double scale = 4}) async {
    final boundary = RenderRepaintBoundary();
    final view = RenderView(
      view: View.of(context),
      configuration: const ViewConfiguration(
        logicalConstraints: BoxConstraints(maxWidth: 1400, maxHeight: 1400),
      ),
      child: RenderPositionedBox(alignment: Alignment.topLeft, child: boundary),
    );
    final pipeline = PipelineOwner()..rootNode = view;
    view.prepareInitialFrame();
    final build = BuildOwner(focusManager: FocusManager());
    final root = RenderObjectToWidgetAdapter<RenderBox>(
      container: boundary,
      child: MediaQuery(
        data: const MediaQueryData(),
        child: Directionality(
          textDirection: TextDirection.ltr,
          child: DefaultTextStyle(
            style: const TextStyle(color: Color(0xff1d2125), fontSize: 11),
            child: child,
          ),
        ),
      ),
    ).attachToRenderTree(build);
    build.buildScope(root);
    build.finalizeTree();
    pipeline
      ..flushLayout()
      ..flushCompositingBits()
      ..flushPaint();
    if (!boundary.hasSize || boundary.size.isEmpty) return null;
    final image = await boundary.toImage(pixelRatio: scale);
    try {
      final data = await image.toByteData(format: ui.ImageByteFormat.png);
      return data == null
          ? null
          : (
              png: data.buffer.asUint8List(),
              width: boundary.size.width,
              height: boundary.size.height,
            );
    } finally {
      image.dispose();
    }
  }

  /// Formulas and marked handwriting for [doc], drawn before layout. A
  /// formula that cannot be drawn stays as its TeX source.
  Future<Map<String, OutlineImage>> _outlineImages(OutlineExport doc) async {
    final out = <String, OutlineImage>{};
    for (final f in outlineFormulas(doc).take(200)) {
      final image = await _rasterize(
        Math.tex(
          f.display ? '\\displaystyle ${f.tex}' : f.tex,
          textStyle: TextStyle(
            fontSize: f.display ? 12 : 11,
            color: const Color(0xff1d2125),
          ),
          parseOptions: const ParseOptions(strict: Strict.ignore),
          onError: (_, _) => const SizedBox.shrink(),
        ),
      );
      if (image != null && image.width > 1) {
        out[outlineMathKey(f.tex, f.display)] = image;
      }
    }
    for (final b in doc.blocks) {
      if (b is! OutlineEntry || b.item?['kind'] != 'unresolved_mark') continue;
      final ink = _markInk(b.item!);
      if (ink.isEmpty) continue;
      final image = await _rasterize(
        SizedBox(
          width: 96,
          height: 72,
          child: CustomPaint(painter: _InkThumbPainter(ink)),
        ),
      );
      if (image != null) out[outlineInkKey(b.item!)] = image;
    }
    return out;
  }

  Future<void> _exportOutlinePdf(Map turn) async {
    final doc = _outlineExport(turn);
    await showDialog<void>(
      context: context,
      builder: (_) => _OutlinePdfDialog(
        doc: doc,
        api: widget.api,
        images: _outlineImages(doc),
        markdown: () => outlineMarkdown(_outlineExport(turn)),
      ),
    );
  }
}

/// Lays the outline out as a PDF and previews pages of exactly the bytes
/// 「保存 PDF」 writes; the daemon renders them with the canvas's page
/// renderer. Changing the paper builds new bytes and a new preview.
class _OutlinePdfDialog extends StatefulWidget {
  const _OutlinePdfDialog({
    required this.doc,
    required this.api,
    required this.images,
    required this.markdown,
  });
  final OutlineExport doc;
  final Api api;
  final Future<Map<String, OutlineImage>> images;
  final String Function() markdown;

  @override
  State<_OutlinePdfDialog> createState() => _OutlinePdfDialogState();
}

class _OutlinePdfDialogState extends State<_OutlinePdfDialog> {
  static Future<OutlineFonts>? _fonts;
  var _layout = const OutlineLayout();
  final _built = <OutlineLayout, Uint8List>{};
  Uint8List? _bytes, _page;
  int _number = 1, _count = 0, _token = 0;
  bool _busy = true, _textLayer = false;
  String? _error, _previewError, _notice;

  @override
  void initState() {
    super.initState();
    unawaited(_build());
  }

  Future<void> _build() async {
    final token = ++_token;
    setState(() {
      _busy = true;
      _error = null;
      _previewError = null;
    });
    try {
      final fonts = await (_fonts ??= OutlineFonts.load());
      final missing = outlineMissingGlyphs(widget.doc, fonts.codepoints);
      if (missing.isNotEmpty) {
        throw _MissingGlyphs(
          l10n.outlinePdfMissing(missing.take(6).join(), missing.length),
        );
      }
      final bytes = _built[_layout] ??= await buildOutlinePdf(
        widget.doc,
        _layout,
        fonts,
        images: await widget.images,
      );
      if (!mounted || token != _token) return;
      _bytes = bytes;
      await _show(_number, token);
    } catch (e) {
      if (mounted && token == _token) {
        setState(() {
          _busy = false;
          _error = e is _MissingGlyphs
              ? e.message
              : l10n.outlineExportFailed('$e');
        });
      }
    }
  }

  Future<void> _show(int number, [int? token]) async {
    token ??= _token;
    final bytes = _bytes;
    if (bytes == null) return;
    setState(() => _busy = true);
    try {
      final out = await widget.api.previewExportPDF(bytes, number) as Map;
      if (!mounted || token != _token) return;
      setState(() {
        _count = (out['count'] as num).toInt();
        _number = number;
        _page = base64Decode('${out['png']}');
        _textLayer = '${out['text'] ?? ''}'.trim().isNotEmpty;
        _previewError = null;
      });
    } on ApiError catch (e) {
      // Past the end after a paper change: show the last page instead.
      if (e.status == 413 && number > 1) return _show(1, token);
      if (mounted && token == _token) {
        setState(() => _previewError = l10n.outlinePdfPreviewFailed('$e'));
      }
    } catch (e) {
      if (mounted && token == _token) {
        setState(() => _previewError = l10n.outlinePdfPreviewFailed('$e'));
      }
    } finally {
      if (mounted && token == _token) setState(() => _busy = false);
    }
  }

  void _relayout(OutlineLayout next) {
    if (next == _layout) return;
    _layout = next;
    unawaited(_build());
  }

  Future<void> _save({bool markdown = false}) async {
    final name = widget.doc.title;
    final path = await pickSavePath(
      context,
      suggestedName: '$name.${markdown ? 'md' : 'pdf'}',
      type: markdown
          ? const XTypeGroup(label: 'Markdown', extensions: ['md'])
          : const XTypeGroup(label: 'PDF', extensions: ['pdf']),
    );
    if (path == null || !mounted) return;
    try {
      if (markdown) {
        await File(path).writeAsString(widget.markdown(), flush: true);
      } else {
        await File(path).writeAsBytes(_bytes!, flush: true);
      }
      if (mounted) setState(() => _notice = l10n.outlineExportSaved(path));
    } on FileSystemException catch (e) {
      if (mounted) {
        setState(
          () => _error = l10n.outlineExportFailed(
            e.osError?.message ?? e.message,
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final size = MediaQuery.sizeOf(context);
    final wide = size.width >= 760;
    final format = _layout.format;
    final preview = Container(
      key: const ValueKey('outline-pdf-preview'),
      alignment: Alignment.center,
      padding: const EdgeInsets.all(Space.lg),
      decoration: BoxDecoration(
        color: cs.surfaceContainerHighest.withValues(alpha: .5),
        borderRadius: BorderRadius.circular(Corners.lg),
      ),
      child: Column(
        children: [
          Expanded(
            child: AspectRatio(
              aspectRatio: format.width / format.height,
              child: AnimatedSwitcher(
                duration: Motion.medium1,
                child: DecoratedBox(
                  key: ValueKey(
                    '${_layout.hashCode}-$_number-${_page?.length}',
                  ),
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(2),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withValues(alpha: .12),
                        blurRadius: 18,
                        offset: const Offset(0, 6),
                      ),
                    ],
                  ),
                  child: _page == null
                      ? Center(
                          child: _previewError != null
                              ? Padding(
                                  padding: const EdgeInsets.all(Space.lg),
                                  child: Text(
                                    _previewError!,
                                    textAlign: TextAlign.center,
                                    style: tt.bodySmall?.copyWith(
                                      color: cs.error,
                                    ),
                                  ),
                                )
                              : _error == null
                              ? const CircularProgressIndicator()
                              : Icon(
                                  Icons.picture_as_pdf_outlined,
                                  color: cs.outline,
                                ),
                        )
                      : Stack(
                          fit: StackFit.expand,
                          children: [
                            Image.memory(
                              _page!,
                              fit: BoxFit.contain,
                              gaplessPlayback: true,
                              filterQuality: FilterQuality.medium,
                            ),
                            if (_busy)
                              ColoredBox(
                                color: Colors.white.withValues(alpha: .5),
                              ),
                          ],
                        ),
                ),
              ),
            ),
          ),
          const SizedBox(height: Space.md),
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              IconButton(
                tooltip: l10n.outlinePrevPage,
                onPressed: _busy || _number <= 1
                    ? null
                    : () => _show(_number - 1),
                icon: const Icon(Icons.chevron_left_rounded),
              ),
              Text(
                _count == 0 ? '–' : l10n.outlinePdfPage(_number, _count),
                key: const ValueKey('outline-pdf-page'),
                style: tt.labelLarge?.copyWith(
                  fontFeatures: const [FontFeature.tabularFigures()],
                ),
              ),
              IconButton(
                tooltip: l10n.outlineNextPage,
                onPressed: _busy || _number >= _count
                    ? null
                    : () => _show(_number + 1),
                icon: const Icon(Icons.chevron_right_rounded),
              ),
            ],
          ),
        ],
      ),
    );
    Widget choice<T>(
      String label,
      T value,
      List<(T, String)> options,
      ValueChanged<T> onChanged,
    ) => Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: tt.labelMedium?.copyWith(color: cs.onSurfaceVariant),
        ),
        const SizedBox(height: Space.xs),
        SegmentedButton<T>(
          showSelectedIcon: false,
          style: const ButtonStyle(visualDensity: VisualDensity.compact),
          segments: [
            for (final (v, l) in options)
              ButtonSegment(value: v, label: Text(l)),
          ],
          selected: {value},
          onSelectionChanged: (v) => onChanged(v.first),
        ),
        const SizedBox(height: Space.md),
      ],
    );
    final controls = Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      mainAxisSize: MainAxisSize.min,
      children: [
        choice(l10n.outlinePaper, _layout.paper, [
          (OutlinePaper.a4, 'A4'),
          (OutlinePaper.letter, 'Letter'),
        ], (v) => _relayout(_layout.copyWith(paper: v))),
        choice(l10n.outlineOrientation, _layout.landscape, [
          (false, l10n.outlinePortrait),
          (true, l10n.outlineLandscape),
        ], (v) => _relayout(_layout.copyWith(landscape: v))),
        choice(l10n.outlineMargins, _layout.compact, [
          (false, l10n.outlineMarginStandard),
          (true, l10n.outlineMarginCompact),
        ], (v) => _relayout(_layout.copyWith(compact: v))),
        if (_bytes != null && _error == null)
          Text(
            [
              if (_textLayer) l10n.outlinePdfTextLayer,
              fmtBytes(_bytes!.length),
            ].join(' · '),
            key: const ValueKey('outline-pdf-facts'),
            style: tt.labelSmall?.copyWith(color: cs.onSurfaceVariant),
          ),
        if (_error != null || _notice != null)
          Padding(
            padding: const EdgeInsets.only(top: Space.sm),
            child: Text(
              _error ?? _notice!,
              key: const ValueKey('outline-pdf-status'),
              style: tt.labelSmall?.copyWith(
                color: _error != null ? cs.error : cs.primary,
              ),
            ),
          ),
        const SizedBox(height: Space.lg),
        FilledButton.icon(
          key: const ValueKey('outline-pdf-save'),
          onPressed: _bytes == null || _error != null || _busy ? null : _save,
          icon: const Icon(Icons.download_rounded, size: 18),
          label: Text(l10n.outlinePdfSave),
        ),
        const SizedBox(height: Space.xs),
        TextButton(
          onPressed: () => _save(markdown: true),
          child: Text(l10n.outlinePdfMarkdown),
        ),
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: Text(l10n.commonCancel),
        ),
      ],
    );
    return Dialog(
      insetPadding: const EdgeInsets.all(Space.lg),
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: 920,
          maxHeight: math.min(720, size.height - 32),
        ),
        child: Padding(
          padding: const EdgeInsets.all(Space.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(l10n.outlinePdfTitle, style: tt.titleLarge),
              Text(
                widget.doc.title,
                style: tt.bodyMedium?.copyWith(color: cs.onSurfaceVariant),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              const SizedBox(height: Space.lg),
              Expanded(
                child: wide
                    ? Row(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          Expanded(child: preview),
                          const SizedBox(width: Space.xl),
                          SizedBox(
                            width: 240,
                            child: SingleChildScrollView(child: controls),
                          ),
                        ],
                      )
                    : Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          Expanded(child: preview),
                          const SizedBox(height: Space.md),
                          Flexible(
                            child: SingleChildScrollView(child: controls),
                          ),
                        ],
                      ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MissingGlyphs implements Exception {
  _MissingGlyphs(this.message);
  final String message;
}
