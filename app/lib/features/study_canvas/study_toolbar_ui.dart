part of 'study_canvas.dart';

const _paperColors = {
  0xffffffff: '白色',
  0xfffff6dd: '米色',
  0xffeef5e8: '浅绿',
  0xffeef0f4: '浅灰',
};

// Layout and interaction reference: Notein 1.2.607.0 on the user's tablet.
// Independently drawn controls; provenance is retained in drawing-controls.md.
extension _StudyToolbars on StudyCanvasState {
  Widget _action(
    String label,
    IconData icon,
    VoidCallback? action, {
    Key? key,
    bool selected = false,
    bool compact = false,
  }) {
    final cs = Theme.of(context).colorScheme;
    // A chosen tool sits in a tonal pill (M3 selected icon button).
    return IconButton(
      key: key,
      tooltip: label,
      onPressed: action,
      isSelected: selected,
      style: IconButton.styleFrom(
        minimumSize: compact ? const Size(32, 32) : const Size(40, 40),
        padding: EdgeInsets.all(compact ? 5 : 8),
        tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        backgroundColor: selected ? cs.secondaryContainer : Colors.transparent,
        foregroundColor: selected ? cs.onSecondaryContainer : cs.onSurface,
        disabledForegroundColor: cs.onSurface.withValues(alpha: .38),
      ),
      icon: Icon(icon, size: compact ? 20 : 22),
    );
  }

  Widget _divider(bool vertical, {bool compact = false}) => SizedBox(
    width: vertical ? 28 : (compact ? 8 : 16),
    height: vertical ? (compact ? 8 : 16) : 28,
    child: Center(
      child: Container(
        width: vertical ? 28 : 1,
        height: vertical ? 1 : 28,
        color: Theme.of(context).colorScheme.onSurface.withValues(alpha: .14),
      ),
    ),
  );

  Widget _topAction(
    String label,
    IconData icon,
    VoidCallback? action, {
    Key? key,
    bool selected = false,
  }) =>
      _action(label, icon, action, key: key, selected: selected, compact: true);

  /// Height of the top controls; macOS leaves room for the window buttons.
  static const double _topBarHeight = 36;
  double get _topBarsInset => Platform.isMacOS ? 28 : Space.sm;
  bool get _stackTopBars => _viewport.width < 900;
  bool get _showZoomControls => _viewport.width >= 600;
  double get _topBarsBottom =>
      _topBarsInset +
      (_stackTopBars ? 2 * _topBarHeight + Space.sm : _topBarHeight);

  /// Related buttons share one glass capsule, as iOS 26 toolbars group
  /// them; a lone action keeps its own circle. Fewer panes also means fewer
  /// backdrop blurs over a playing video.
  Widget _topButtons(String id, List<List<Widget>> groups, {Key? key}) =>
      SizedBox(
        key: key,
        height: _topBarHeight,
        child: Row(
          key: ValueKey('study-top-$id'),
          mainAxisSize: MainAxisSize.min,
          children: [
            for (var g = 0; g < groups.length; g++) ...[
              if (g > 0)
                SizedBox(width: _viewport.width < 360 ? Space.xs : Space.sm),
              GlassSurface(
                radius: Corners.full,
                padding: EdgeInsets.symmetric(
                  horizontal: groups[g].length > 1 ? Space.xxs : 0,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    for (final a in groups[g])
                      SizedBox.square(
                        dimension: _topBarHeight,
                        child: Center(child: a),
                      ),
                  ],
                ),
              ),
            ],
          ],
        ),
      );

  Widget _fixedTopBars() {
    // 返回 stands alone; the rest of each side is one group.
    final nav = _navigationActions();
    final navigation = _topButtons('navigation', [
      nav.take(1).toList(),
      nav.skip(1).toList(),
    ], key: _moduleKey);
    final actions = _topButtons('actions', [
      [_canvasMenu()],
    ]);
    final tools = GlassSurface(
      key: _toolbarKey,
      radius: Corners.full,
      padding: const EdgeInsets.symmetric(horizontal: Space.xs),
      child: SizedBox(
        key: const ValueKey('study-top-tools'),
        height: _topBarHeight,
        child: SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: _toolbarActions(),
          ),
        ),
      ),
    );
    return Positioned(
      top: _topBarsInset,
      left: Space.sm,
      right: Space.sm,
      child: SizedBox(
        key: _topBarKey,
        child: _stackTopBars
            ? Column(
                children: [
                  Row(children: [navigation, const Spacer(), actions]),
                  const SizedBox(height: Space.sm),
                  Align(alignment: Alignment.topCenter, child: tools),
                ],
              )
            : Row(
                children: [
                  Expanded(
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: navigation,
                    ),
                  ),
                  Expanded(flex: 3, child: Center(child: tools)),
                  Expanded(
                    child: Align(
                      alignment: Alignment.centerRight,
                      child: actions,
                    ),
                  ),
                ],
              ),
      ),
    );
  }

  List<Widget> _navigationActions() => [
    _topAction('返回', Icons.arrow_back, widget.onBack),
    _topAction('页面与材料', Icons.view_sidebar_outlined, _pageIndex),
    if (!widget.standalone)
      _topAction(l10n.studyOpenAIConversation, Icons.forum_outlined, _ask, key: const ValueKey('study-ai-tab')),
    if (!widget.standalone)
      _topAction(
        l10n.studyShelf,
        Icons.inventory_2_outlined,
        () => _toggleShelf(),
        key: const ValueKey('study-open-shelf'),
        selected: _shelfTab != null,
      ),
  ];

  List<Widget> _toolbarActions() {
    final d = document!;
    return [
      _topAction(l10n.commonUndo, Icons.undo, d.canUndo ? d.undo : null),
      _topAction(l10n.studyRedo, Icons.redo, d.canRedo ? d.redo : null),
      _divider(false, compact: true),
      _topAction(
        l10n.studyPanTool,
        Icons.pan_tool_outlined,
        () => _chooseTool(StudyTool.pan),
        key: const ValueKey('study-tool-pan'),
        selected: _tool == StudyTool.pan,
      ),
      _topAction(
        l10n.studyReadTool,
        Icons.near_me_outlined,
        () => _chooseTool(StudyTool.select),
        key: const ValueKey('study-tool-select'),
        selected: _tool == StudyTool.select,
      ),
      _divider(false, compact: true),
      _topAction('插入文字笔记', Icons.text_fields, () => _text()),
      _topAction('插入图片、PDF 或文件', Icons.image_outlined, () => _import()),
      _divider(false, compact: true),
      _topAction(l10n.studyLayers, Icons.layers_outlined, _layers),
      if (_pluginsOffered)
        _topAction(
          l10n.studyPlugins,
          Icons.extension_outlined,
          _plugins,
          key: const ValueKey('study-plugins'),
        ),
      _topAction('纸张背景', Icons.wallpaper_outlined, _paperSettings),
      if (_remotePending) ...[
        TextButton(
          onPressed: () => _resolveRemote(true),
          child: const Text('保留本机'),
        ),
        TextButton(
          onPressed: () => _resolveRemote(false),
          child: const Text('采用另一端'),
        ),
      ],
    ];
  }

  Widget _canvasMenu() => PopupMenuButton<String>(
    tooltip: _saveError == null ? '画布菜单' : l10n.studySaveFailed,
    padding: EdgeInsets.zero,
    style: IconButton.styleFrom(
      minimumSize: const Size(32, 32),
      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
    ),
    icon: Icon(
      Icons.more_horiz,
      size: 20,
      color: _saveError == null ? null : Theme.of(context).colorScheme.error,
    ),
    onSelected: (v) {
      switch (v) {
        case 'lesson':
          unawaited(_openLesson());
        case 'course':
          _courseMaterials();
        case 'video':
          _videoModules();
        case 'chat':
          _openConversation(newConversation: true, bind: false);
        case 'save':
          flush();
        case 'fit':
          _fit();
        case 'brush':
          _advancedPenSettings();
        case 'links':
          _connectionSettings();
        case 'export':
          exportRecovery();
        case 'new-canvas':
          widget.onNewCanvas?.call();
      }
    },
    itemBuilder: (_) => [
      PopupMenuItem(
        enabled: false,
        child: Text(
          widget.title ?? widget._player?.lecture?.shortTitle ?? '学习画布',
        ),
      ),
      if (_saveError != null)
        PopupMenuItem(value: 'save', child: Text(l10n.studySaveFailed))
      else if (_status != l10n.studySaved)
        PopupMenuItem(enabled: false, child: Text(_status)),
      if (!widget.standalone) ...[
        if (context.read<PluginStore?>()?.supportsLessonStudy == true)
          PopupMenuItem(value: 'lesson', child: Text(l10n.lessonStudyStart)),
        const PopupMenuItem(value: 'course', child: Text('课程资料')),
        const PopupMenuItem(value: 'video', child: Text('视频')),
        const PopupMenuItem(value: 'chat', child: Text('AI 对话')),
        const PopupMenuDivider(),
      ],
      if (!_showZoomControls)
        PopupMenuItem(value: 'fit', child: Text(l10n.studyFit)),
      const PopupMenuItem(value: 'brush', child: Text('画笔与橡皮设置')),
      const PopupMenuItem(value: 'links', child: Text('管理知识关联')),
      const PopupMenuItem(value: 'export', child: Text('导出画布副本')),
      if (widget.onNewCanvas != null)
        PopupMenuItem(value: 'new-canvas', child: Text(l10n.studyCreateBlankCanvas)),
    ],
  );

  Future<void> _paperSettings() async {
    await showDialog<void>(
      context: context,
      builder: (c) => StatefulBuilder(
        builder: (c, refresh) {
          void change(VoidCallback action) {
            _update(action);
            refresh(() {});
            unawaited(_saveLayout());
          }

          return AlertDialog(
            title: const Text('纸张背景'),
            content: SizedBox(
              width: 280,
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Wrap(
                      spacing: 12,
                      runSpacing: 12,
                      children: [
                        for (final entry in const {
                          'blank': '空白',
                          'ruled': '横线',
                          'dots': '点阵',
                          'grid': '方格',
                        }.entries)
                          Semantics(
                            selected: _paperStyle == entry.key,
                            child: InkWell(
                              onTap: () =>
                                  change(() => _paperStyle = entry.key),
                              borderRadius: BorderRadius.circular(10),
                              child: Container(
                                width: 126,
                                padding: const EdgeInsets.all(8),
                                decoration: BoxDecoration(
                                  borderRadius: BorderRadius.circular(10),
                                  border: Border.all(
                                    color: _paperStyle == entry.key
                                        ? StudyChrome.of(context).accent
                                        : StudyChrome.of(context).line,
                                    width: 1.5,
                                  ),
                                ),
                                child: Column(
                                  children: [
                                    CustomPaint(
                                      size: const Size(108, 48),
                                      painter: _Paper(
                                        style: entry.key,
                                        color: _paperColor,
                                      ),
                                    ),
                                    const SizedBox(height: 6),
                                    Text(entry.value),
                                  ],
                                ),
                              ),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(height: 20),
                    const Text('底色'),
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 8,
                      runSpacing: 8,
                      children: [
                        for (final entry in _paperColors.entries)
                          ChoiceChip(
                            label: Text(entry.value),
                            backgroundColor: Color(entry.key),
                            selectedColor: Color(entry.key),
                            selected: _paperColor == entry.key,
                            onSelected: (_) =>
                                change(() => _paperColor = entry.key),
                          ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(c),
                child: const Text('完成'),
              ),
            ],
          );
        },
      ),
    );
  }

  Future<void> _pageIndex() async {
    final selected = await showDialog<String>(
      context: context,
      builder: (c) => Dialog(
        child: SizedBox(
          width: 360,
          height: math.min(620, MediaQuery.sizeOf(c).height - 48),
          child: Column(
            children: [
              const ListTile(title: Text('页面与材料')),
              Expanded(
                child: ListView(
                  children: [
                    for (final card in document!.cards)
                      ListTile(
                        leading: Icon(
                          card.type == 'pdf' || card.type == 'pdfdoc'
                              ? Icons.picture_as_pdf_outlined
                              : card.type == 'lecture' || card.type == 'video'
                              ? Icons.play_circle_outline
                              : Icons.description_outlined,
                        ),
                        title: Text(
                          card.title.isEmpty ? card.type : card.title,
                          maxLines: 2,
                        ),
                        onTap: () => Navigator.pop(c, card.id),
                      ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
    if (!mounted || selected == null) return;
    final card = document!.cards.where((c) => c.id == selected).firstOrNull;
    if (card == null) return;
    document!.camera =
        _viewport.center(Offset.zero) - card.rect.center * document!.zoom;
    _cameraChanged();
  }
}
