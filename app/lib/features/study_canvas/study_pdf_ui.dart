part of 'study_canvas.dart';

class _PDFBody extends StatefulWidget {
  const _PDFBody({required this.owner, required this.card});
  final StudyCanvasState owner;
  final StudyCard card;
  @override
  State<_PDFBody> createState() => _PDFBodyState();
}

class _PDFBodyState extends State<_PDFBody> {
  bool busy = false;
  String? error;
  Future<void> page(int number) async {
    if (busy || number < 1 || number > widget.card.pageCount) return;
    setState(() {
      busy = true;
      error = null;
    });
    try {
      final out = await widget.owner._loadPDFPage(
        widget.card,
        number,
      );
      if (!mounted) return;
      final d = widget.owner.document!;
      d.checkpoint();
      widget.card
        ..pageAspect = (out['width'] as num) / (out['height'] as num)
        ..page = number
        ..path = '${out['file']}'
        ..text = '${out['text']}'
        ..textBoxes = (out['boxes'] as List? ?? []).cast<Map>();
      d.changed();
    } catch (e) {
      if (mounted) setState(() => error = '$e');
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final card = widget.card;
    return Column(
      children: [
        Expanded(
          child: Stack(
            children: [
              Positioned.fill(
                child: Image(
                  image: ResizeImage(
                    NetworkImage(widget.owner.widget.api.fileUrl(card.path)),
                    width: 1500,
                    height: 1500,
                    policy: ResizeImagePolicy.fit,
                  ),
                  fit: BoxFit.contain,
                  errorBuilder: (_, e, s) => Center(
                    child: TextButton(
                      onPressed: () => page(card.page),
                      child: const Text('重载此页'),
                    ),
                  ),
                ),
              ),
              if (busy) const Center(child: CircularProgressIndicator()),
            ],
          ),
        ),
        if (error != null)
          Text(
            error!,
            maxLines: 2,
            style: const TextStyle(fontSize: 11, color: Colors.red),
          ),
        SizedBox(
          height: 36,
          child: Row(
            children: [
              IconButton(
                tooltip: '上一页',
                onPressed: busy || card.page <= 1
                    ? null
                    : () => page(card.page - 1),
                icon: const Icon(Icons.chevron_left, size: 18),
              ),
              Text(
                '${card.page} / ${card.pageCount}',
                style: const TextStyle(fontSize: 12),
              ),
              IconButton(
                tooltip: '下一页',
                onPressed: busy || card.page >= card.pageCount
                    ? null
                    : () => page(card.page + 1),
                icon: const Icon(Icons.chevron_right, size: 18),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// The local screenshot deck uses one page per timestamped slide, in order.
int? studySlidePage(String path,List<Slide> slides,int pages,int ms){
  if(!path.endsWith('_课件截图版.pdf') || pages!=slides.length || slides.isEmpty) return null;
  final directory=slides.first.path.split('/slides/');
  if(directory.length!=2 || !path.startsWith('${directory.first}/courseware/')) return null;
  int selected=1;
  for(var i=0;i<slides.length;i++){
    final s=slides[i];
    if(s.idx!=i+1 || !s.path.startsWith('${directory.first}/slides/')) return null;
    if(s.tSec*1000<=ms) selected=i+1;
  }
  return selected;
}

/// A paper viewport inside a canvas card. Its parent owns the frame and title bar.
class _CanvasPdf extends StatefulWidget {
  const _CanvasPdf({
    super.key,
    required this.owner,
    required this.card,
    this.root,
  });
  final StudyCard? root;
  final StudyCanvasState owner;
  final StudyCard card;
  @override
  State<_CanvasPdf> createState() => _CanvasPdfState();
}

class _CanvasPdfState extends State<_CanvasPdf> {
  final _reader = GlobalKey<PDFDockState>();
  LecturePlayer? _linked;
  int? _pendingMs;
  bool _scheduled=false;
  void _scheduleFollow() {
    if(_pendingMs==null || _scheduled) return;
    _scheduled=true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scheduled=false;
      if(!mounted || _pendingMs==null) return;
      final player=_linked;
      final reader=_reader.currentState;
      if(player==null || reader==null || player.slides.isEmpty || _doc==null) return;
      final ms=_pendingMs!;
      _pendingMs=null;
      final card=widget.card;
      if(!player.prefs.slideFollow || widget.root!=null || card.collapsed || reader.writing) return;
      final page=studySlidePage(card.originalPath,player.slides,_doc!.sizes.length,ms);
      if(page!=null) reader.jumpToPage(page);
    });
    WidgetsBinding.instance.ensureVisualUpdate();
  }
  void _seek(){ _pendingMs=_linked?.positionMs; _scheduleFollow(); }
  void _bind(){
    final next=widget.root==null && widget.card.originalPath.endsWith('_课件截图版.pdf') ? widget.owner.widget._player : null;
    if(identical(next,_linked)) return;
    _linked?.seekEvents.removeListener(_seek);
    _linked?.removeListener(_scheduleFollow);
    _linked=next;
    next?.seekEvents.addListener(_seek);
    next?.addListener(_scheduleFollow);
    _seek();
  }
  @override
  void initState(){super.initState();_bind();}
  @override
  void didUpdateWidget(_CanvasPdf old){super.didUpdateWidget(old);_bind();}
  @override
  void dispose(){
    _linked?.seekEvents.removeListener(_seek);
    _linked?.removeListener(_scheduleFollow);
    super.dispose();
  }
  late Future<DockPDF> _opening = _open();
  DockPDF? _doc;
  Future<bool> flush() async => await _reader.currentState?.flush() ?? true;

  Future<DockPDF> _open() =>
      widget.owner._openPaper(widget.root ?? widget.card);

  void _eraseLegacy(
    int page,
    List<Offset> trail,
    Size size,
    double radius,
    String mode,
  ) {
    widget.owner.document!.erasePDFInk(
      widget.card,
      page,
      _doc!.sizes[page - 1].aspectRatio,
      trail,
      size,
      radius,
      mode,
    );
  }

  @override
  Widget build(BuildContext context) => FutureBuilder<DockPDF>(
    future: _opening,
    builder: (context, s) {
      if (s.hasError)
        return Center(
          child: TextButton(
            onPressed: () => setState(() => _opening = _open()),
            child: Text('${s.error}\n${l10n.commonRetry}'),
          ),
        );
      if (!s.hasData) return const Center(child: CircularProgressIndicator());
      _doc = s.data!;
      _scheduleFollow();
      final reading = widget.owner._focusedMaterial?.id == widget.card.id;
      final zoom = reading ? 1.0 : widget.owner.document!.zoom * widget.card.contentScale;
      return PDFDock(
        key: _reader,
        freezeScroll: widget.owner._pdfPinching,
        embedded: !reading,
        shapeAssist:
            widget.owner._presets[widget.owner._presetIndex].shapeAssist,
        pressureThreshold: widget.owner._pressureEraser
            ? widget.owner._pressureThreshold
            : null,
        canvasZoom: zoom,
        pdf: s.data!,
        horizontal: widget.card.pdfPaged || (reading && widget.root != null),
        firstPage: widget.card.page,
        singlePage: reading || widget.root == null ? null : widget.card.page,
        allowMouseInk:
            widget.owner._ink ||
            widget.owner._tool == StudyTool.eraser,
        erasing: widget.owner._tool == StudyTool.eraser,
        inkColor: Color(widget.owner._color)
            .withValues(alpha: widget.owner._brush.opacity),
        penWidth: widget.owner._penWidth,
        squareTip: widget.owner._brush.squareTip,
        marker: widget.owner._brush.family == 'highlighter',
        eraseMode: widget.owner._eraserMode,
        eraseRadius: widget.owner._eraserSize / zoom,
        onTool: (eraser) =>
            widget.owner._chooseTool(eraser ? StudyTool.eraser : StudyTool.pen),
        onEraseLegacy: _eraseLegacy,
        onPage: (page) {
          if (widget.root != null || widget.card.page == page) return;
          widget.card.page = page;
          widget.owner.document!.changed();
        },
        legacyInk: (page, size) {
          final card = widget.card, doc = widget.owner.document!;
          final strokes = doc.inkOnCard(card.id)
              .where(
                (s) =>
                    doc.visible(s.layer) &&
                    (s.page == page || (s.page == 0 && card.page == page)),
              )
              .toList();
          if (strokes.isEmpty) return null;
          return _LegacyPageInk(
            studyOriginalPaper(card, size.aspectRatio),
            strokes,
            card.rect.center,
            card.rotation + card.quarterTurns * math.pi / 2,
          );
        },
      );
    },
  );
}

class _LegacyPageInk extends CustomPainter {
  const _LegacyPageInk(this.paper, this.strokes, this.center, this.angle);
  final Offset center;
  final double angle;
  final Rect paper;
  final List<StudyStroke> strokes;
  @override
  void paint(Canvas canvas, Size size) {
    canvas.save();
    canvas.clipRect(Offset.zero & size);
    canvas.scale(size.width / paper.width, size.height / paper.height);
    canvas.translate(center.dx - paper.left, center.dy - paper.top);
    canvas.rotate(-angle);
    canvas.translate(-center.dx, -center.dy);
    for (final stroke in strokes) {
      stroke.paint(canvas);
    }
    canvas.restore();
  }

  @override
  bool shouldRepaint(_LegacyPageInk old) =>
      paper != old.paper || center != old.center || angle != old.angle ||
      !listEquals(strokes, old.strokes);
}
