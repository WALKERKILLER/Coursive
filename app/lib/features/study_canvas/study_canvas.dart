import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:file_selector/file_selector.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter/rendering.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';
import 'package:provider/provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:window_manager/window_manager.dart';

import '../../core/api.dart';
import '../../core/format.dart';
import '../../core/models.dart';
import '../../core/plugins.dart';
import '../../core/ui_slots.dart';
import '../../theme/glass.dart';
import '../../theme/tokens.dart';
import '../../widgets/coursive_markdown.dart';
import '../../widgets/pickers.dart';
import '../../l10n/l10n.dart';
import '../player/lecture_player.dart';
import '../plugins/canvas_fallback.dart';
import '../plugins/plugin_actions.dart' show PluginViewPatch;
import '../plugins/plugin_contributions.dart' show PluginViewPatchScope;
import '../player/pdf_dock.dart';
import '../settings/models_page.dart' show AssistantModelButton;
import '../assistant/assistant_controller.dart';

import 'package:path/path.dart' as filePath;

import '../player/video_card_hold.dart';
import '../practice/quiz.dart';
import 'study_document.dart';
import 'study_query.dart';
import 'study_visual.dart';
import 'study_attachment.dart';
import 'study_pdf_column.dart';
import 'study_shape.dart';
import 'outline_document.dart';
import 'outline_pdf.dart';
import 'reading_glow.dart';
import 'study_selection_geometry.dart';
import 'study_transform.dart';
import 'study_input_router.dart';

import 'package:val_latex_flutter/val_latex_flutter.dart' hide Segment;

part 'study_agent_ui.dart';
part 'study_snapshot_capture.dart';
part 'study_source_navigation.dart';
part 'study_agent_stage.dart';
part 'study_shelf_ui.dart';
part 'study_brush_ui.dart';
part 'study_interaction.dart';
part 'study_conversation_ui.dart';
part 'study_pdf_ui.dart';
part 'study_toolbar_ui.dart';
part 'study_edge_dock.dart';
part 'study_theme.dart';
part 'study_pen_palette.dart';
part 'study_shape_interaction.dart';
part 'study_stylus.dart';
part 'study_selection_tools.dart';
part 'study_transforms.dart';
part 'study_evidence.dart';
part 'study_outline_review.dart';
part 'study_outline_export.dart';
part 'study_plugin_ui.dart';

enum StudyTool { select, pen, highlighter, eraser, region }

class StudyCanvas extends StatefulWidget {
  const StudyCanvas({
    super.key,
    required this.api,
    required LecturePlayer ctl,
    required this.video,
    required this.panel,
    required this.onClassic,
    this.pdfLibraryRoot,
    this.openCanvasPDF = DockPDF.open,
    required this.onAsk,
    required this.onBack,
    this.fullscreen = false,
  }) : _player = ctl,
       notebookId = null,
       title = null;

  const StudyCanvas.notebook({
    super.key,
    required this.api,
    required String id,
    required String name,
    this.pdfLibraryRoot,
    this.openCanvasPDF = DockPDF.open,
    required this.onBack,
  }) : notebookId = id,
       title = name,
       _player = null,
       video = const SizedBox.shrink(),
       panel = const SizedBox.shrink(),
       onAsk = null,
       onClassic = null,
       fullscreen = false;
  final String? notebookId, title;
  bool get standalone => notebookId != null;
  String get draftKey => standalone
      ? 'notebook-draft-$notebookId'
      : 'study-draft-${ctl.lectureId}';
  Future<dynamic> load() => standalone
      ? api.get('/api/notebooks/$notebookId/canvas')
      : api.studyCanvas(ctl.lectureId);
  Future<dynamic> save(Map<String, dynamic> document) => standalone
      ? api.put('/api/notebooks/$notebookId/canvas', document)
      : api.saveStudyCanvas(ctl.lectureId, document);
  final Api api;
  final LecturePlayer? _player;
  LecturePlayer get ctl => _player!;
  final Widget video, panel;
  final VoidCallback? onClassic;
  final String? pdfLibraryRoot;
  final Future<DockPDF> Function(String, String) openCanvasPDF;
  final VoidCallback onBack;
  final ValueChanged<String>? onAsk;
  final bool fullscreen;
  @override
  State<StudyCanvas> createState() => StudyCanvasState();
}

class StudyCanvasState extends State<StudyCanvas>
    with WidgetsBindingObserver, TickerProviderStateMixin {
  /// Theme-derived chrome colours for the whole workspace.
  StudyChrome get chrome => StudyChrome.of(context);

  /// Dark paper: ink and paper are toned for the dark scheme.
  bool get _dark => Theme.of(context).brightness == Brightness.dark;
  StudyDocument? document;
  Object? _loadError, _saveError;

  /// Enabled plugins and whether one offers a canvas tool, read in [build].
  Set<String> _enabledPlugins = const {};
  bool _hasCanvasTools = false;
  StudyTool _tool = StudyTool.select;
  int _color = 0xff25344a;
  double _penWidth = 3;
  StudyBrush _brush = const StudyBrush(), _gestureBrush = const StudyBrush();
  double _gestureWidth = 3, _eraserSize = 18;
  int _gestureColor = 0xff25344a, _presetIndex = 0;
  List<StudyPenPreset> _presets = StudyPenPreset.defaults();
  String _eraserMode = 'partial', _paperStyle = 'ruled';
  int _paperColor = 0xffffffff;
  StudyDockPlacement _penPlacement = const StudyDockPlacement(
    StudyDockEdge.bottom,
  );
  StudyDockPlacement _aiPlacement = const StudyDockPlacement(
    StudyDockEdge.right,
    .34,
  );
  final _aiDockKey = GlobalKey();
  bool _dockSmall = false;
  final _penItemKeys = <int, GlobalKey>{};
  final _eraserKey = GlobalKey();
  final _eraserLasso = <Offset>[], _lasso = <Offset>[];
  final _camera = ValueNotifier(0), _cursor = ValueNotifier(0);

  /// Carries the view to a new place instead of cutting to it; see [_flyTo].
  late final _fly = AnimationController(vsync: this, duration: Motion.long2)
    ..addListener(() => _flyStep?.call());
  void Function()? _flyStep;

  /// Where a finger rests on a card while the hold to move it runs.
  final _holdCue = ValueNotifier<Offset?>(null);
  final _selectionMenuKey = GlobalKey();
  Rect? _stageRect, _selectionBounds;
  Offset? _eraserCursor, _pressureOrigin, _pressureStart;
  double _cursorRadius = 18, _pressureThreshold = .8;
  bool _pressureEraser = true, _temporaryEraser = false;
  bool _selectionReady = false, _selectionDragging = false;
  bool _selectionToolbarDocked = false, _selectionMeasurePending = false;
  List<Rect> _selectionObstacles = [];
  StudySelectionHandle? _selectionHandle;
  Rect? _selectionResizeStart;
  List<Offset> _selectionResizePolygon = [];
  bool _lassoRectangle = true, _captureLasso = false;
  final _selectionKinds = StudySelectionKind.values.toSet();
  bool _selectionBusy = false;
  List<StudyAnchor>? _linkStart;
  Rect? _linkStartBounds;
  Timer? _pressureTimer;
  Timer? _shapeTimer;
  Timer? _cardHoldTimer;
  Offset? _cardHoldOrigin;
  Offset? _rotatePivot;
  double _rotateLast = 0;
  StudyTransformSession? _transformSession;
  int? _transformPointer, _transformCorner;
  Offset _transformStart = Offset.zero, _transformVector = Offset.zero;
  double _rotationRaw = 0, _rotationBase = 0;
  String? _rotationLabel;
  bool _angleSnapping = true;
  StudyAngleSnap _angleSnap = StudyAngleSnap();
  final _readingContacts = <int>{}, _blockedTouches = <int>{};
  final _videoSurfacePointers = <int>{};
  final _videoControlPointers = <int>{};
  final _inputSurface = GlobalKey();
  StreamSubscription? _stylusEvents;
  Timer? _predictionExpiry;
  bool _predictionEnabled = true, _penButtons = true;
  List<InkPoint> _predicted = [];
  int _acceptedPredictions = 0;
  int? _lastRealStylusTime;
  Offset _lastRealStylusPosition = Offset.zero;
  Offset? _shapeHoldOrigin, _shapeHandle;
  StudyShape? _shape;
  List<InkPoint>? _shapePreview;
  int _pressureKeep = 0;
  List<StudyStroke> _copiedInk = [];
  List<StudyCard> _copiedCards = [];
  Rect? _copiedBounds;

  final _topBarKey = GlobalKey();
  final _moduleKey = GlobalKey(), _toolbarKey = GlobalKey();
  final _conversationKeys = <String, GlobalKey<_ConversationBodyState>>{};
  final _initialConversations = <String, Map>{};
  String? _activeConversation;

  /// Conversations mounted this session; a hidden one keeps its run going.
  final _liveThreads = <String>{};

  /// Last known turn count and question per conversation, for thread nodes.
  final _threadSummary = <String, Map>{};
  final _threadTick = ValueNotifier(0);
  final _stageKeys = <GlobalKey>{};
  bool get _inlinePDF =>
      widget.pdfLibraryRoot != null &&
      (Platform.isMacOS ||
          Platform.isAndroid ||
          widget.openCanvasPDF != DockPDF.open);
  final _pdfBodies = <String, GlobalKey<_CanvasPdfState>>{};
  final _pdfFiles = <String, Future<DockPDF>>{};
  bool _pdfPageCard(StudyCard c) =>
      _inlinePDF &&
      c.type == 'pdf' &&
      c.pdfColumn.isNotEmpty &&
      document!.cards.any((p) => p.id == c.pdfColumn && p.type == 'pdfdoc');
  Future<DockPDF> _openPaper(StudyCard root) {
    final base = filePath.normalize(widget.pdfLibraryRoot!);
    final source = filePath.normalize(filePath.join(base, root.source));
    final scope = widget.standalone
        ? 'notebooks/${widget.notebookId}'
        : widget.ctl.lecture!.dir;
    final folder = filePath.normalize(filePath.join(base, scope, 'courseware'));
    if (!filePath.isWithin(base, source) || !filePath.isWithin(base, folder))
      throw StateError(l10n.pdfDockChanged);
    final title = filePath.basenameWithoutExtension(root.title);
    final output = filePath.join(folder, title + l10n.pdfDockSuffix + '.pdf');
    return _pdfFiles.putIfAbsent(source + '\n' + output, () async {
      await Directory(folder).create(recursive: true);
      try {
        return await widget.openCanvasPDF(source, output);
      } catch (_) {
        _pdfFiles.remove(source + '\n' + output);
        rethrow;
      }
    });
  }

  final _cardHeaderActions = <String, GlobalKey>{};
  final _composerFocus = FocusNode(debugLabel: 'study-composer');
  final _idleInput = TextEditingController();

  /// True while a stroke, pan or drag is under way; the stage steps back.
  final _gesturing = ValueNotifier(false);
  bool _stageOpen = false;
  bool _quickAsk = false, _quickSending = false, _quickImages = false;
  String? _quickError;
  final _quickInput = TextEditingController();
  final _sourceNavigations = <String, Future<void>>{};
  final _messageScroll = <String?, ScrollController>{};
  bool _snapshotting = false, _snapshotPreparing = false;
  final _snapshotFrames = <String, Uint8List>{};
  List<Offset> _selectionPolygon = [];

  /// The screenshot previewed in the quick composer once 「看图」 is on.
  StudySnapshot? _quickSnapshot;
  bool _quickCapturing = false;

  /// Views left by following citations, newest last; see [_goBack].
  final _bookmarks = <_ViewMark>[];
  final _returnKey = GlobalKey();

  /// The outline under review and, on a narrow window, whether the review
  /// steps down to show the source.
  _ReviewTarget? _review;
  bool _reviewPeek = false;
  final _reviewKey = GlobalKey();

  /// The review item last pointed at or focused.
  FocusNode? _reviewFocus;

  /// The citation under the pointer or keyboard focus, or the one just followed.
  final _evidenceFocus = ValueNotifier<Map?>(null);
  late final _evidenceSweep = AnimationController(
    vsync: this,
    duration: Motion.medium2,
  );
  late final _evidenceLanding = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 2400),
  );

  /// The open 课次素材 tab, or null when the shelf is away.
  _ShelfTab? _shelfTab, _shelfShown;
  final _shelfKey = GlobalKey();

  /// Where a dragged shelf item would land, in canvas pixels.
  final _dropHint = ValueNotifier<Rect?>(null);
  String _status = l10n.studyOpening;
  bool _importing = false, _disposed = false;
  Timer? _saveTimer;
  Timer? _syncCheck;
  StreamSubscription? _syncSubscription;
  bool _remotePending = false, _refreshing = false;
  Map? _remoteDocument;
  Future<bool>? _saving;
  int _savedEdit = 0;
  final _scene = ValueNotifier(0), _live = ValueNotifier(0);
  final _selection = <String>{};
  final _detail = StudySelection();
  final _capture = GlobalKey();
  final _dockKey = GlobalKey(), _zoomKey = GlobalKey();
  final _localVideoKeys = <String, GlobalKey<_LocalVideoState>>{};
  final _localVideoTimes = <String, int>{};
  Rect? _regionSelection, _sourceHighlight;
  void _update(VoidCallback action) => setState(action);
  final _pointers = <int, Offset>{};
  final _touchPointers = <int>{}, _stylusPointers = <int>{};
  List<InkPoint> _points = [];
  final List<StudyStroke> _prefix = [];
  int _prefixEnd = 0;
  String _anchor = '';
  Offset? _start, _last;
  Offset? _panCamera;
  bool _pdfPinching = false;
  bool _trackpadOnPdf = false;
  bool _onPdf(Offset global) => _pdfBodies.values.any((key) {
    final box = key.currentContext?.findRenderObject() as RenderBox?;
    return box != null &&
        box.hasSize &&
        box.size.contains(box.globalToLocal(global));
  });
  void _clearPaperCursor() {
    if (_eraserCursor != null) {
      _eraserCursor = null;
      _cursor.value++;
    }
  }

  Rect? _marquee;
  String? _dragCard;
  int _cardTextEpoch = 0;
  bool _erasing = false;
  double? _pinchDistance, _pinchZoom;
  Offset? _pinchWorld;
  int? _drawingPointer;
  int _pointBudget = 20000;
  Size _viewport = const Size(1000, 700);

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    if (widget.standalone) _tool = StudyTool.pen;
    _load();
    _syncCheck = Timer.periodic(const Duration(seconds: 5), (_) {
      final lifecycle = WidgetsBinding.instance.lifecycleState;
      if (lifecycle == AppLifecycleState.paused ||
          lifecycle == AppLifecycleState.hidden ||
          lifecycle == AppLifecycleState.detached ||
          !mounted ||
          ModalRoute.of(context)?.isCurrent == false)
        return;
      unawaited(_refreshFromSync());
    });
    _syncSubscription = widget.api.sse('/api/events').listen((e) {
      if (e['type'] == 'sync_done' &&
          ((e['data'] as Map?)?['downloaded'] as num? ?? 0) > 0) {
        unawaited(_refreshFromSync());
      }
    }, onError: (_) {});
    unawaited(_titlebar(true));
    unawaited(_startStylus());
    _loadBrushSettings();
    _loadLayout();
  }

  Future<void> _titlebar(bool canvas) async {
    if (!Platform.isMacOS) return;
    try {
      await windowManager.setTitleBarStyle(
        canvas ? TitleBarStyle.hidden : TitleBarStyle.normal,
      );
      if (canvas && !mounted)
        await windowManager.setTitleBarStyle(TitleBarStyle.normal);
    } on PlatformException catch (_) {
      // Window chrome is optional; canvas navigation remains in the toolbar.
    } on MissingPluginException catch (_) {}
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) unawaited(_refreshFromSync());
    if (state != AppLifecycleState.resumed) {
      _finishTransform(cancel: true);
      _cancelCardHold();
      _cancelShapeHold();
      _clearPrediction();
    } else {
      unawaited(_configureStylus());
    }
  }

  bool get _editingCanvas =>
      _transformSession != null ||
      _importing ||
      _selectionBusy ||
      (document != null && document!.edit != _savedEdit) ||
      _saving != null ||
      _drawingPointer != null ||
      _erasing ||
      _dragCard != null ||
      _rotatePivot != null ||
      _pdfPinching;

  bool get _wholeCardSelected =>
      _selection.isNotEmpty && _detail.objectId == null;

  Future<void> _load({Map? synced}) async {
    try {
      final data = synced ?? await widget.load();
      if (!mounted) return;
      final prefs = await SharedPreferences.getInstance();
      final draft = prefs.getString(widget.draftKey);
      if (!mounted) return;
      if (synced != null && _editingCanvas) return;
      final d = StudyDocument.fromJson(data as Map);
      if (draft != null) {
        document?.dispose();
        d.dispose();
        _remoteDocument = data;
        _remotePending = true;
        document = StudyDocument.fromJson(jsonDecode(draft) as Map);
        _adoptThreads(document!);
        document!.edit = 1;
        _savedEdit = 0;
        document!.addListener(_changed);
        if (mounted) setState(() => _status = '已恢复本机草稿，请选择保留版本');
        return;
      }
      document?.dispose();
      document = d;
      _adoptThreads(d);
      d.addListener(_changed);
      _savedEdit = d.edit;
      setState(() {
        _loadError = null;
        _status = l10n.studySaved;
      });
    } catch (e) {
      if (mounted) setState(() => _loadError = e);
    }
  }

  Future<void> _refreshFromSync() async {
    if (_refreshing || !mounted) return;
    _refreshing = true;
    try {
      final remote = await widget.load() as Map;
      final d = document;
      if (d == null || !mounted) return;
      if (remote['base_hash'] == d.baseHash) return;
      // Finish an edit already in progress. Its save rebases against the latest
      // revision, so the last saved whole canvas wins (ADR 0022).
      if (_editingCanvas) return;
      await _load(synced: remote);
    } catch (_) {
    } finally {
      _refreshing = false;
    }
  }

  Future<void> _resolveRemote(bool keepLocal) async {
    final remote = _remoteDocument;
    if (remote == null) return;
    if (keepLocal) {
      document!.revision = (remote['revision'] as num).toInt();
      document!.baseHash = remote['base_hash'] as String? ?? '';
      _remotePending = false;
      if (!await flush()) {
        _remotePending = true;
        return;
      }
    } else {
      _remotePending = false;
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(widget.draftKey);
      await _load();
    }
    if (!_remotePending) {
      final prefs = await SharedPreferences.getInstance();
      await prefs.remove(widget.draftKey);
      if (mounted) setState(() {});
    }
  }

  void _changed() {
    if (_disposed) return;
    if (_selectionReady) {
      final strokes = document!.strokes.map((s) => s.id).toSet();
      final cards = document!.cards.map((c) => c.id).toSet();
      _detail.strokeIds.removeWhere((id) => !strokes.contains(id));
      _selection.removeWhere((id) => !cards.contains(id));
      _selectionBounds = _regionSelection ?? _boundsOfSelection();
      if (_selectionBounds == null && _detail.objectId == null)
        _selectionReady = false;
    }
    _saveTimer?.cancel();
    _saveTimer = Timer(const Duration(milliseconds: 1000), _saveWhenIdle);
    _scene.value++;
    setState(() => _status = l10n.studyPendingSave);
  }

  void _saveWhenIdle() {
    if (_drawingPointer != null || _erasing || _dragCard != null) {
      _saveTimer = Timer(const Duration(seconds: 1), _saveWhenIdle);
    } else {
      unawaited(flush());
    }
  }

  Future<bool> flush() async {
    if (_transformSession != null) return false;
    _saveTimer?.cancel();
    for (final key in _pdfBodies.values.toList()) {
      if (!(await key.currentState?.flush() ?? true)) return false;
    }
    final current = _saving;
    if (current != null) {
      if (!await current) return false;
    }
    final d = document;
    if (_remotePending) return false;
    if (d == null || d.edit == _savedEdit) return true;
    final future = _saveLoop(d);
    _saving = future;
    try {
      return await future;
    } finally {
      if (identical(_saving, future)) _saving = null;
    }
  }

  Future<bool> _saveLoop(StudyDocument d) async {
    try {
      var retries = 0;
      while (d.edit != _savedEdit) {
        final edit = d.edit;
        if (mounted) setState(() => _status = l10n.studySaving);
        Map out;
        try {
          out = await widget.save(d.json()) as Map;
        } on ApiError catch (e) {
          if (e.status != 409 || retries++ >= 2) rethrow;
          final remote = await widget.load() as Map;
          if (widget.standalone) {
            final prefs = await SharedPreferences.getInstance();
            await prefs.setString(widget.draftKey, jsonEncode(d.json()));
            _remoteDocument = remote;
            _remotePending = true;
            rethrow;
          }
          // Retain the superseded saved document before replacing it.
          await widget.api.post(
            '/api/lectures/${widget.ctl.lectureId}/study-canvas/draft',
            remote,
          );
          final remoteVersion = (remote['version'] as num).toInt();
          if (remoteVersion > 5)
            throw FormatException(l10n.studyUnknownVersion);
          d.formatVersion = math.max(d.formatVersion, remoteVersion);
          d.revision = (remote['revision'] as num).toInt();
          d.baseHash = remote['base_hash'] as String? ?? '';
          continue;
        }
        d.revision = (out['revision'] as num).toInt();
        d.baseHash = out['base_hash'] as String? ?? '';
        _savedEdit = edit;
      }
      if (mounted)
        setState(() {
          _saveError = null;
          _status = l10n.studySaved;
        });
      return true;
    } catch (e) {
      if (mounted)
        setState(() {
          _saveError = e;
          _status = l10n.studySaveFailed;
        });
      return false;
    }
  }

  /// The system back action first returns from a followed citation.
  bool returnFromSource() => document != null && _goBack();

  bool handleKey(KeyEvent e) {
    if (e is! KeyDownEvent || document == null) return false;
    // Typing in the composer or a card must not switch tools.
    final focus = FocusManager.instance.primaryFocus?.context;
    if (focus != null &&
        (focus.widget is EditableText ||
            focus.findAncestorWidgetOfExactType<EditableText>() != null))
      return false;
    final k = e.logicalKey;
    final keys = HardwareKeyboard.instance;
    // Esc steps back one level: composer, selection, then a followed citation.
    if (k == LogicalKeyboardKey.escape) {
      if (_quickAsk) {
        _update(() => _quickAsk = false);
        return true;
      }
      if (_selectionReady) {
        _clearSelection();
        return true;
      }
      return _goBack();
    }
    if ((keys.isAltPressed && k == LogicalKeyboardKey.arrowLeft) ||
        (keys.isMetaPressed && k == LogicalKeyboardKey.bracketLeft)) {
      return _goBack();
    }
    if ((k == LogicalKeyboardKey.enter ||
            k == LogicalKeyboardKey.numpadEnter) &&
        _tool == StudyTool.region &&
        _selectionReady &&
        !_quickAsk) {
      if (!widget.standalone) unawaited(_ask());
      return true;
    }
    if ((keys.isMetaPressed || keys.isControlPressed) &&
        k == LogicalKeyboardKey.keyZ) {
      keys.isShiftPressed ? document!.redo() : document!.undo();
      return true;
    }
    final tool = {
      LogicalKeyboardKey.keyV: StudyTool.select,
      LogicalKeyboardKey.keyH: StudyTool.select,
      LogicalKeyboardKey.keyD: StudyTool.pen,
      LogicalKeyboardKey.keyE: StudyTool.eraser,
    }[k];
    if (tool != null) {
      _chooseTool(tool);
      return true;
    }
    return false;
  }

  @override
  void dispose() {
    _cancelCardHold();
    unawaited(_titlebar(false));
    WidgetsBinding.instance.removeObserver(this);
    _syncCheck?.cancel();
    _disposed = true;
    _saveTimer?.cancel();
    _syncSubscription?.cancel();
    _pressureTimer?.cancel();
    _shapeTimer?.cancel();
    _predictionExpiry?.cancel();
    unawaited(_stylusEvents?.cancel());
    _fly.dispose();
    _holdCue.dispose();
    _camera.dispose();
    _cursor.dispose();
    _threadTick.dispose();
    _composerFocus.dispose();
    _idleInput.dispose();
    _quickInput.dispose();
    _evidenceFocus.dispose();
    _evidenceSweep.dispose();
    _evidenceLanding.dispose();
    _gesturing.dispose();
    _dropHint.dispose();
    for (final file in _pdfFiles.values) {
      unawaited(
        file
            .then((doc) async {
              await doc.close();
            })
            .catchError((_) {}),
      );
    }
    document?.removeListener(_changed);
    document?.dispose();
    _scene.dispose();
    _live.dispose();
    super.dispose();
  }

  Offset _world(Offset p) => (p - document!.camera) / document!.zoom;
  bool get _ink => _tool == StudyTool.pen || _tool == StudyTool.highlighter;
  bool get _writing =>
      _ink || _tool == StudyTool.eraser || _tool == StudyTool.region;
  void _cameraChanged() {
    document!.edit++;
    _camera.value++;
    final stage = _stageRect, view = _visibleWorld;
    if (stage == null ||
        !stage.contains(view.topLeft) ||
        !stage.contains(view.bottomRight)) {
      setState(() {});
    }
    _saveTimer?.cancel();
    _saveTimer = Timer(const Duration(milliseconds: 700), () {
      if (!mounted) return;
      setState(() {}); // Re-cull only after the gesture settles.
      unawaited(flush());
    });
  }

  bool _selectsPdf(String id) =>
      _tool == StudyTool.region ||
      (_tool == StudyTool.select &&
          _selection.contains(id) &&
          _detail.objectId == null);

  bool _overControls(
    Offset global, {
    bool expandedInk = false,
    bool includePdf = true,
  }) {
    for (final key in [
      _dockKey,
      _aiDockKey,
      _zoomKey,
      _selectionMenuKey,
      _returnKey,
      _reviewKey,
      _moduleKey,
      _toolbarKey,
      _topBarKey,
      _shelfKey,
      ..._stageKeys,
      if (includePdf)
        for (final entry in _pdfBodies.entries)
          if (!_selectsPdf(entry.key) &&
              (entry.value.currentState?.widget.root == null || expandedInk))
            entry.value,
    ]) {
      final box = key.currentContext?.findRenderObject() as RenderBox?;
      if (box != null &&
          box.hasSize &&
          box.size.contains(box.globalToLocal(global)))
        return true;
    }
    return false;
  }

  void _cancelCardHold() {
    _cardHoldTimer?.cancel();
    _cardHoldTimer = null;
    _cardHoldOrigin = null;
    _holdCue.value = null;
  }

  /// Moves the view to [camera] at [zoom] along one eased path. The point
  /// at the centre of the window travels in a straight line while the zoom
  /// changes geometrically, so the way there stays readable. Any touch,
  /// wheel or zoom stops it where it is.
  void _flyTo(Offset camera, double zoom) {
    final d = document!;
    _fly.stop();
    if (MediaQuery.disableAnimationsOf(context) ||
        ((d.camera - camera).distance < 1 && (d.zoom - zoom).abs() < .001)) {
      d
        ..zoom = zoom
        ..camera = camera;
      _cameraChanged();
      return;
    }
    final centre = _viewport.center(Offset.zero);
    final z0 = d.zoom, from = (centre - d.camera) / z0;
    final to = (centre - camera) / zoom;
    _flyStep = () {
      if (document != d) return _fly.stop();
      final t = Motion.emphasized.transform(_fly.value);
      d.zoom = z0 * math.pow(zoom / z0, t);
      d.camera = centre - Offset.lerp(from, to, t)! * d.zoom;
      _cameraChanged();
    };
    _fly.forward(from: 0);
  }

  void _down(PointerDownEvent e) {
    final d = document!;
    // Controls remain usable by the other hand during pen contact.
    if (_overControls(e.position, includePdf: false) ||
        _videoControlPointers.contains(e.pointer))
      return;
    final point = _world(e.localPosition);
    final card = d.cards
        .where(
          (c) =>
              c.type != 'conversation' &&
              d.visible(c.layer) &&
              c.containsWorld(point),
        )
        .lastOrNull;
    final actions = card == null
        ? null
        : _cardHeaderActions[card.id]?.currentContext?.findRenderObject()
              as RenderBox?;
    if (actions != null &&
        actions.hasSize &&
        actions.size.contains(actions.globalToLocal(e.position)))
      return;
    _trackStylusPresence(e);
    if (e.kind == PointerDeviceKind.touch &&
        (_touchBlocked || _blockedTouches.contains(e.pointer))) {
      _blockedTouches.add(e.pointer);
      return;
    }
    _cancelCardHold();
    _realStylus(e);
    _fly.stop();
    if (_drawingPointer != null && _drawingPointer != e.pointer) return;
    final stylus =
        e.kind == PointerDeviceKind.stylus ||
        e.kind == PointerDeviceKind.invertedStylus;
    _pointers[e.pointer] = e.localPosition;
    if (e.kind == PointerDeviceKind.touch) _touchPointers.add(e.pointer);
    if (_touchPointers.length == 2 && _drawingPointer == null) {
      _finishTransform();
      _marquee = null;
      _lasso.clear();
      _selectionDragging = false;
      final touches = _touchPointers.map((id) => _pointers[id]!).toList();
      _pinchDistance = (touches.first - touches.last).distance;
      _pinchZoom = d.zoom;
      _pinchWorld = _world((touches.first + touches.last) / 2);
      _panCamera = null;
      setState(() => _pdfPinching = true);
      return;
    }
    if (_pointers.length > 1) return;
    final local = card == null
        ? Offset.zero
        : StudyCardTransform(card).toLocal(point);
    final header = card != null && local.dy < 36;
    final paper =
        card != null &&
        _inlinePDF &&
        (card.type == 'pdfdoc' || _pdfPageCard(card)) &&
        !header;
    final reading =
        card != null &&
        !header &&
        (paper ||
            card.readerCard ||
            const {
              'text',
              'note',
              'explanation',
              'knowledge',
              'practice',
            }.contains(card.type));
    final pan =
        e.buttons == kMiddleMouseButton ||
        (e.kind == PointerDeviceKind.mouse &&
            HardwareKeyboard.instance.logicalKeysPressed.contains(
              LogicalKeyboardKey.space,
            ));
    final owner = studyContactOwner(
      kind: e.kind,
      header: header,
      paper: paper,
      reading: reading,
      inkTool: _ink || _tool == StudyTool.eraser,
      lasso: _tool == StudyTool.region,
      selectionHandle: _selectionReady && _selectionBounds != null &&
        studySelectionHandleAt(_selectionBounds!, point, 16 / d.zoom) != null,
      pan: pan,
    );
    _start = _last = point;
    if (owner == StudyContactOwner.card) {
      if (card == null || !d.canTransformCard(card)) return;
      if (!_selection.contains(card.id))
        _selection
          ..clear()
          ..add(card.id);
      _detail.clear();
      _selectionReady = false;
      _regionSelection = null;
      _selectionPolygon = [];
      _selectionBounds = null;
      _dragCard = card.id;
      _transformSession = StudyTransformSession(
        d,
        Set.of(_selection),
        {},
        pivot: card.rect.center,
      );
      _transformStart = point;
      _saveTimer?.cancel();
      setState(() {});
      return;
    }
    if (owner == StudyContactOwner.paper ||
        owner == StudyContactOwner.reading) {
      _readingContacts.add(e.pointer);
      _start = _last = null;
      return;
    }
    if (owner == StudyContactOwner.selection) {
      final selected = _selectionReady && _selectionBounds != null;
      final handle = selected
          ? studySelectionHandleAt(_selectionBounds!, point, 16 / d.zoom)
          : null;
      if (handle != null) {
        _selectionHandle = handle;
        _selectionResizeStart = _selectionBounds;
        _selectionResizePolygon = List.of(_selectionPolygon);
        _selectionDragging = true;
      } else if (selected &&
          _selectionBounds!.contains(point) &&
          (_selection.isNotEmpty || _detail.strokeIds.isNotEmpty)) {
        _transformSession = StudyTransformSession(
          d,
          Set.of(_selection),
          Set.of(_detail.strokeIds),
          pivot: _selectionBounds!.center,
        );
        _transformStart = point;
        _selectionDragging = true;
      } else {
        _selectionReady = false;
        _selectionBounds = null;
        _detail.clear();
        _selection.clear();
        _regionSelection = null;
        _lasso
          ..clear()
          ..add(point);
        _marquee = Rect.fromPoints(point, point);
      }
      _scene.value++;
      setState(() {});
      return;
    }
    if (owner == StudyContactOwner.ink) {
      if (_tool == StudyTool.select && stylus) _tool = StudyTool.pen;
      final temporary =
          e.kind == PointerDeviceKind.invertedStylus ||
          (_penButtons && stylus && (e.buttons & kPrimaryStylusButton) != 0);
      if (_tool == StudyTool.eraser || temporary) {
        _temporaryEraser = temporary;
        _beginErase(e, point);
        return;
      }
      if (!d.editable(d.activeLayer)) {
        _toast(l10n.studyLayerUnavailable);
        return;
      }
      final used = d.strokes.fold<int>(0, (n, s) => n + s.points.length);
      if (d.strokes.length >= 29000 || used >= 490000) {
        _toast(l10n.studyInkLimit);
        return;
      }
      _pointBudget = math.min(20000, 499900 - used);
      _drawingPointer = e.pointer;
      _pressureStart = e.localPosition;
      _resetShape();
      _gestureBrush = _brush;
      _gestureWidth = _penWidth;
      _gestureColor = _color;
      _points = [InkPoint(point, _pressure(e))];
      _prefix.clear();
      _prefixEnd = 0;
      _anchor = card?.id ?? '';
      _watchPressure(e);
      _watchShape(e);
      _live.value++;
      setState(() {});
      return;
    }
    if (e.kind == PointerDeviceKind.mouse && card != null && !pan) {
      _selection
        ..clear()
        ..add(card.id);
      _detail.clear();
      _regionSelection = null;
      setState(() {});
      return;
    }
    if (card == null) {
      _selection.clear();
      _detail.clear();
      _selectionReady = false;
      _selectionBounds = null;
    }
    _panCamera = d.camera;
    _start = e.localPosition;
    _scene.value++;
    setState(() {});
  }

  double _pressure(PointerEvent e) =>
      e.kind == PointerDeviceKind.stylus ||
          e.kind == PointerDeviceKind.invertedStylus
      ? ((e.pressure - e.pressureMin) /
                math.max(.001, e.pressureMax - e.pressureMin))
            .clamp(.05, 1)
      : .7;
  void _move(PointerMoveEvent e) {
    _trackStylusPresence(e);
    if (_drawingPointer == e.pointer) _realStylus(e);
    final d = document!;
    if (!_pointers.containsKey(e.pointer)) return;
    if (_cardHoldOrigin != null) {
      final pen = e.kind == PointerDeviceKind.stylus;
      if ((e.localPosition - _cardHoldOrigin!).distance > (pen ? 6 : 8)) {
        _cancelCardHold();
      } else if (!pen && _panCamera == null) {
        _pointers[e.pointer] = e.localPosition;
        return;
      }
    }
    _pointers[e.pointer] = e.localPosition;
    if (!_gesturing.value) _gesturing.value = true;
    if (_pointers.length == 2 &&
        _pinchDistance != null &&
        _drawingPointer == null) {
      final p = _pointers.values.toList();
      d.zoom =
          (_pinchZoom! * (p[0] - p[1]).distance / math.max(1, _pinchDistance!))
              .clamp(.05, 8);
      d.camera = (p[0] + p[1]) / 2 - _pinchWorld! * d.zoom;
      _cameraChanged();
      return;
    }
    if (_pointers.length > 1) return;
    if (_readingContacts.contains(e.pointer)) return;
    final p = _world(e.localPosition);
    if (_pdfPinching) return;
    if (_panCamera != null) {
      d.camera = _panCamera! + e.localPosition - _start!;
      _cameraChanged();
      return;
    }
    if (_drawingPointer == e.pointer) {
      if (_shape != null) {
        _adjustShape(p);
        return;
      }
      _watchPressure(e);
      _watchShape(e);
      final previous = _points.last,
          delta = (p - previous.p) * (1 - _gestureBrush.smoothing * .7);
      if (delta.distance < .4 / d.zoom || _points.length >= _pointBudget)
        return;
      final steps = _gestureBrush.family == 'highlighter'
          ? 1
          : (delta.distance / 4).ceil().clamp(
              1,
              math.min(100, _pointBudget - _points.length),
            );
      for (var i = 1; i <= steps; i++) {
        final t = i / steps;
        _points.add(
          InkPoint(
            previous.p + delta * t,
            previous.pressure + (_pressure(e) - previous.pressure) * t,
          ),
        );
      }
      // Painting translucent chunks separately leaves dark bands at their joins.
      while (_gestureBrush.family != 'highlighter' &&
          _points.length - _prefixEnd > 65) {
        _prefix.add(
          StudyStroke(
            id: studyId(),
            layer: d.activeLayer,
            points: _points.sublist(_prefixEnd, _prefixEnd + 65),
            color: _gestureColor,
            width: _gestureWidth,
            brush: _gestureBrush,
            tool: _tool == StudyTool.highlighter ? 'highlighter' : 'pen',
            anchor: _anchor,
            page:
                d.cards
                    .where((c) => c.id == _anchor && c.type == 'pdfdoc')
                    .firstOrNull
                    ?.page ??
                0,
          ),
        );
        _prefixEnd += 64;
        _scene.value++;
      }
      _live.value++;
      return;
    }
    if (_erasing) {
      _updateEraserCursor(e.localPosition);
      if (_eraserMode == 'lasso') {
        if (_eraserLasso.length < 1024 &&
            (_eraserLasso.last - p).distance > 3 / d.zoom)
          _eraserLasso.add(p);
        _cursor.value++;
      } else {
        _eraseAt(_last!, p);
      }
      _last = p;
      return;
    }
    if (_selectionDragging && _selectionBounds != null) {
      if (_selectionHandle != null) {
        final to = studyResizeSelection(
          _selectionResizeStart!,
          _selectionHandle!,
          p - _start!,
          24 / d.zoom,
        );
        final original = _selectionResizeStart!;
        final polygon = _selectionResizePolygon.isEmpty
            ? [to.topLeft, to.topRight, to.bottomRight, to.bottomLeft]
            : _selectionResizePolygon
                  .map(
                    (point) => Offset(
                      to.left +
                          (point.dx - original.left) *
                              to.width /
                              original.width,
                      to.top +
                          (point.dy - original.top) *
                              to.height /
                              original.height,
                    ),
                  )
                  .toList();
        _setRegionSelection(to, polygon);
        _update(() {});
      } else {
        _transformSession?.preview(translation: p - _transformStart);
        _scene.value++;
        setState(() {});
      }
      _last = p;
      return;
    }
    if (_dragCard != null) {
      _moveHeldCard(p);
      return;
    }
    if (_marquee != null) {
      _marquee = _lassoRectangle
          ? Rect.fromPoints(_start!, p)
          : _marquee!.expandToInclude(Rect.fromPoints(p, p));
      if (_lasso.length < 2048 && (_lasso.last - p).distance > 2 / d.zoom)
        _lasso.add(p);
      _cursor.value++;
    }
  }

  void _moveHeldCard(Offset p) {
    _transformSession?.preview(translation: p - _transformStart);
    _scene.value++;
    setState(() {});
  }

  Widget _videoInput(Widget child) => VideoCardHoldScope(
    writing: _writing,
    onSurfaceDown: (e) => _videoSurfacePointers.add(e.pointer),
    onControlDown: (e) => _videoControlPointers.add(e.pointer),
    onSurfaceEnd: (e) {
      _videoSurfacePointers.remove(e.pointer);
      _videoControlPointers.remove(e.pointer);
    },
    child: child,
  );

  void _up(PointerEvent e) {
    _trackStylusPresence(e);
    _blockedTouches.remove(e.pointer);
    _readingContacts.remove(e.pointer);
    final d = document!;
    if (!_pointers.containsKey(e.pointer)) return;
    if (_transformSession != null && _transformPointer == null)
      _finishTransform(cancel: e is PointerCancelEvent);
    _cancelCardHold();
    _touchPointers.remove(e.pointer);
    if (_pdfPinching && _touchPointers.isEmpty)
      setState(() => _pdfPinching = false);
    if (_drawingPointer == e.pointer) {
      _clearPrediction();
      _lastRealStylusTime = null;
      _cancelShapeHold();
    }
    _pressureTimer?.cancel();
    _pressureTimer = null;
    _pressureOrigin = null;
    _pressureStart = null;
    _pointers.remove(e.pointer);
    if (_pointers.isEmpty) _gesturing.value = false;
    if (e is PointerCancelEvent && _drawingPointer == e.pointer) {
      _resetShape();
      _points = [];
      _prefix.clear();
      _prefixEnd = 0;
      _drawingPointer = null;
      _scene.value++;
      _live.value++;
    }
    if (_drawingPointer == e.pointer) {
      final d = document!;
      d.addStroke(
        StudyStroke(
          id: studyId(),
          layer: d.activeLayer,
          points: List.of(_points),
          color: _gestureColor,
          width: _gestureWidth,
          brush: _gestureBrush,
          tool: _tool == StudyTool.highlighter ? 'highlighter' : 'pen',
          anchor: _anchor,
          page:
              d.cards
                  .where((c) => c.id == _anchor && c.type == 'pdfdoc')
                  .firstOrNull
                  ?.page ??
              0,
        ),
      );
      _commitShape();
      final added = d.strokes.last;
      _detail.clear();
      _detail.strokeIds.addAll(
        d.strokes.where((s) => s.gesture == added.gesture).map((s) => s.id),
      );
      var region = d.strokes
          .where((s) => _detail.strokeIds.contains(s.id))
          .map((s) => s.bounds)
          .reduce((a, b) => a.expandToInclude(b))
          .inflate(16);
      _selectionPolygon = [];
      _regionSelection = region;
      _selection.clear();
      final anchored = d.cards.where((c) => c.id == _anchor).firstOrNull;
      if (anchored != null) {
        _selection.add(anchored.id);
        _detail.objectId = anchored.id;
        region = region.intersect(_sourceBounds(anchored));
        _detail.region = _normalizeRegion(region, _sourceBounds(anchored));
      } else {
        _detail.region = region;
      }
      _points = [];
      _prefix.clear();
      _prefixEnd = 0;
      _drawingPointer = null;
      _resetShape();
      _scene.value++;
      _live.value++;
    }
    if (_pointers.length == 1 && _pinchDistance != null) {
      _pinchDistance = null;
      _panCamera = d.camera;
      _start = _pointers.values.single;
    }
    if (_pointers.isEmpty) {
      if (_erasing && _eraserMode == 'lasso' && e is! PointerCancelEvent) {
        d.eraseLasso(_eraserLasso);
        _eraserLasso.clear();
      }
      if (_tool == StudyTool.region &&
          _marquee != null &&
          e is! PointerCancelEvent) {
        _finishSelection();
      }
      if (_selectionHandle != null &&
          e is PointerCancelEvent &&
          _selectionResizeStart != null) {
        _setRegionSelection(_selectionResizeStart!, _selectionResizePolygon);
      }
      // The composer's screenshot follows the resized region.
      if (_selectionHandle != null && _quickAsk && _quickImages) {
        unawaited(_previewQuickVision());
      }
      _selectionHandle = null;
      _selectionResizeStart = null;
      _selectionResizePolygon = [];
      _selectionDragging = false;
      _temporaryEraser = false;
      _eraserCursor = null;
      _cursor.value++;
      _lasso.clear();
      _eraserLasso.clear();
      _panCamera = null;
      _start = _last = null;
      _marquee = null;
      _dragCard = null;
      _erasing = false;
      _pinchDistance = null;
      _scene.value++;
      setState(() {});
    }
  }

  void _scroll(PointerSignalEvent e) {
    if (e is! PointerScrollEvent ||
        _drawingPointer != null ||
        _overControls(e.position))
      return;
    GestureBinding.instance.pointerSignalResolver.register(e, (_) {
      _fly.stop();
      final d = document!;
      if (HardwareKeyboard.instance.isMetaPressed ||
          HardwareKeyboard.instance.isControlPressed) {
        final p = _world(e.localPosition);
        d.zoom = (d.zoom * math.exp(-e.scrollDelta.dy * .002)).clamp(.05, 8);
        d.camera = e.localPosition - p * d.zoom;
      } else {
        d.camera -= e.scrollDelta;
      }
      _cameraChanged();
    });
  }

  void _zoomBy(double factor) {
    _fly.stop();
    final d = document!,
        center = _viewport.center(Offset.zero),
        p = _world(_viewport.center(Offset.zero));
    d.zoom = (d.zoom * factor).clamp(.05, 8);
    d.camera = center - p * d.zoom;
    _cameraChanged();
  }

  void _fit() {
    final d = document!;
    final cards = d.cards.where(
      (a) => a.type != 'conversation' && d.visible(a.layer),
    );
    var r = cards.isEmpty
        ? const Rect.fromLTWH(0, 0, 880, 550)
        : cards.first.rect;
    for (final c in cards) {
      r = r.expandToInclude(c.rect);
    }
    final zoom = math
        .min(
          (_viewport.width - 100) / r.width,
          (_viewport.height - 100) / r.height,
        )
        .clamp(.05, 1.5)
        .toDouble();
    _flyTo(_viewport.center(Offset.zero) - r.center * zoom, zoom);
  }

  void _toast(String s) {
    if (mounted)
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(s)));
  }

  Future<void> _setPDFView(StudyCard c, String view) async {
    if (_importing ||
        !await flush() ||
        !mounted ||
        !document!.cards.contains(c))
      return;
    if (view == 'column') {
      await _expandPDFColumn(c);
      return;
    }
    try {
      if (c.pdfColumn.isNotEmpty)
        document!.collapsePDFColumn(c.id);
      else
        document!.checkpoint();
      c.pdfPaged = view == 'paged';
      if (c.collapsed) {
        c.collapsed = false;
        c.rect = Rect.fromLTWH(
          c.rect.left,
          c.rect.top,
          c.rect.width,
          math.max(240, c.expandedH),
        );
      }
      document!.changed();
      await flush();
    } catch (e) {
      if (mounted) _toast('$e');
    }
  }

  Future<dynamic> _loadPDFPage(StudyCard card, int page) => widget.standalone
      ? widget.api.get('/api/notebooks/${widget.notebookId}/pdf-page', {'source': card.source, 'page': '$page'})
      : widget.api.studyPDFPage(widget.ctl.lectureId, card.source, page);

  Future<Map> _expandedPage(StudyCard root, int page) async {
    try {
      return await _loadPDFPage(root, page) as Map;
    } catch (_) {
      if (!_inlinePDF) rethrow;
    }
    final pdf = await _openPaper(root);
    final rel = filePath.join(
      filePath.dirname(root.source),
      'canvas-page-$page.png',
    );
    final target = File(filePath.join(widget.pdfLibraryRoot!, rel));
    await target.parent.create(recursive: true);
    if (!await target.exists()) {
      final tmp = File('${target.path}.${studyId()}.tmp');
      try {
        await tmp.writeAsBytes(await pdf.render(page), flush: true);
        await tmp.rename(target.path);
      } finally {
        if (await tmp.exists()) await tmp.delete();
      }
    }
    final size = pdf.sizes[page - 1];
    return {
      'file': rel,
      'width': size.width,
      'height': size.height,
      'text': page == root.page ? root.text : '',
    };
  }

  Future<void> _expandPDFColumn(StudyCard root) async {
    if (_importing) return;
    final d = document!;
    setState(() => _importing = true);
    try {
      d.beginPDFColumn(root);
      if (!await flush()) return;
      for (var page = 1; page <= root.pageCount; page++) {
        if (!mounted || !d.cards.contains(root) || root.pdfColumn != root.id)
          return;
        if (d.pdfPages(root.id).any((c) => c.page == page)) continue;
        setState(() => _status = '正在展开课件 $page / ${root.pageCount} 页');
        final data = await _expandedPage(root, page);
        if (!mounted || !d.cards.contains(root) || root.pdfColumn != root.id)
          return;
        d.appendPDFPage(root, page, data);
        if (!await flush()) return;
      }
    } catch (e) {
      if (mounted) _toast('展开未完成：$e。已展开的页面和笔迹已保留，可点击“继续展开整列”重试');
    } finally {
      if (mounted)
        setState(() {
          _importing = false;
        });
    }
  }

  Future<void> _collapsePDFColumn(String id) async {
    try {
      document!.collapsePDFColumn(id);
      await flush();
    } catch (e) {
      if (mounted) _toast('$e');
    }
  }

  Future<void> _import({
    String? courseware,
    bool wholeCourse = false,
    Offset? at,
  }) async {
    if (_importing) return;
    XFile? file;
    if (courseware == null) {
      file = await openFile(
        acceptedTypeGroups: [
          XTypeGroup(
            label: l10n.studyMaterials,
            extensions: [
              'pdf',
              'png',
              'jpg',
              'jpeg',
              'gif',
              'webp',
              'txt',
              'md',
              'markdown',
              'mp4',
              'mov',
              'm4v',
              'webm',
              'mkv',
            ],
          ),
        ],
      );
      if (!mounted || file == null) return;
    }
    final d = document!;
    if (!d.editable(d.activeLayer)) {
      _toast(l10n.studyPickLayer);
      return;
    }
    setState(() {
      _importing = true;
      _status = l10n.studyImporting;
    });
    try {
      dynamic out;
      if (courseware != null) {
        out = wholeCourse
            ? await widget.api.importStudyMaterial(
                widget.ctl.lectureId,
                courseware,
              )
            : await widget.api.importStudyCourseware(
                widget.ctl.lectureId,
                courseware,
              );
      } else {
        final length = await file!.length();
        if (length > 300 << 20) throw FormatException(l10n.studyFileTooLarge);
        out = widget.standalone
            ? await widget.api.upload(
                '/api/notebooks/${widget.notebookId}/assets',
                {'name': file.name, 'paged': '1'},
                file.openRead(),
                length,
              )
            : await widget.api.importStudyAsset(
                widget.ctl.lectureId,
                file.name,
                file.openRead(),
                length,
              );
      }
      if (!mounted) return;
      final cards = [
        for (final a in out['objects'] as List) StudyCard.fromJson(a as Map),
      ];
      if (cards.length + d.cards.length > 2000)
        throw FormatException(l10n.studyCardLimit);
      final center = at ?? _world(_viewport.center(Offset.zero));
      d.checkpoint();
      var y = center.dy - (at == null ? 120 : 60);
      for (final c in cards) {
        c.layer = d.activeLayer;
        c.rect = Rect.fromLTWH(center.dx - 160, y, c.rect.width, c.rect.height);
        y += c.rect.height + 36;
      }
      d.cards.addAll(cards);
      _selection
        ..clear()
        ..add(cards.first.id);
      d.changed();
      _tool = StudyTool.select;
      await flush();
    } catch (e) {
      _toast(l10n.studyImportError('$e'));
    } finally {
      if (mounted)
        setState(() {
          _importing = false;
          if (_saveError == null) _status = l10n.studySaved;
        });
    }
  }

  Future<void> _text([StudyCard? card]) async {
    final input = TextEditingController(text: card?.text ?? '');
    final text = await showDialog<String>(
      context: context,
      builder: (c) => AlertDialog(
        title: Text(card == null ? l10n.studyAddText : l10n.studyEditText),
        content: SizedBox(
          width: 500,
          child: TextField(
            controller: input,
            autofocus: true,
            minLines: 5,
            maxLines: 15,
            maxLength: 200000,
            decoration: InputDecoration(hintText: l10n.studyTextHint),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(c),
            child: Text(l10n.commonCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(c, input.text),
            child: Text(l10n.commonSave),
          ),
        ],
      ),
    );
    // Dialog animations can retain the input after the route result completes.
    Future.delayed(const Duration(milliseconds: 400), input.dispose);
    if (!mounted || text == null) return;
    final d = document!;
    if (!d.editable(card?.layer ?? d.activeLayer)) {
      _toast(l10n.studyLocked);
      return;
    }
    d.checkpoint();
    if (card != null) {
      card.text = text;
    } else {
      final p = _world(_viewport.center(Offset.zero));
      final c = StudyCard(
        id: studyId(),
        type: 'text',
        layer: d.activeLayer,
        rect: Rect.fromLTWH(p.dx - 100, p.dy - 100, 480, 360),
        title: l10n.studyTextCard,
        text: text,
      );
      d.cards.add(c);
      _selection
        ..clear()
        ..add(c.id);
    }
    d.changed();
  }

  Future<void> _ask() async {
    if (_selectionReady &&
        _selectionBounds != null &&
        _tool == StudyTool.region) {
      _update(() => _quickAsk = true);
      if (_quickImages) await _previewQuickVision();
    } else {
      await _openConversation();
    }
  }

  /// Sends the quick composer's question, or with [outline] drafts a review
  /// outline of the bound material, the question becoming its brief.
  Future<void> _sendQuickQuestion({bool outline = false}) async {
    final text = _quickInput.text.trim();
    if ((text.isEmpty && !outline) || _quickSending) return;
    _update(() {
      _quickSending = true;
      _quickError = null;
    });
    try {
      if (_selection.isEmpty &&
          _detail.objectId == null &&
          _detail.strokeIds.isEmpty) {
        throw FormatException(l10n.studySelectionNeedsSource);
      }
      await _startConversation(
        send: !outline
            ? text
            : text.isEmpty
            ? l10n.outlineAsk
            : '${l10n.outlineAsk}：$text',
        task: outline ? 'propose_outline' : null,
        allowImages: _quickImages,
        snapshot: _quickImages ? _quickSnapshot : null,
      );
      if (!mounted) return;
      _quickInput.clear();
      _update(() {
        _quickAsk = false;
        _quickImages = false;
        _quickSnapshot = null;
        _tool = StudyTool.select;
      });
    } catch (e) {
      if (mounted) _update(() => _quickError = l10n.studyQuickSendFailed('$e'));
    } finally {
      if (mounted) _update(() => _quickSending = false);
    }
  }

  /// The player's full side panel (outline, AI tools) as a dialog.
  void _openPlayerPanel() => showDialog<void>(
    context: context,
    builder: (_) => Dialog(
      child: SizedBox(
        width: 480,
        height: math.min(600, _viewport.height - 48),
        child: widget.panel,
      ),
    ),
  );

  void _layers() {
    showModalBottomSheet(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, update) => SafeArea(
          child: SizedBox(
            height: 330,
            child: Column(
              children: [
                ListTile(
                  title: Text(l10n.studyLayers),
                  trailing: IconButton(
                    tooltip: l10n.studyAddLayer,
                    icon: const Icon(Icons.add),
                    onPressed: () {
                      final d = document!;
                      if (d.layers.length >= 32) return;
                      d.checkpoint();
                      final l = StudyLayer(
                        studyId(),
                        l10n.studyLayerName(d.layers.length + 1),
                      );
                      d.layers.add(l);
                      d.activeLayer = l.id;
                      d.changed();
                      update(() {});
                    },
                  ),
                ),
                Expanded(
                  child: ListView(
                    children: [
                      for (final l in document!.layers)
                        ListTile(
                          selected: document!.activeLayer == l.id,
                          title: Text(l.name),
                          onTap: () {
                            setState(() => document!.activeLayer = l.id);
                            update(() {});
                          },
                          leading: IconButton(
                            tooltip: l10n.studyToggleVisible,
                            icon: Icon(
                              l.visible
                                  ? Icons.visibility_outlined
                                  : Icons.visibility_off_outlined,
                            ),
                            onPressed: () {
                              document!.checkpoint();
                              l.visible = !l.visible;
                              document!.changed();
                              update(() {});
                            },
                          ),
                          trailing: IconButton(
                            tooltip: l10n.studyToggleLocked,
                            icon: Icon(
                              l.locked
                                  ? Icons.lock_outline
                                  : Icons.lock_open_outlined,
                            ),
                            onPressed: () {
                              document!.checkpoint();
                              l.locked = !l.locked;
                              document!.changed();
                              update(() {});
                            },
                          ),
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> exportRecovery() async {
    if (document == null) return;
    final dest = await getSaveLocation(
      suggestedName: '学习画布-${widget.notebookId ?? widget.ctl.lectureId}.json',
    );
    if (dest == null || !mounted) return;
    try {
      await File(dest.path).writeAsString(jsonEncode(document!.json()));
      _toast(l10n.studyRecoverySaved);
    } catch (e) {
      _toast(l10n.studyExportError('$e'));
    }
  }

  @override
  Widget build(BuildContext context) {
    if (widget.fullscreen) return widget.video;
    if (_loadError != null)
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('画布打开失败：$_loadError'),
            TextButton(onPressed: _load, child: const Text('重试')),
          ],
        ),
      );
    final d = document;
    if (d == null) return const Center(child: CircularProgressIndicator());
    _enabledPlugins = _activePlugins();
    _hasCanvasTools = _canvasToolContributions(context).isNotEmpty;
    return LayoutBuilder(
      builder: (_, size) {
        _viewport = Size(size.maxWidth, size.maxHeight);
        _measureSelectionControls();
        _selectionToolbarDocked =
            !_snapshotting &&
            _selectionReady &&
            !_quickAsk &&
            _tool == StudyTool.region &&
            _selectionBounds != null &&
            _selectionMenuRect() == null;
        if (_selectionToolbarDocked) {
          _viewport = Size(size.maxWidth, math.max(0, size.maxHeight - 56));
        }
        return Column(
          children: [
            Expanded(child: _canvas(d)),
            if (_selectionToolbarDocked)
              SizedBox(height: 56, child: _selectionToolbar(docked: true)),
          ],
        );
      },
    );
  }

  void _moveDock(String id, StudyDockPlacement next) {
    setState(() {
      switch (id) {
        case 'ai':
          _aiPlacement = next;
          _stageOpen = true;
        default:
          _penPlacement = next;
          _dockSmall = false;
      }
    });
    unawaited(_saveLayout());
  }

  Rect get _visibleWorld => Rect.fromLTWH(
    -document!.camera.dx / document!.zoom,
    -document!.camera.dy / document!.zoom,
    _viewport.width / document!.zoom,
    _viewport.height / document!.zoom,
  );

  Widget _canvas(StudyDocument d) {
    final cardIds = d.cards.map((c) => c.id).toSet();
    _cardHeaderActions.removeWhere((id, _) => !cardIds.contains(id));
    _pdfBodies.removeWhere((id, _) => !cardIds.contains(id));
    final stage = _visibleWorld.inflate(
      math.max(_viewport.width, _viewport.height) / d.zoom,
    );
    _stageRect = stage;
    return RepaintBoundary(
      key: _inputSurface,
      child: ClipRect(
        child: Stack(
          fit: StackFit.expand,
          children: [
            // Only the paper's hit-test path can start a drawing gesture.
            Listener(
              key: const ValueKey('study-input'),
              behavior: HitTestBehavior.translucent,
              onPointerHover: (e) {
                _trackStylusPresence(e);
                if (_onPdf(e.position)) {
                  _clearPaperCursor();
                  return;
                }
                if (_tool == StudyTool.eraser ||
                    e.kind == PointerDeviceKind.invertedStylus)
                  _updateEraserCursor(e.localPosition);
              },
              onPointerDown: _down,
              onPointerMove: _move,
              onPointerUp: _up,
              onPointerCancel: _up,
              onPointerSignal: _scroll,
              onPointerPanZoomStart: (e) {
                if (_drawingPointer != null ||
                    _erasing ||
                    _overControls(e.position, includePdf: false))
                  return;
                _trackpadOnPdf = _onPdf(e.position);
                _fly.stop();
                _pinchZoom = d.zoom;
                _pinchWorld = _world(e.localPosition);
                _panCamera = d.camera;
              },
              onPointerPanZoomUpdate: (e) {
                if (_pinchWorld == null ||
                    _pinchZoom == null ||
                    _panCamera == null)
                  return;
                if (_trackpadOnPdf && e.scale == 1) return;
                d.zoom = (_pinchZoom! * e.scale).clamp(.05, 8);
                d.camera = e.localPosition + e.pan - _pinchWorld! * d.zoom;
                _cameraChanged();
              },
              onPointerPanZoomEnd: (_) {
                _pinchWorld = null;
                _panCamera = null;
              },
              child: MouseRegion(
                onExit: (_) {
                  _eraserCursor = null;
                  _cursor.value++;
                },
                child: Stack(
                  fit: StackFit.expand,
                  clipBehavior: Clip.hardEdge,
                  children: [
                    Positioned.fill(
                      child: RepaintBoundary(
                        key: _capture,
                        child: Stack(
                          fit: StackFit.expand,
                          clipBehavior: Clip.none,
                          children: [
                            Positioned.fill(
                              child: RepaintBoundary(
                                child: CustomPaint(
                                  painter: _Paper(
                                    document: d,
                                    repaint: _camera,
                                    style: _paperStyle,
                                    color: _paperColor,
                                    scheme: Theme.of(context).colorScheme,
                                  ),
                                ),
                              ),
                            ),
                            // Camera changes transform an already laid-out world. Media widgets and
                            // recorded ink are retained until the visible area leaves this bounded stage.
                            Positioned(
                              left: 0,
                              top: 0,
                              width: stage.width,
                              height: stage.height,
                              child: AnimatedBuilder(
                                animation: _camera,
                                builder: (_, child) => Transform(
                                  alignment: Alignment.topLeft,
                                  transform: Matrix4.identity()
                                    ..translate(
                                      d.camera.dx + stage.left * d.zoom,
                                      d.camera.dy + stage.top * d.zoom,
                                    )
                                    ..scale(d.zoom),
                                  child: child,
                                ),
                                child: Stack(
                                  clipBehavior: Clip.none,
                                  children: [
                                    Positioned.fill(
                                      child: IgnorePointer(
                                        child: RepaintBoundary(
                                          child: CustomPaint(
                                            painter: _ConnectionPainter(
                                              d,
                                              stage,
                                              _scene,
                                              chrome,
                                            ),
                                          ),
                                        ),
                                      ),
                                    ),
                                    for (final c in d.cards.where(
                                      (a) =>
                                          a.type != 'conversation' &&
                                          d.visible(a.layer) &&
                                          (a.type == 'lecture' ||
                                              a.type == 'video' ||
                                              a.visualBounds.overlaps(stage)),
                                    ))
                                      Positioned.fromRect(
                                        key: ValueKey('study-card/${c.id}'),
                                        rect: c.visualBounds.shift(
                                          -stage.topLeft,
                                        ),
                                        child: OverflowBox(
                                          minWidth: c.rect.width,
                                          maxWidth: c.rect.width,
                                          minHeight: c.rect.height,
                                          maxHeight: c.rect.height,
                                          child: Transform.rotate(
                                            angle: c.rotation,
                                            child: RepaintBoundary(
                                              child: RotatedBox(
                                                quarterTurns: c.quarterTurns,
                                                child: FittedBox(
                                                  fit: BoxFit.fill,
                                                  child: SizedBox(
                                                    width:
                                                        c.contentRect.width /
                                                        c.contentScale,
                                                    height:
                                                        c.contentRect.height /
                                                        c.contentScale,
                                                    child: _card(c),
                                                  ),
                                                ),
                                              ),
                                            ),
                                          ),
                                        ),
                                      ),
                                    for (final o in d.pluginObjects.where(
                                      (o) => o.rect.overlaps(stage),
                                    ))
                                      _pluginObject(
                                        o,
                                        stage,
                                        _enabledPlugins.contains(o.pluginId),
                                      ),
                                    Positioned.fill(
                                      child: IgnorePointer(
                                        child: RepaintBoundary(
                                          child: CustomPaint(
                                            painter: _InkScene(
                                              inlinePDF: _inlinePDF,
                                              document: d,
                                              live: _prefix,
                                              stage: stage,
                                              repaint: _scene,
                                              dark: _dark,
                                            ),
                                          ),
                                        ),
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                            Positioned.fill(
                              child: IgnorePointer(
                                child: RepaintBoundary(
                                  child: CustomPaint(
                                    painter: _InkTail(
                                      document: d,
                                      points: () =>
                                          _shapePreview ??
                                          (_predicted.isEmpty
                                              ? _points
                                              : [
                                                  ..._points.skip(_prefixEnd),
                                                  ..._predicted,
                                                ]),
                                      start: () =>
                                          _predicted.isEmpty ? _prefixEnd : 0,
                                      color: () =>
                                          _dark &&
                                              !studyOnPrintedPage(
                                                d.cards
                                                    .where(
                                                      (c) => c.id == _anchor,
                                                    )
                                                    .firstOrNull
                                                    ?.type,
                                              )
                                          ? inkOnDark(
                                              _gestureColor,
                                              _tool == StudyTool.highlighter
                                                  ? 'highlighter'
                                                  : 'pen',
                                            )
                                          : _gestureColor,
                                      width: () => _gestureWidth,
                                      brush: () => _gestureBrush,
                                      darkPaper: () =>
                                          _dark &&
                                          !studyOnPrintedPage(
                                            d.cards
                                                .where((c) => c.id == _anchor)
                                                .firstOrNull
                                                ?.type,
                                          ),
                                      tool: () => _tool,
                                      repaint: Listenable.merge([
                                        _live,
                                        _scene,
                                        _camera,
                                      ]),
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                    Positioned.fill(
                      child: IgnorePointer(
                        child: RepaintBoundary(
                          child: CustomPaint(
                            key: const ValueKey('study-feedback'),
                            painter: _InteractionFeedback(
                              this,
                              Listenable.merge([_cursor, _scene, _camera]),
                            ),
                          ),
                        ),
                      ),
                    ),
                    Positioned.fill(
                      child: IgnorePointer(
                        child: RepaintBoundary(
                          child: CustomPaint(
                            key: const ValueKey('study-evidence'),
                            painter: _EvidencePainter(this),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            if (Platform.isMacOS)
              const Positioned(
                top: 0,
                left: 80,
                right: 0,
                height: 22,
                child: DragToMoveArea(child: SizedBox.expand()),
              ),
            if (!widget.standalone) _assetDropTarget(),
            _dropOutline(),
            _HoldCue(at: _holdCue),
            _DockGroup(
              topInset: _topBarsBottom + 8,
              children: [
                _penDock(),
                if (!widget.standalone && !_stageOpen) _aiDock(),
              ],
            ),
            if (_tool == StudyTool.select ||
                _tool == StudyTool.region ||
                (_wholeCardSelected && _drawingPointer == null))
              _rotationHandle(),
            _fixedTopBars(),
            if (!widget.standalone) _agentStage(),
            if (!widget.standalone) _reviewPanel(),
            if (!widget.standalone) _returnPill(),
            if (!_selectionToolbarDocked) _selectionToolbar(),
            if (!widget.standalone) _shelf(),
            if (_viewport.width >= 600)
              Positioned(
                right: studyZoomInset,
                bottom: studyZoomInset,
                child: GlassSurface(
                  key: _zoomKey,
                  radius: Corners.full,
                  padding: const EdgeInsets.symmetric(horizontal: Space.xxs),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      IconButton(
                        tooltip: l10n.studyZoomOut,
                        visualDensity: VisualDensity.compact,
                        onPressed: () => _zoomBy(1 / 1.2),
                        icon: const Icon(Icons.remove_rounded, size: 18),
                      ),
                      ValueListenableBuilder<int>(
                        valueListenable: _camera,
                        builder: (_, value, child) => SizedBox(
                          width: 44,
                          child: Text(
                            '${(d.zoom * 100).round()}%',
                            textAlign: TextAlign.center,
                            style: Theme.of(context).textTheme.labelMedium
                                ?.copyWith(
                                  fontFeatures: const [
                                    FontFeature.tabularFigures(),
                                  ],
                                ),
                          ),
                        ),
                      ),
                      IconButton(
                        tooltip: l10n.studyZoomIn,
                        visualDensity: VisualDensity.compact,
                        onPressed: () => _zoomBy(1.2),
                        icon: const Icon(Icons.add_rounded, size: 18),
                      ),
                      IconButton(
                        tooltip: l10n.studyFit,
                        visualDensity: VisualDensity.compact,
                        onPressed: _fit,
                        icon: const Icon(Icons.fit_screen_outlined, size: 18),
                      ),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _card(StudyCard c) {
    final d = document!,
        selected = !_snapshotting && _selection.contains(c.id),
        chrome = StudyChrome.of(context),
        cs = Theme.of(context).colorScheme;
    final shape = RoundedSuperellipseBorder(
      borderRadius: BorderRadius.circular(Corners.lg),
      side: BorderSide(
        color: selected
            ? chrome.accent
            : cs.onSurface.withValues(alpha: _dark ? .12 : .08),
        width: selected ? 2 : 1,
      ),
    );
    Widget action(String tooltip, IconData icon, VoidCallback? onPressed) =>
        IconButton(
          tooltip: tooltip,
          onPressed: onPressed,
          iconSize: 16,
          visualDensity: VisualDensity.compact,
          constraints: const BoxConstraints.tightFor(width: 32, height: 32),
          padding: EdgeInsets.zero,
          color: cs.onSurfaceVariant,
          icon: Icon(icon),
        );
    final pdfColumn = c.type == 'pdfdoc' && c.pdfColumn.isNotEmpty;
    // Rare actions live in one menu so the header never runs out of room.
    final more = <(String, IconData, VoidCallback?)>[
      if (!c.generated)
        for (final source in c.sources)
          (
            l10n.studySelectionSource(
              source.page > 0 ? '${source.kind} · ${source.page}' : source.kind,
            ),
            Icons.link,
            () => _focusSource(source),
          ),
      if (!pdfColumn && c.quarterTurns == 0)
        (
          c.collapsed ? l10n.studyCardExpand : l10n.studyCardCollapse,
          c.collapsed ? Icons.unfold_more_rounded : Icons.unfold_less_rounded,
          () {
            d.checkpoint();
            c.collapsed = !c.collapsed;
            if (c.collapsed) {
              c.expandedH = c.rect.height;
              c.rect = Rect.fromLTWH(c.rect.left, c.rect.top, c.rect.width, 60);
            } else {
              c.rect = Rect.fromLTWH(
                c.rect.left,
                c.rect.top,
                c.rect.width,
                math.max(c.type == 'conversation' ? 160 : 240, c.expandedH),
              );
            }
            d.changed();
          },
        ),
      if (c.pdfColumn.isNotEmpty)
        (
          l10n.studyCardFoldColumn,
          Icons.vertical_align_center_rounded,
          _importing ? null : () => _collapsePDFColumn(c.pdfColumn),
        ),
      if (c.generated)
        (
          l10n.studyCardEditKnowledge,
          Icons.edit_outlined,
          () => _editKnowledge(c),
        ),
      if (c.type == 'text')
        (l10n.studyEditText, Icons.edit_outlined, () => _text(c)),
      if (c.type != 'lecture' && !pdfColumn)
        (
          l10n.studyCardDelete,
          Icons.delete_outline_rounded,
          () {
            d.checkpoint();
            d.cards.removeWhere((v) => v.id == c.id);
            d.connections.removeWhere((v) => v.from == c.id || v.to == c.id);
            d.strokes.removeWhere((v) => v.anchor == c.id);
            d.changed();
          },
        ),
    ];
    return IgnorePointer(
      ignoring: !d.editable(c.layer),
      child: Container(
        decoration: ShapeDecoration(
          color: chrome.paper,
          shape: shape,
          // Dark cards separate by a tonal step; a shadow would only muddy.
          shadows: _dark
              ? null
              : [
                  BoxShadow(
                    color: chrome.shadow,
                    blurRadius: 24,
                    offset: const Offset(0, 6),
                  ),
                ],
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          children: [
            SizedBox(
              height: 36,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: c.type == 'lecture'
                      ? chrome.accentSoft
                      : Colors.transparent,
                  border: Border(
                    bottom: BorderSide(
                      color: cs.onSurface.withValues(alpha: .08),
                    ),
                  ),
                ),
                child: Row(
                  children: [
                    const SizedBox(width: Space.md),
                    Icon(
                      switch (c.type) {
                        'lecture' || 'video' => Icons.smart_display_outlined,
                        'text' => Icons.notes_rounded,
                        'conversation' => Icons.forum_outlined,
                        'image' => Icons.image_outlined,
                        'note' ||
                        'explanation' ||
                        'knowledge' => Icons.lightbulb_outline_rounded,
                        'practice' => Icons.quiz_outlined,
                        _ => Icons.description_outlined,
                      },
                      size: 16,
                      color: c.type == 'conversation'
                          ? cs.primary
                          : cs.onSurfaceVariant,
                    ),
                    const SizedBox(width: Space.sm),
                    Expanded(
                      child: Tooltip(
                        message: c.title,
                        ignorePointer: true,
                        waitDuration: const Duration(milliseconds: 600),
                        child: Text(
                          c.type == 'pdf' && c.pdfColumn.isNotEmpty
                              ? l10n.studyCardPdfPage(
                                  c.title,
                                  c.page,
                                  c.pageCount,
                                )
                              : c.title,
                          style: Theme.of(context).textTheme.labelLarge
                              ?.copyWith(color: chrome.ink),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ),
                    Row(
                      key: _cardHeaderActions.putIfAbsent(
                        c.id,
                        () => GlobalKey(),
                      ),
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        if (_inlinePDF && c.type == 'pdfdoc')
                          PopupMenuButton<String>(
                            tooltip: l10n.pdfCardView,
                            iconSize: 17,
                            icon: const Icon(Icons.view_carousel_outlined),
                            enabled: !_importing,
                            onSelected: (view) => _setPDFView(c, view),
                            itemBuilder: (_) => [
                              for (final entry in {
                                'scroll': l10n.pdfCardScroll,
                                'paged': l10n.pdfCardPaged,
                                'column': l10n.pdfCardColumns,
                              }.entries)
                                CheckedPopupMenuItem(
                                  value: entry.key,
                                  checked:
                                      entry.key ==
                                      (pdfColumn
                                          ? 'column'
                                          : c.pdfPaged
                                          ? 'paged'
                                          : 'scroll'),
                                  child: Text(entry.value),
                                ),
                            ],
                          ),
                        if (!_inlinePDF &&
                            c.type == 'pdfdoc' &&
                            (c.pdfColumn.isEmpty ||
                                d.pdfPages(c.id).length < c.pageCount))
                          action(
                            c.pdfColumn.isEmpty ? '在画布展开为一列' : '继续展开整列',
                            Icons.view_agenda_outlined,
                            _importing ? null : () => _expandPDFColumn(c),
                          ),
                        if (c.type != 'conversation')
                          Draggable<String>(
                            data: c.id,
                            feedback: Material(
                              type: MaterialType.transparency,
                              child: GlassSurface(
                                depth: GlassDepth.float,
                                radius: Corners.full,
                                padding: const EdgeInsets.symmetric(
                                  horizontal: Space.md,
                                  vertical: Space.sm,
                                ),
                                child: Row(
                                  mainAxisSize: MainAxisSize.min,
                                  children: [
                                    Icon(
                                      Icons.add_comment_outlined,
                                      size: 16,
                                      color: cs.primary,
                                    ),
                                    const SizedBox(width: Space.sm),
                                    ConstrainedBox(
                                      constraints: const BoxConstraints(
                                        maxWidth: 220,
                                      ),
                                      child: Text(
                                        c.title,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                            child: action(
                              l10n.studyCardAsk,
                              Icons.add_comment_outlined,
                              () => _chooseConversation(c),
                            ),
                          ),
                        if (more.isNotEmpty)
                          MenuAnchor(
                            menuChildren: [
                              for (final (label, icon, run) in more)
                                MenuItemButton(
                                  leadingIcon: Icon(icon, size: 18),
                                  onPressed: run,
                                  child: Text(label),
                                ),
                            ],
                            builder: (context, menu, _) => action(
                              l10n.studyCardMore,
                              Icons.more_horiz_rounded,
                              () => menu.isOpen ? menu.close() : menu.open(),
                            ),
                          ),
                      ],
                    ),
                    Icon(
                      Icons.drag_indicator_rounded,
                      size: 16,
                      color: chrome.inkFaint,
                    ),
                    const SizedBox(width: Space.sm),
                  ],
                ),
              ),
            ),
            Expanded(
              child: OverflowBox(
                minHeight: c.collapsed ? math.max(480, c.expandedH) - 56 : null,
                maxHeight: c.collapsed ? math.max(480, c.expandedH) - 56 : null,
                child: Offstage(
                  offstage: c.collapsed,
                  child: IgnorePointer(
                    ignoring:
                        _tool == StudyTool.region ||
                        (_selectsPdf(c.id) &&
                            (c.type == "pdfdoc" || c.type == "pdf")),
                    child: switch (c.type) {
                      'pdfdoc' when _inlinePDF && !c.collapsed => _CanvasPdf(
                        key: _pdfBodies.putIfAbsent(
                          c.id,
                          () => GlobalKey<_CanvasPdfState>(),
                        ),
                        owner: this,
                        card: c,
                      ),
                      'pdfdoc' when _inlinePDF => const SizedBox.shrink(),
                      'pdfdoc' => _PDFBody(owner: this, card: c),
                      'conversation' => _ThreadNode(owner: this, card: c),
                      'lecture' => Column(
                        children: [
                          Expanded(
                            child: _snapshotVideo(c, _videoInput(widget.video)),
                          ),
                          TextButton.icon(
                            key: const ValueKey('study-video-shelf'),
                            style: TextButton.styleFrom(
                              minimumSize: const Size(0, 48),
                              padding: const EdgeInsets.symmetric(
                                horizontal: 16,
                                vertical: 10,
                              ),
                            ),
                            onPressed: () => _toggleShelf(_ShelfTab.transcript),
                            icon: const Icon(
                              Icons.subtitles_outlined,
                              size: 16,
                            ),
                            label: Text(
                              l10n.studyShelfFromVideo,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ],
                      ),
                      'pdf' when _pdfPageCard(c) => _CanvasPdf(
                        key: _pdfBodies.putIfAbsent(
                          c.id,
                          () => GlobalKey<_CanvasPdfState>(),
                        ),
                        owner: this,
                        card: c,
                        root: document!.cards.firstWhere(
                          (p) => p.id == c.pdfColumn,
                        ),
                      ),
                      'image' || 'pdf' => Image(
                        image: ResizeImage(
                          NetworkImage(widget.api.fileUrl(c.path)),
                          width: 1500,
                          height: 1500,
                          policy: ResizeImagePolicy.fit,
                        ),
                        fit: BoxFit.contain,
                        errorBuilder: (_, error, stack) =>
                            Center(child: Text(l10n.studyImageFailed)),
                      ),
                      'video' => _snapshotVideo(
                        c,
                        _videoInput(
                          _LocalVideo(
                            key: _localVideoKeys.putIfAbsent(
                              c.id,
                              () => GlobalKey<_LocalVideoState>(),
                            ),
                            url: c.path.isEmpty
                                ? widget.api.studyMediaUrl(
                                    widget.ctl.lectureId,
                                    c.id,
                                  )
                                : widget.api.fileUrl(c.path),
                            prepare: flush,
                            initialMs: _localVideoTimes[c.id] ?? 0,
                            onPosition: (ms) => _localVideoTimes[c.id] = ms,
                          ),
                        ),
                      ),
                      'text' when c.readerCard => SingleChildScrollView(
                        key: PageStorageKey('study-reader/${c.id}'),
                        primary: false,
                        physics: _pdfPinching
                            ? const NeverScrollableScrollPhysics()
                            : null,
                        padding: const EdgeInsets.all(18),
                        child: CoursiveMarkdown(
                          c.text,
                          style: TextStyle(
                            fontSize: 16,
                            height: 1.55,
                            color: chrome.ink,
                          ),
                        ),
                      ),
                      'note' ||
                      'explanation' ||
                      'knowledge' ||
                      'practice' => KeyedSubtree(
                        key: ValueKey('${c.id}:$_cardTextEpoch'),
                        child: _KnowledgeBody(
                          card: c,
                          onSource: _focusSource,
                          editable: d.editable(c.layer),
                          onBeginAnswer: d.checkpoint,
                          onAnswer: (i, value) {
                            if (d.editable(c.layer)) {
                              c.responses['$i'] = value;
                              d.changed();
                            }
                          },
                        ),
                      ),
                      _ => SingleChildScrollView(
                        key: ValueKey('${c.id}:$_cardTextEpoch'),
                        padding: const EdgeInsets.all(20),
                        child: Align(
                          alignment: Alignment.topLeft,
                          child: SelectableText(
                            c.text,
                            onSelectionChanged: (selection, cause) {
                              if (!selection.isCollapsed) {
                                _selection
                                  ..clear()
                                  ..add(c.id);
                                _detail.clear();
                                _detail
                                  ..objectId = c.id
                                  ..start = selection.start
                                  ..end = selection.end;
                              }
                            },
                            style: TextStyle(
                              fontSize: 16,
                              height: 1.65,
                              color: chrome.paperInk,
                            ),
                          ),
                        ),
                      ),
                    },
                  ),
                ),
              ),
            ),
            SizedBox(
              height: 16,
              child: Align(
                alignment: Alignment.bottomRight,
                child: c.quarterTurns != 0
                    ? null
                    : Icon(
                        Icons.south_east,
                        size: 12,
                        color: selected ? chrome.accent : chrome.inkFaint,
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _LocalVideo extends StatefulWidget {
  const _LocalVideo({
    super.key,
    required this.url,
    required this.initialMs,
    required this.prepare,
    required this.onPosition,
  });
  final String url;
  final int initialMs;
  final Future<bool> Function() prepare;
  final ValueChanged<int> onPosition;
  @override
  State<_LocalVideo> createState() => _LocalVideoState();
}

class _LocalVideoState extends State<_LocalVideo> {
  late final player = Player();
  late final controller = VideoController(player);
  StreamSubscription? _errors;
  StreamSubscription? _positions;
  bool _failed = false;
  late final Future<bool> ready;
  @override
  void initState() {
    super.initState();
    _errors = player.stream.error.listen((error) {
      if (mounted) setState(() => _failed = true);
    });
    _positions = player.stream.position.listen(
      (p) => widget.onPosition(p.inMilliseconds),
    );
    ready = _open();
  }

  Future<bool> _open() async {
    try {
      // The media route resolves saved canvas objects. Publishing the reference
      // must finish before the player requests its authenticated URL.
      if (!await widget.prepare()) throw const FormatException('Save failed');
      if (!mounted) return false;
      await controller.platform.future;
      if (!mounted) return false;
      await player.open(Media(widget.url), play: false);
      if (mounted && widget.initialMs > 0)
        await player.seek(Duration(milliseconds: widget.initialMs));
      return mounted && !_failed;
    } catch (_) {
      if (mounted) setState(() => _failed = true);
      return false;
    }
  }

  Future<void> seekPaused(int milliseconds) async {
    if (!await ready || !mounted || _failed)
      throw const FormatException('Video unavailable');
    await player.seek(Duration(milliseconds: milliseconds));
    await player.pause();
  }

  @override
  void dispose() {
    _errors?.cancel();
    _positions?.cancel();
    unawaited(player.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => !_failed
      ? MaterialDesktopVideoControlsTheme(
          normal: const MaterialDesktopVideoControlsThemeData(
            modifyVolumeOnScroll: false,
            playAndPauseOnTap: false,
            toggleFullscreenOnDoublePress: false,
            primaryButtonBar: [],
            topButtonBar: [],
            buttonBarHeight: 48,
            seekBarContainerHeight: 36,
            bottomButtonBarMargin: EdgeInsets.zero,
          ),
          fullscreen: const MaterialDesktopVideoControlsThemeData(),
          child: MaterialVideoControlsTheme(
            normal: const MaterialVideoControlsThemeData(
              volumeGesture: false,
              brightnessGesture: false,
              seekGesture: false,
              seekOnDoubleTap: false,
              speedUpOnLongPress: false,
              primaryButtonBar: [],
              topButtonBar: [],
              bottomButtonBar: [
                MaterialPlayOrPauseButton(),
                MaterialPositionIndicator(),
                Spacer(),
                MaterialFullscreenButton(),
              ],
              buttonBarHeight: 48,
              seekBarContainerHeight: 36,
              bottomButtonBarMargin: EdgeInsets.zero,
            ),
            fullscreen: const MaterialVideoControlsThemeData(),
            child: VideoCardHoldRegion(
              bottomControlsHeight: 84,
              child: Video(controller: controller),
            ),
          ),
        )
      : Center(child: Text(l10n.studyLocalVideoError));
}

/// The paper colour as shown: in dark mode each paper keeps a trace of its
/// hue over the scheme's low container surface.
Color studyPaperTone(int color, ColorScheme cs) =>
    cs.brightness == Brightness.dark
    ? Color.alphaBlend(
        Color(color).withValues(alpha: color == 0xffffffff ? 0 : .06),
        cs.surfaceContainerLow,
      )
    : Color(color);

class _Paper extends CustomPainter {
  _Paper({
    this.document,
    Listenable? repaint,
    this.style = 'ruled',
    this.color = 0xffffffff,
    this.scheme,
  }) : super(repaint: repaint);
  final String style;
  final int color;
  final StudyDocument? document;

  /// Tones the paper for dark mode; null paints [color] as stored.
  final ColorScheme? scheme;
  @override
  void paint(Canvas c, Size s) {
    final tone = scheme == null ? Color(color) : studyPaperTone(color, scheme!);
    c.drawRect(Offset.zero & s, Paint()..color = tone);
    if (style == 'blank') return;
    var spacing = document == null ? 14.0 : 36 * document!.zoom;
    while (document != null && spacing < 22) {
      spacing *= 2;
    }
    final camera = document?.camera ?? Offset.zero;
    final line = Paint()
      ..color = tone.computeLuminance() > .5
          ? const Color(0x1826354a)
          : const Color(0x1fffffff)
      ..strokeWidth = .7;
    for (var y = camera.dy % spacing; y < s.height; y += spacing) {
      if (style == 'ruled' || style == 'grid') {
        c.drawLine(Offset(0, y), Offset(s.width, y), line);
      } else {
        for (var x = camera.dx % spacing; x < s.width; x += spacing) {
          c.drawCircle(Offset(x, y), .8, line);
        }
      }
    }
    if (style == 'grid') {
      for (var x = camera.dx % spacing; x < s.width; x += spacing) {
        c.drawLine(Offset(x, 0), Offset(x, s.height), line);
      }
    }
  }

  @override
  bool shouldRepaint(_Paper old) => true;
}

class _InkScene extends CustomPainter {
  _InkScene({
    this.inlinePDF = false,
    required this.document,
    required this.live,
    required this.stage,
    required Listenable repaint,
    this.dark = false,
  }) : super(repaint: repaint);
  final StudyDocument document;
  final bool inlinePDF;
  final List<StudyStroke> live;
  final Rect stage;
  final bool dark;
  @override
  void paint(Canvas c, Size s) {
    final d = document;
    c.save();
    c.translate(-stage.left, -stage.top);
    final viewport = stage;
    final printed = dark
        ? {
            for (final card in d.cards)
              if (studyOnPrintedPage(card.type)) card.id,
          }
        : const <String>{};
    final inCard = inlinePDF
        ? {
            for (final card in d.cards)
              if ((card.type == 'pdfdoc' && !card.collapsed) ||
                  (card.type == 'pdf' &&
                      card.pdfColumn.isNotEmpty &&
                      d.cards.any(
                        (r) => r.id == card.pdfColumn && r.type == 'pdfdoc',
                      )))
                card.id,
          }
        : const <String>{};
    for (final stroke in [...d.strokes, ...live]) {
      if (inCard.contains(stroke.anchor)) continue;
      if (d.strokeVisible(stroke) && stroke.bounds.overlaps(viewport))
        stroke.paint(c, dark: dark && !printed.contains(stroke.anchor));
    }
    c.restore();
  }

  @override
  bool shouldRepaint(_InkScene old) =>
      old.stage != stage || old.document != document || old.dark != dark;
  @override
  bool shouldRebuildSemantics(_InkScene old) => false;
}

class _InkTail extends CustomPainter {
  _InkTail({
    required this.document,
    required this.points,
    required this.start,
    required this.color,
    required this.width,
    required this.tool,
    required this.brush,
    required this.darkPaper,
    required Listenable repaint,
  }) : super(repaint: repaint);
  final StudyDocument document;
  final List<InkPoint> Function() points;
  final int Function() start, color;
  final double Function() width;
  final StudyTool Function() tool;
  final StudyBrush Function() brush;
  final bool Function() darkPaper;
  @override
  void paint(Canvas c, Size s) {
    final p = points();
    if (p.isEmpty) return;
    c.save();
    c.translate(document.camera.dx, document.camera.dy);
    c.scale(document.zoom);
    paintInk(
      c,
      p.sublist(start()),
      color(),
      width(),
      tool() == StudyTool.highlighter ? 'highlighter' : 'pen',
      brush: brush(),
      darkPaper: darkPaper(),
    );
    c.restore();
  }

  @override
  bool shouldRepaint(_InkTail old) => true;
  @override
  bool shouldRebuildSemantics(_InkTail old) => false;
}
