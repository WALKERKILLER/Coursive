# Coursive 迁移到 MyGo 的可行性与实施方案

Artifact-Version: 1
Status: Approved

## Intent

**可以立项，建议按桌面、Android、iOS/iPadOS 分批发布。** 以 MyGo 原生 Go UI 为主，沿用 Coursive 的数据与服务契约。先在桌面和 Android 平板验证学习画布、媒体和原地升级，再决定替换各平台的 Flutter 发布包。iOS/iPadOS 作为新增端，单独验收本地核心和分发能力。

研究基线为 2026-11-18 的 MyGo v0.10.0 正式发布，以及 Coursive 本地源码 `bfc7ec8`。该版本的上游发布说明明确包含 iOS、iPadOS 和 Android，提供原生 UI 与系统 WebView、设备开发及签名打包。iOS 构建需要 macOS/Xcode 和真机签名配置，Android 需要 SDK、NDK 和 JDK。MyGo v0.10.0 的 Go 模块声明 Go 1.27.3，现有 daemon 声明 1.27.0。原型锁定工具链及依赖版本。[MyGo v0.10.0](https://github.com/egoist/mygo/releases/tag/v0.10.0) · [移动端构建文档](https://github.com/egoist/mygo/blob/v0.10.0/docs/mobile/README.md) · [MyGo go.mod](https://github.com/egoist/mygo/blob/v0.10.0/go.mod) · [daemon go.mod](../../daemon/go.mod)

现有客户端约 155 个已跟踪 Dart 源文件和 68 个测试文件，以 `app/lib` 和 `app/test` 下的 Dart 文件计。基础页面可以逐屏重写。播放器、画布输入、插件视图和安装升级需要按行为迁移，工期按能力拆分。Go daemon 和 Node 助手已有明确所有权，优先复用其领域逻辑与 HTTP/SSE 契约。[系统架构](../architecture.md) · [客户端依赖](../../app/pubspec.yaml)

## Contract

### 平台与服务边界

| 平台 | 建议上线范围 | 放行条件 |
| --- | --- | --- |
| Linux、macOS、Windows | MyGo 客户端启动现有 Go daemon，沿用它对 Node 助手和源插件进程的管理 | 在三个系统验证签名或打包、媒体依赖、中文输入、文件选择、休眠重连和原地升级 |
| Android 现有 APK 渠道 | 首版沿用现有 proot 工作区。将 Flutter 平台通道承担的工作区启动、前台服务和 URI 文件桥迁到 MyGo Kotlin 宿主 | 保持应用 ID、签名和安装数据目录。真机确认 native library 的提取及执行、两个现有 ABI、工作区复用、进程重启和长任务恢复 |
| Android Google Play | 单独评估商店构建。优先评估随包核心与用户授权的服务能力 | 明确运行时和插件分发方式。逐项审查现有工作区下载、助手安装和可执行插件，再决定 AAB 是否发布 |
| iOS/iPadOS | 首版候选为本地资料导入、离线播放、字幕和画布读写、导出与同步。在线助手、源插件及长期后台任务按能力上线 | 将可移植的 Go 核心作为应用内服务运行。逐项验证 SQLite/FTS、文件访问、生命周期以及使用到的外部工具。App Store 能力表需单独评审 |

Android 的工作区以[现有实现与草案](draft-android-app.md)为基线。iOS 设计将 Go 核心启动和关闭从 CLI 入口中拆出，显式接收上下文、目录与能力配置。先验证现有服务能在进程内复用，可由同一服务实现提供受控的本地传输。端口、令牌生成、访问控制、取消和 SSE 重连沿用现有契约。桌面和 Android APK 首版保持进程边界。对 iOS 上依赖外部进程的 Node/Pi、源插件、FFmpeg 和本地 ASR，逐项选择随包可审查的实现、明确授权的配对服务或延后交付。配对若立项，需要单独评审身份、TLS、绑定撤销和数据权限，且离线学习继续在本机可用。商店对下载可执行代码与后台运行有自己的约束。[daemon CLI](../../daemon/cmd/coursive-daemon/main.go) · [Apple 审核指南 2.5.2、2.5.4](https://developer.apple.com/app-store/review/guidelines/#software-requirements) · [Google Play Device and Network Abuse](https://support.google.com/googleplay/android-developer/answer/16559646)

客户端按现有契约读取课程、字幕、画布、备份及会话。Go UI 消费现有 HTTP/JSON、SSE 和插件视图协议。凭据保管仍由 daemon 负责。沿用媒体中继给出的访问方式，验证 Range、跳转、鉴权、字幕与播放时钟。播放器保留独立媒体接口，先比较 MyGo `ui/video` 与移动端 `plugins/videoplayer` 的实际覆盖，按平台选实现。特别验证视频卡片上叠墨、裁剪、层级、坐标变换与输入命中。MyGo 移动插件的原生视图位于自绘内容下方，文档说明可以覆盖绘制和接收触摸，合成效果仍需真机确认。PDF 先复用 daemon 渲染能力，验证多页缓存和大文档。Markdown、公式和插件表单分别核对选择、流式刷新、嵌套布局与批准流程。[学习画布持久化契约](../study-canvas.md) · [插件 UI 协议](0032-ui-extension-protocol.md) · [MyGo 视频](https://mygo.run/docs/ui/video) · [移动插件与原生视图](https://github.com/egoist/mygo/blob/v0.10.0/docs/mobile/plugins.md)

### Material Design 3 设计系统

**Mayu 可以降低基础组件成本，采用前先做兼容原型。** 社区库 Mayu 使用 MIT 许可，提供 HCT 种子色、色调层级、状态层、密度和常用控件。可优先验证按钮、卡片、列表、筛选 Chip、表单、对话框和 Navigation Rail。调研提交 `9223a1158cf6ce6ae535f8537c3e8af666875b78` 的模块仍声明 MyGo v0.5.1，远低于移动发布版本。Wasm Gallery 证明组件可展示。移动端输入和可访问性需要单独验收。[Mayu 组件目录](https://github.com/harukizmoe/mayu/tree/9223a1158cf6ce6ae535f8537c3e8af666875b78) · [Mayu go.mod](https://github.com/harukizmoe/mayu/blob/9223a1158cf6ce6ae535f8537c3e8af666875b78/go.mod)

设计从现有 `theme.dart`、`tokens.dart`、`prefs.dart` 和中文 ARB 提取。以 Coursive 的语义色、字体回退、字号、间距、形状和动效为产品基准。先做浅色、深色、高对比度和减少动态效果的组件样板。Android 系统取色需要另验，取不到时使用用户种子色。手机底部导航、平板与桌面侧栏按窗口宽度切换，键盘弹出时处理停靠栏与输入框避让。玻璃停靠栏、笔盒、画布选区与播放器控件保留 Coursive 自己的实现和无模糊回退。原生返回、系统文件选择、文本输入和分享交给平台。只在主题统一或语义缺失处封装组件。若 Mayu 无法低成本升到 v0.10.0，用 MyGo 基础控件补齐首批页面所需的 MD3 样式。[现有主题](../../app/lib/theme/theme.dart) · [设计令牌](../../app/lib/theme/tokens.dart) · [现有本地化](../../app/lib/l10n/app_zh.arb) · [MyGo 移动界面](https://github.com/egoist/mygo/blob/v0.10.0/docs/mobile/interface.md)

### 手势适配

普通页面使用 MyGo 的滚动、滑动回弹、点按、长按、刷新、惯性与系统返回。移动文档明确支持缩放和旋转，也给出 `InputZoom` 与 `mobile.Zoomable`。画布应在一个输入归属明确的区域维护统一状态，屏蔽自定义识别与控件默认点击、滚动的重复提交。普通图片预览可以使用通用缩放组件，画布使用自身世界坐标、视口锚点和触点状态。[MyGo 移动界面](https://github.com/egoist/mygo/blob/v0.10.0/docs/mobile/interface.md) · [现有画布交互](../drawing-controls.md)

| 场景 | 迁移行为 | 验收重点 |
| --- | --- | --- |
| 页面返回 | iOS 沿用边缘交互返回，Android 沿用系统预测式返回。弹层和页面返回栈使用同一导航状态 | 回拉取消保留视口、键盘与编辑状态，完成仅弹出一层。边缘返回与笔盒拖拽、横滑页和缩放同时验证 |
| 手指画布 | 单指平移，卡片上原地长按 600 ms 转为整卡拖动。第二指加入取消长按或未提交的拖卡预览，并按两指中心平移和等比缩放。手指数变化重设基准 | 两指分别落在视频、PDF 或空白时仍归同一个画布。抬一指、取消、第三指加入均无坐标跳变或误提交 |
| 手写笔 | 保留压感笔、倒置与侧键橡皮、笔悬停和接触优先。抬笔后延续 1 s 掌触保护。有效压力才允许重压橡皮，末端停留仍用于一笔成形 | 笔打断手指缩放、设备取消、笔与掌并发、出入视图均不得留下幽灵笔迹。无压设备使用固定笔宽和显式橡皮 |
| 独立播放器 | 单击控制层，双击两侧快退/快进、中区播放暂停，长按临时倍速，横向进度、纵向音量。拖动先锁轴，抬手确认进度，取消预览时恢复临时状态 | 先命中按钮与进度条。系统打断、退后台或切页时恢复原倍速。嵌入画布的视频画面将触摸交还画布 |
| 桌面与平板外设 | 保留鼠标拖动、滚轮与组合键缩放、触控板缩放、右键菜单和撤销快捷键。键盘可达所有工具动作 | 分别测 macOS 触控板、Windows 触屏与目标 Linux 会话。测试快捷键焦点、连续缩放、双输入设备切换 |
| 停靠栏与无障碍 | 保留滑过标签时的选择预览与松手确认、拖拽吸附和触觉反馈。为拖动、缩放、移卡提供按钮或键盘入口 | TalkBack、VoiceOver、放大文字、高对比度和减少动态效果下可操作。窄屏、分屏、输入法和安全区不会遮挡控件 |

公开的 `Element.Touches()` 含触点 ID、局部坐标、阶段和压力等字段，按构建帧读取。缺省压力会用 0.5，公开结构缺少笔种、压力有效性、时序采样和侧键字段。MyGo 移动宿主源码显示 Android 会采集工具类型和压力，iOS 会采集 `UITouch.type` 和 `force`。v0.10.0 的 `surface.touch` 把笔事件交给 `mouse` 分支。该分支构造指针事件时丢弃笔类型、压力和触点 ID。这条移动输入路径需要补桥，或使用包含等效修复的上游版本。原型须逐事件校验笔和手指两条路径。[MyGo Touches](https://github.com/egoist/mygo/blob/v0.10.0/ui/touches.go) · [Go 宿主事件分发](https://github.com/egoist/mygo/blob/v0.10.0/internal/mobile/surface.go) · [Android 宿主](https://github.com/egoist/mygo/blob/v0.10.0/internal/mobile/android/mygo/src/main/java/dev/mygo/SurfaceWindow.kt) · [iOS 宿主](https://github.com/egoist/mygo/blob/v0.10.0/internal/mobile/ios/Sources/MyGoHost/SurfaceWindow.swift)

普通手势用框架 API，专业输入先验证能否用最薄的 Kotlin/Swift 桥补齐。桥需把触点 ID、设备与工具类型、坐标、单调时间、有效压感、按钮、悬停、历史样本和取消原因传给画布。Android 取自 `MotionEvent`，按目标设备校验 palm cancellation 和侧键。iPadOS 用 UIKit 区分 Pencil 与手指，视设备能力补合并采样与 Pencil 交互。预测点只用于当帧预览。若原生模块无法接管主渲染视图的输入，优先向 MyGo 提交小补丁。只要笔迹保存或触控归属无法达标，该平台继续使用现有客户端，新增端保留预览资格。交互识别器以纯 Go 编写，用事件回放核对手势切换、取消和撤销边界。[Android 手写笔](https://developer.android.com/develop/ui/views/touch-and-input/stylus-input/advanced-stylus-features) · [Android 预测式返回](https://developer.android.com/guide/navigation/custom-back/predictive-back-gesture) · [Apple Pencil 交互](https://developer.apple.com/documentation/uikit/uipencilinteraction)

### 数据、升级和回退

迁移首版以旧数据可读写为发布条件。使用脱敏旧版样本验证画布版本、坐标、笔迹 ID、文档引用、导出、会话、下载与同步。保持当前格式，避免新旧客户端同时写同一份库。客户端偏好用可重复的一次性导入，保留旧来源直到升级验收完成。按现有发布约定保留包标识、签名和数据目录。Android 回退用同签名且更高 versionCode 的 Flutter 修复包演练。各端采用现有回退保留规则，保护升级后新写入的课程笔记和会话。[仓库升级规则](../../AGENTS.md#local-updates-and-backup-retention) · [发布策略](../release-policy.md)

## Plan

按 [迁移实施清单](../mygo-migration-todo.md) 执行。清单按阶段列出 Flutter 与 daemon 源码入口、可复用的测试和完成证据。

建议走 MyGo 原生 UI 分批迁移。Flutter 继续承担现有稳定版发布，直到目标平台完成验收。桌面 WebView 可用于需要网页的现有入口。全 WebView 方案也能搭建壳，但画布、媒体、输入和跨系统手写仍需重做，且 Mayu 的 Go 原生组件无法直接用于 DOM。保留 Flutter 并继续加固 Android 是原型遇到硬阻塞时的回退路线。

| 阶段 | 交付与退出条件 | 工程师周 |
| --- | --- | --- |
| 0 技术原型 | Linux、Android 真机和 iPadOS 上跑最小课程页。穿过真实 daemon 接口，验证 Mayu、画布触笔、视频叠墨和升级安装样例。记录版本和设备证据，做继续或暂停决定 | 2～3 |
| 1 壳与常用业务 | 桌面与 Android 上实现课程库、导航、主题、搜索、设置、文件导入、通知、下载进度和服务恢复 | 3～5 |
| 2 核心学习 | 播放器、时间轴、字幕、PDF、画布、旧文档、撤销重做和导出。过手势真机验收 | 5～8 |
| 3 功能齐套与桌面/Android 发布 | 助手、同步、插件和能力权限，三个桌面系统与 Android 原地升级、回退、灰度发布 | 4～7 |
| 4 iOS/iPadOS 首版 | 进程内核心、平台桥、离线学习、TestFlight build、真机验收和分发评审 | 4～8 |

阶段 0～3 合计约 14～23 工程师周，加入 iOS/iPadOS 本地学习首版共 18～31 工程师周。以上为初步人力估算，假设熟悉现有系统、能接触目标设备与签名配置，且复用 daemon 契约、存储格式和测试样本。iOS 完整助手/插件同等覆盖、商店审核等待和大规模维护框架 fork 单独估算。阶段 0 用实测结果重估阶段 2 和 4。若需要长期维护移动宿主 fork、变更分发政策或牺牲本地优先，再提交独立决策与规格评审。

## Verification

- 视效按设计系统对等，不按像素位对等验收。色板、间距、形状、字号和断点必须与 Flutter 端同值，由两边源码之间的机器校验保证；同一页面在两端的截图差异另做测量并记录，阈值由逐次评审确定，因为两套渲染引擎的字形光栅化不会逐位相同。
- 阶段 0 不依赖设备的检查先跑完：源码链接解析、Mayu 对 MyGo v0.10.0 的整包编译、daemon 与既有客户端的测试矩阵。触笔与叠墨需要目标 Android 平板，iPad 本地核心需要 macOS 与签名环境；缺任一条件时该项记为受阻，不计为通过。
- 阶段 0 的构建证据包含锁定版本下的三端产物、安装、启动和课程页可读。Android 额外记录现有 APK 覆盖升级，iPad 额外记录签名真机、沙盒读写与退后台重开。相同样本保存后，核对课程数、笔迹点与关键文件哈希。
- 测试分三层。Go 事件回放验证缩放锚点、长按取消、压感橡皮与撤销。MyGo Tester 和 `ui/golden` 验证触摸、取消、安全区与主题。真机验证系统返回、触笔、中文输入法、视频合成、文件选择、无障碍与后台行为。[MyGo 测试能力](https://github.com/egoist/mygo/releases/tag/v0.10.0)
- 设备至少覆盖 Android 手机、现有目标 Android 手写平板、iPad 与支持压感的 Pencil、macOS 触控板、Windows 触屏和 Linux 桌面。另用 iPhone 验证窄屏。只对设备实际支持的侧键、悬停和压感作通过声明。
- 同机同数据录制 Flutter 基线。脚本覆盖播放中连续缩放、跨视频与 PDF 卡片双指移动、连写、撤销与抬笔保存。候选门槛为 60 Hz 场景中 p95 帧时间不高于 Flutter 基线的 1.1 倍，并以 16.7 ms 为目标。记录触笔到可见墨迹延迟、丢帧和峰值内存。功能硬门槛是取消不产生幽灵笔迹、视频触控不抢画布、应用被杀后可恢复已保存笔迹。
- 发布前在旧版安装样本上演练升级和回退。验证空库启动、离线学习、长时媒体、同步重试、断网恢复、插件权限与批准。记录两种客户端能否互读新写入数据。按平台分别决定放行。

## Evidence

本规格使用 Coursive 源码、官方版本文档、上游输入实现与官方平台资料。Mayu 与 MyGo v0.10.0 的编译兼容已由阶段 0 前的源码核查和编译探针确认；两端实际触笔事件、视频叠墨性能、iOS 本地核心及商店验收等待阶段 0 验证。

### 源码核查

[迁移实施清单](../mygo-migration-todo.md) 的 101 个源码链接在 `bfc7ec8` 上全部解析到存在的文件和行，且逐行落在所指符号上，例如 `DaemonService._launch`、`Api._send`、`StudyCanvasState._videoInput`、`StylusBridge.touch`、daemon `run`、`_holdMaterial` 与 `_overControls`。

MyGo v0.10.0 的输入断言核对成立，但丢失点比预期更窄。`internal/mobile/wire.TouchEvent` 已经携带 `Pointer`、`Kind`、`Pressure`、`Buttons` 和 `Time`；`platform.SurfaceEvent` 已经有 `Pressure` 字段和 `PointerPen` 取值；Android 宿主的 `SurfaceWindow.onTouchEvent` 填了 `getToolType`、`getPressure`、`getPointerId`、`buttonState` 与 `eventTime`，iOS 宿主对 `.pencil` 填 `kind: "pen"` 和 `force`。缺口只在 `internal/mobile/surface.go`：`touch` 把 `Kind != "touch"` 的事件整体交给 `mouse`，该分支只发 `PointerDown`、`PointerMove`、`PointerUp` 与 `PointerLeave`，因此丢掉笔种、压力和触点 ID；`wire.TouchEvent` 本身也没有笔尖与橡皮之分、悬停状态、历史样本、预测点和取消原因。

两个后果是行为层面的，不只是字段缺失。`ui.Touch.Pressure` 没有 `pressureMin`/`pressureMax`，缺省填 0.5，而 Flutter 客户端判压感有效用的是 `e.pressureMax <= e.pressureMin`（`app/lib/features/study_canvas/study_interaction.dart`）；在 MyGo 里这个条件恒为假，压感橡皮会在没有压感的设备上误触发。笔种丢失则让 `study_stylus.dart` 的 `PointerDeviceKind.stylus` 分支整段失效：笔按手指处理，掌触保护和倒置橡皮都不生效。

### 编译探针

Mayu 在 `9223a115` 的 `go.mod` 声明 `mygo v0.5.1` 与 `go 1.27.2`，把该模块指向 MyGo v0.10.0 后 `go build github.com/harukizmoe/mayu/...` 整包通过，无需改动源码。该提交已提供 P02 需要的按钮、卡片、列表行、筛选 Chip、文本输入、对话框、Navigation Rail 与 SegmentedButton。

### 原型环境

探针于 2026-11-18 在 CachyOS 桌面执行：Go 1.27.0 与 daemon 一致，MyGo 触发的工具链自动切换到 go1.27.3 及以上；Flutter 3.44.9；Android SDK platforms 34 至 37、build-tools 36.0.0、NDK 28.2.13676358、JDK 27；无 Xcode，无连接的 Android 或 iOS 设备，未安装 `mygo` CLI。

### 设计系统对等

设计系统只有一个来源：Flutter 端的源码。`app-mygo/tokens.go` 是 `app/lib/theme/tokens.dart` 的镜像，`tokens_parity_test.go` 解析 Dart 文件逐项比对间距、圆角、页面内边距、内容宽度上限、断点和默认种子色，任一项漂移即失败。

色板不走重新生成。`app/test/design_system_export_test.dart` 用 Flutter 自己的 `ColorScheme.fromSeed` 写出 `app-mygo/design/palette.json`，Go 端直接读这份文件构造 Mayu 的 `ColorScheme` 和 MyGo 的 `ui.Theme`；该测试在常规模式下反过来校验提交的文件与当前主题一致。这样做的原因是实测差距：用同一个种子，Mayu 的 HCT 生成器与 Flutter 的取值在浅色 `primary` 上相差最多 22/255，深色相差 2/255，其余角色 0～3。这个量级会被人眼看出，所以颜色必须是同一组数字，而不是同一套算法。

组件度量按 `buildTheme` 逐个对齐：卡片 `surfaceContainerLow`、圆角 12、无阴影（Mayu 的 filled 卡片是 `surfaceContainerHighest`，因此卡片按令牌直接构建，不取 Mayu 的配色）；按钮 StadiumBorder 与 24/10 内边距；Chip 圆角 8 与 `outlineVariant` 描边；输入框圆角 8 与 16/16 内边距；导航栏底色 `surface`、指示器 `secondaryContainer`、标签 `labelMedium`。字号取 M3 阶梯叠加 Coursive 的覆盖（正文行高 1.6 与 1.55，标题类字重 500）。

两端引擎不同，字形光栅化不会逐位相同：MyGo 用自己的字体栈和排版实现，Flutter 走 Skia 与系统字体。因此验收口径是设计系统对等加截图差异测量，而不是像素位相等。

### 阶段 0 在桌面上的结果

`app-mygo/` 已在 Linux 上跑通课程库这一条路径：启动真实 daemon、读 `COURSIVE_READY` 握手、用自己签发的 token 调 `GET /api/courses`、建本地课程再读回，错误 token 被拒。握手中位耗时 11 ms。样本库是临时目录，未接触真实用户库。

Mayu 在真实窗口里可用：导航栏、卡片、筛选 Chip、文本框、主题切换都按 Coursive 的种子色 `#3F6CB4` 出图，浅深两套由 golden 图固定。

`app-mygo/gesture.go` 把画布输入规则写成不依赖 UI 类型的纯 Go，用事件回放验证：单指平移、600 ms 长按转整卡拖动、移动先于长按即拖动、第二指接管取消长按、缩放锚点、抬一指不跳变、第三指不跳变、笔打断手指、抬笔后 1 s 掌触保护、取消不留幽灵笔迹、取消回滚卡片位置、无压感设备不触发压感橡皮、笔种丢失时回落为手指。

### iOS 进程内核心的能力面

`run` 的依赖分三类。纯 Go 可原样进沙盒的有：`store.Open` 的 SQLite 与 `segments_fts`/`notes_fts` 两个 FTS5 虚表（`modernc.org/sqlite`，无 cgo）、`library.Open`、`events` 总线、`vault` 密封、`jobs`。需要换实现的有：`signal.NotifyContext(os.Interrupt, SIGTERM)` 与 `-exit-on-stdin-eof` 在应用里没有对应信号，改成显式 context 与显式关闭；`net.Listen("tcp", ...)` 的本地端口在退到后台后不保证存活，受控本地传输或前台续期要单独设计。必须延后的有：`agent.Supervisor` 与 `sources.Registry`/`provider.Exe` 的 Node 与插件子进程、`localasr` 的引擎二进制、`asr` 与 `study_media`、`local_courses`、`study_pdf` 对 `ffmpeg`/`ffprobe` 与 PDF 渲染器的调用——这些在 iOS 上都是外部可执行文件，商店也不允许随包下载。
