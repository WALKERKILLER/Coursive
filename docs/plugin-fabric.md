# Coursive Plugin Fabric 与 UI Extension Protocol

> **已通过的设计参考（Accepted design reference）**
> 版本：v1.0 · 通过日期：2026-11-09 · 适用仓库：[WALKERKILLER/Coursive](https://github.com/WALKERKILLER/Coursive)
> 规范契约：[spec 0031](specs/0031-plugin-fabric-core.md) · [spec 0032](specs/0032-ui-extension-protocol.md)
> 架构决策：[decision 0033](decisions/0033-plugin-fabric-and-ui-extension-protocol.md)
> 目标读者：产品负责人、Flutter/Go/Node.js 开发者、插件开发者、Agent 工程师、代码审查者。

本文件保留已通过的设计方向、取舍与跨端架构，供后续开发参考。可观察行为以两份 Approved Spec 为准；若本文件与 spec 冲突，以 spec 为准。实施顺序与子任务见 [Epic #38](https://github.com/WALKERKILLER/Coursive/issues/38)。

## 0. 执行摘要与结论

**核心判断：可以实现 DSH 风格的“插件新增页面 + 增强现有 UI + 自定义渲染 + Agent 调用插件”，但必须区分插件组织框架、JavaScript 执行引擎和 Flutter UI 渲染器三种职责。**

采用 **Coursive Plugin Fabric（统一插件能力总线）** 与 **Coursive UI Extension Protocol（UI 扩展契约）**：以 Go Daemon 承担插件安装、授权、能力注册与跨进程协调；保留 Node.js + Pi Agent 宿主处理复杂 Agent 扩展；新增可选的 MoeJS 轻量 JavaScript 执行器；Flutter 负责原生声明式页面、UI Slots、画布对象渲染；高自由度 UI 采用独立且受限的 WebView 方案。

“**一切皆插件**”是产品表达，不是把全部核心功能拆成可卸载代码。应当做到：**一切可扩展的能力有明确接口，插件可声明、组合、替换、停用、回退；核心数据与安全边界始终由宿主持有。**

**路线：** 先统一现有插件契约及生命周期（P0），再做动态页面和原生 Slot（P1），之后引入通用声明式 UI（P2）、服务依赖与 Agent 能力发现（P3）、MoeJS 执行器（P4），最后对桌面/移动平台逐项验证自定义 Web UI（P5）。不要为“技术上更纯粹”而重写现有 Pi 扩展或 ASR/视频处理核心。

### 本文的范围与状态

- 本文件不宣称已在本地构建或跑通跨端原型。它是基于 2026-11-09 仓库公开源码与上游文档的只读设计分析，现已作为已通过的设计参考保留。
- 本文件不替代 Approved Spec，也不变更已有 spec 的结论；落地必须遵守仓库治理、权限和安全规则。
- 不把 MoeJS 的基准成绩当作 Coursive 的真实加速结果；只给出可测量的假设与验证门槛。

## 1. 产品愿景、目标与边界

### 1.1 愿景

Coursive 从“带插件市场的录播学习客户端”发展为“**由 Agent 驱动的、可组合的本地优先学习工作空间**”。一份课程视频、一个字幕片段、一张手写画布、一条知识关系、一张练习卡，都能被不同插件读懂、链接、可视化与进一步加工。

### 1.2 产品目标

| 编号 | 目标 | 业务上的可观察结果 |
|---|---|---|
| G1 | 插件可新增完整页面 | 安装复习插件后新增「记忆复习」页；禁用后入口消失并安全回退 |
| G2 | 插件可增强已有 UI | 公式旁多出解释/推导/绘图工具，课程页增加进度组件 |
| G3 | 插件可以组合服务 | 知识图谱不依赖特定 ASR、LLM 或课程平台 |
| G4 | 原生表现不被破坏 | 手机/桌面、多尺寸、深浅主题、无障碍与现有画布操作一致 |
| G5 | Agent 能发现能力 | Agent 知道可用、缺失、受限和待授权工具，选择已有插件完成目标 |
| G6 | 运行时可选 | 轻量 Hook 用 MoeJS，Pi 扩展仍在 Node，重计算走 Go/外部进程 |
| G7 | 安全、可撤销 | 插件禁用、升级失败、资源异常不污染核心数据或使 App 无法启动 |

### 1.3 非目标

- 任意第三方 Dart/Flutter 源码运行时注入发布版 App；Flutter 的 AOT 发布模式不适合这种方案。
- 插件直接访问 Coursive 主数据库、凭据库、所有文件或所有 HTTP 路由。
- 默认允许插件重写整个应用 Shell、替代应用授权规则或注入任意 native library。
- 让插件在未授权时自动安装、自动执行系统命令、替用户完成不可逆修改。
- 把 WebView 作为默认的所有 UI 渲染方式。

### 1.4 设计原则

**核心保真、能力开放、生命周期可逆、数据可移植、默认最小权限、宿主最终裁决、离线优先、跨平台明确降级。** 插件以稳定协议访问能力，不得跨越 Flutter/Go/Node 责任边界直接依赖内部文件结构。所有可变更用户数据的行为必须保留现有撤销、审批和审计语义。

## 2. 现有架构基线与差距（以 main 为准）

### 2.1 事实基线

现有 Coursive 由 `app/` Flutter 客户端、`daemon/` Go 守护进程、`agent/` Node.js + Pi 助手宿主组成；Flutter 通过带短期凭据的 loopback HTTP/SSE 访问 Daemon，Agent 也由 Daemon 授权访问数据。

| 位置 | 已观察到的能力 | 仍需补齐 |
|---|---|---|
| `app/lib/shell.dart` | 固定 `Tabs` 索引、导航数组、`_root(i)` 页面 switch、多尺寸布局 | 动态导航注册、页面注册、插件离线/禁用时退路 |
| `app/lib/core/plugins.dart` | PluginStore、插件贡献项、`entries(kind, surface)`、状态监听 | 统一页面/Slot 描述、增量刷新、UI 生命周期 |
| `app/lib/features/plugins/plugin_views.dart` | `pluginViewBuilders` 注册映射，当前可见 `checklist`、未知视图降级卡片、弹窗/菜单动作 | 通用声明式 UI 解析、状态绑定、局部插槽及复杂交互 |
| `agent/src/plugins.mjs` | Pi 扩展加载、`coursive.action/effect`、权限网关、会话工具 | 跨运行时能力目录、服务依赖、轻量执行器桥接 |
| `daemon/internal/sources` | 课程来源 Provider、子进程生命周期、JSON-RPC 及授权数据边界 | 面向通用插件的执行器抽象、服务选择与隔离策略 |
| `docs/specs/0004-plugin-platform.md` | Pi 插件、声明式视图、有限功能入口、`coursive.effect` | 可扩展页面、UI Slots、完整动态组件系统 |
| `docs/specs/0025-plugin-marketplace.md` | 已批准的 Daemon 统一安装/签名索引/镜像/权限方案 | 核对真正落地状态及 API 迁移完成度 |

**重要版本事实：** 已批准的插件市场规范规划 Plugin API 2.0、由 Daemon 统一持有安装状态；但本次读取的 `agent/src/plugins.mjs` 仍声明 `PLUGIN_API_VERSION = "1.0.0"` 并保留旧式 PluginHost 管理逻辑。该差异不一定代表运行失败，但必须先建立“设计—代码—迁移状态”台账，禁止直接在两个状态管理体系上叠加新机制。

### 2.2 必须保留的原有决定

仓库架构决策 0005 选择 **Pi 扩展 + 权限化 Coursive API + Flutter 声明式视图**，理由是减少重复运行时、延迟暴露 Tool Schema、维持原生 Material 3 与可测试性。新的 Plugin Fabric 应是**跨进程契约和补充执行器**，不是推翻 Pi。

来源 Provider 的登录和凭据、下载接管、课程素材在安装/卸载插件后保持可读、后台工作不阻塞 App 启动等语义不得回退。`docs/specs/0024` 与 `0025` 的安全边界保留。

## 3. DSH / Cordis 与 MoeJS：取其思想，不照搬实现

### 3.1 Cordis 的五项关键机制

1. **Service**：插件提供能力，其他插件通过稳定服务名称获取，而不是 import 具体实现。
2. **Inject**：插件声明依赖，依赖满足后再加载；缺失时显示可理解的降级和修复入口。
3. **Event / Hook**：模块之间用类型化事件、可串行或并行的中间件协作。
4. **Effect / Disposer**：安装/注册时同时获得撤销句柄，禁用与热重载能清理工具、事件、页面、定时器和资源。
5. **Context**：把服务获取限制到插件当前的权限和作用域，避免全局对象任意共享。

### 3.2 DSH UI Slots 能力

DSH Web Client 将 UI 设计成 Typed React Slot 组合树，注册可撤销、分层声明、支持 `single`、`list`、`keyed`、`chain`、`hoc` 等模式。插件能够注册左侧导航项、主面板以及聊天/输入区的局部组件。其 React Component 本身可以由 Web Client 的插件代码提供，**这不等于 Flutter 可以直接加载同样的组件**。

Coursive 应借鉴 Slot 的**注册机制、契约、优先级、作用域与生命周期**，而不是把 React API 逐字迁移到 Flutter。

### 3.3 MoeJS 的位置

MoeJS 是纯 Go JavaScript 引擎，适合嵌入大量短小脚本、Hook、数据变换和局部策略逻辑，支持编译复用、运行环境隔离、内存上限、中断与宿主 API 控制。**它不是 HTML/CSS 渲染引擎，也不能执行任意 npm/Node 项目以替代现有 Agent Host。**

应选择性将其放在 Go Daemon 内，与 Node Pi 形成并存的执行器。性能收益需以端到端 Coursive 工作负载实测为准。

## 4. 用户故事与交互流程

### US-01 安装后出现独立分页

用户在插件市场安装「记忆复习」→ 检查签名、版本和权限 → 同意安装 → 桌面端 NavigationRail 出现「记忆复习」，手机端「扩展」入口或可固定的快捷导航中出现该项 → 首次打开显示插件页面 → 断网也可查看本地复习数据 → 关闭/卸载插件后入口消失，当前页面回退到宿主安全页面。

**验收：** 不重启 App；不修改核心导航源码；状态可恢复；禁用后不存在悬空导航；插件错误时显示隔离的错误页而非白屏。

### US-02 插件增强现有课程和公式

用户在课程视频字幕中选中一条公式 → 插件注册的 `canvas.card.actions` 显示「符号解释」「分步推导」「曲线绘制」 → 单击启动无需模型的轻量计算或需要授权的 Agent Tool → 结果与原始视频时间戳建立引用 → 结果卡片可移动、撤销、重做、导出。

**验收：** 插件无权修改未选中的画布对象；取消执行不产生半成品；多插件共存可排序；关闭任何插件不删除用户原始资料。

### US-03 插件依赖与服务替换

知识图谱插件声明需要 `transcript.read` 与 `llm.generate`。本地 sherpa-onnx Provider 与云 ASR Provider 均可提供统一 `transcript.read`。用户切换优先 Provider 后，知识图谱插件无需修改代码，系统显示选择来源、失败原因及回退结果。

### US-04 插件自带完整自定义界面

用户安装“虚拟实验室”插件 → 打开插件全屏页 → 页面自带交互曲线和控制面板 → 在隔离 WebView 内渲染 React 等前端构建产物 → 通过能力桥请求课程片段/保存模拟数据 → 宿主对写操作预览和审批 → 卸载后页面 UI 退出，但用户已保存的标准学习对象仍可被基础客户端读取。

**验收：** WebView 无默认 `localhost` 通行证、无 Cookie/Token 外泄、无法绕过权限读取本机文件；不支持 WebView 的平台显示替代的原生说明页。

### US-05 Agent 发现插件并组合任务

用户说“将这节课所有公式整理成可交互推导卡片并关联视频”。Agent 查询能力目录 → 获得可用/缺失/需授权信息 → 加载相关 Tool Schema → 请求读取选定课程范围 → 调用转录、公式识别与画布创建 → 将计划和变更预览交给用户 → 宿主验证、原子提交并保留撤销点。

**验收：** Agent 不得自行突破插件授权、不能绕过审批模式，工具结果具备出处、错误和审计信息。

## 5. 目标架构与责任划分

```text
Flutter UI  [Shell / Nav Registry / UI Slots / Declarative Renderer / WebView]
       │  受控 UI 事件与状态同步（HTTP / SSE / typed payload）
       ▼
Go Daemon  [Plugin Store / Registry / Dependencies / Lifecycle / Permission Broker]
       ├── Go Built-in Services（库、任务、ASR 调度、撤销与数据校验）
       ├── MoeJS Executor（轻量 JS Hook / 变换 / 业务策略）
       ├── Process Executor（课程来源/原生工具；JSON-RPC）
       └── Node Agent Host [Pi SDK / LLM / Skills / Complex JS Extensions]
                          │
                          └── Agent Tool Registry / Capability Inspector
```

### 5.1 所有权矩阵

| 资源/职责 | 唯一所有者 | 其他层如何访问 |
|---|---|---|
| 安装记录、版本、启停、用户授予权限 | Daemon | 类型化 API；Flutter 显示，Node 只读可执行清单 |
| 课程原件、笔记、画布持久化、撤销数据 | Daemon/现有画布数据契约 | Scoped API；插件不直接写底层文件 |
| 页面布局、路由、主题、键盘/触控交互 | Flutter | UI Extension Protocol，禁止跨层注入 Widget 对象 |
| Agent 会话、LLM Tool 调度、记忆 | Node Pi | Daemon 授权 Tool Gateway |
| 轻量 JS 执行环境 | Go 内 MoeJS Adapter | `Execute(pluginId, action, context)`，无宿主机默认权限 |
| 来源登录、媒体解析 | Source Provider 与 Daemon Vault | 已有 Source Provider RPC，保持原契约 |

### 5.2 统一 Plugin Descriptor

每个插件有一个逻辑身份（ID/版本/发布者/兼容性/权限），可以贡献一个或多个执行组件。执行组件属于各自运行时，**不强制一个插件只能运行在单一引擎**。例如同一个插件可以包含声明式 Flutter 页面、一个 MoeJS 数据转换器和一个 Pi Tool 声明。

Plugin Registry 维护 `installed / enabled / resolving / ready / degraded / failed / disabled` 状态，区分“插件安装成功”和“某个贡献可使用”。无 Agent Runtime 时，纯 UI 页面或本地轻量功能依然可以使用。

## 6. 核心协议：Services、依赖、事件与生命周期

### 6.1 Service Registry

服务标识建议采用 `domain.verb` 方式，例如 `transcript.read@1`、`canvas.objects.create@1`、`llm.generate@1`。每项服务都必须发布 `id`、`version`、`inputSchema`、`outputSchema`、`owner`、`scope`、`requiredPermissions`、`costHint`、`availability`。

依赖解析顺序：版本匹配 → 权限与设备能力 → 当前用户偏好 → 可用状态 → 优先级 → 确定性冲突规则。禁止隐藏地从一个 Provider 切换到数据边界不同的云服务；例如本地 ASR 降级到云端须再次提示潜在上传行为。

### 6.2 Inject 与缺失依赖

- 必需依赖不满足：插件贡献显示“不可用”，并提供安装/启用/授权建议，不阻塞整个 App。
- 可选依赖不满足：能力降级（例如无 `llm.generate` 时只提供规则式思维导图）。
- 检查依赖循环，拒绝激活互相等待的环；使用声明式诊断显示循环路径。
- 多 Provider 可共存，默认选择由能力路由器承担；插件不应直接识别硬编码的机构/ASR 品牌。

### 6.3 生命周期与可撤销 Effect

推荐状态机：`discovered → installed → resolved → activating → ready → deactivating → disabled`，失败可进入 `degraded/failed`。安装事务与激活事务分离；持久化资源和临时 UI 注册分别处理。

每一次注册必须返回 `dispose()`：工具、菜单入口、导航项、Slot、事件订阅、计时器、任务、资源池均必须实现。禁用插件时先停止新的调用、等待或取消在途任务、撤销 UI，再释放 Runtime；失败时自动回退。

**持久化数据不应跟随临时 Effect 一起删除。** 插件私有缓存可清理，但用户选择保留的学习卡片和笔记应独立于插件安装状态。

### 6.4 Event Bus 设计

| 类型 | 语义 | 典型事件 |
|---|---|---|
| `emit` | 无返回值通知 | `lecture.playback.changed` |
| `parallel` | 独立观察者并发执行 | `course.index.finished` |
| `serial` | 有序执行 | `note.export.prepare` |
| `waterfall` | 按链修改候选结果 | `transcript.normalize` |
| `bail` | 第一个明确处理者生效 | `source.resolve` |

事件带 schema、`traceId`、超时和作用域。观察者不应通过事件获得超出自身授权的数据。任何 `waterfall`/替换类 Hook 必须保留原始结果的审计记录与冲突处理策略。

## 7. UI Extension Protocol 总体设计

### 7.1 三级 UI 扩展策略

| 级别 | 开发体验 | 渲染 | 推荐范围 |
|---|---|---|---|
| L1 声明式 UI | 插件提交 JSON/JSON Schema + 动作描述 | 宿主 Flutter 原生 Widget | 大部分页面、表单、统计卡片、复习组件 |
| L2 UI Slots | 插件向已公开位置添加/装饰/有条件替代组件 | Flutter 受控组件 | 课程页、播放器、画布、聊天消息、设置区域 |
| L3 自定义 Web UI | 插件提供已构建静态 HTML/CSS/JS 资源 | 受限 WebView | 绘图编辑器、复杂互动实验、特殊可视化 |

**默认使用 L1+L2；L3 必须按平台和权限显式启用。** 插件不得直接加载第三方 Dart 代码、反射应用内部 Widget、下载原生库后执行。

### 7.2 动态导航与完整页面

新增 `NavigationRegistry` 与 `PluginPageRegistry`。每项页面贡献包含：`id`、`pluginId`、`titleKey/title`、`icon`、`route`、`renderer`、`minHostVersion`、`availableOn`、`requiredServices`、`requiredPermissions`、`order`、`keepAlivePolicy`、`fallbackRoute`。

桌面 NavigationRail 允许“固定常用插件页”，较多插件折叠到「扩展」区域；移动端底部导航保持固定、将插件页放入统一扩展中心，允许用户固定少量快捷入口。保留原 `Tabs` 整数索引向后兼容，但新导航必须使用稳定字符串 Route ID；不把插件页插入固定索引造成已有调用错位。

禁用/卸载/依赖失效：当前插件页完成页面退出动画 → 返回最近一个有效宿主页 → 取消订阅及 action → 可选恢复书签；不能留下空白页或不能退出的 Navigator。

### 7.3 UI Slot 目录（第一批）

| Slot ID | 用途 | 策略 |
|---|---|---|
| `app.navigation.entries` | 导航额外入口 | `list`，可固定/隐藏 |
| `app.main.pages` | 插件完整页面 | `keyed`，按 routeKey |
| `course.detail.sections` | 课程内容分段 | `list`，可折叠 |
| `course.header.actions` | 课程顶部动作 | `list`，限制可见数量 |
| `player.controls.trailing` | 播放控制扩展 | `list`，不影响核心播放 |
| `player.transcript.after` | 字幕后置注释 | `list`，按片段作用域 |
| `canvas.toolbar.tools` | 新画布工具 | `list`，触控笔优先 |
| `canvas.card.actions` | 卡片动作 | `list`，传入选中对象上下文 |
| `canvas.card.renderer` | 特定卡片类型渲染 | `keyed/chain`，失败回退 |
| `canvas.overlay` | 画布非持久浮层 | `list`，限制层级 |
| `assistant.message.after` | 回复尾部增强 | `list`，不修改消息正文 |
| `assistant.tool.result` | 工具结果展示 | `keyed`，按 tool/result type |
| `assistant.composer.actions` | 输入框快捷入口 | `list`，移动端折叠 |
| `settings.sections` | 插件设置 | `list`，独立权限域 |

### 7.4 Slot 语义及冲突规则

- `list`：多插件追加，按 `order`、`priority`、`pluginId` 稳定排序。
- `single`：只能一个获胜；冲突按明确优先级或用户选择处理，保留宿主回退。
- `keyed`：某对象/类型选择一个渲染器；重复 key 不允许静默覆盖。
- `chain`：按顺序匹配首个可用策略，超时回退原始组件。
- `hoc/decorate`：仅允许宿主认可的装饰能力（徽章、批注、高亮等），不暴露任意 Widget 构造与内部 `BuildContext`。

**作用域（Scope）**：全局/当前课程/讲次/画布文档/会话/选区；只传递该 Slot 所需的最小上下文，如 `courseId`、`lectureId`、`objectIds`、`selectionRange`、`sourceRefs`。不向插件透传数据库连接、密钥、整个 appState。

### 7.5 UI 注册与原子更新

页面或 Slot 不是逐条散落修改，而应在同一插件激活事务中提交。只有所有声明通过 schema、版本、权限和资源检查后才对 Flutter 发布变更。插件升级或禁用统一回收，并通知 Flutter 重新构建相应子树；禁止在 UI build 阶段执行昂贵的插件脚本。

## 8. 声明式 Flutter UI DSL

### 8.1 核心理念

插件提交**数据化组件树**，而不是 Dart 源码。宿主维护版本化白名单：`column`、`row`、`card`、`text`、`button`、`input`、`tabs`、`list`、`table`、`chart`、`timeline`、`math`、`mediaRef`、`sourceRef`、`canvasGraph`。布局由宿主约束，样式只引用设计令牌（如 `surface`, `primary`, `spacing.md`），不接受任意 CSS 或任意表达式执行。

### 8.2 页面 Schema 示例（建议 UI DSL v1）

```json
{
  "schemaVersion": 1,
  "id": "review.dashboard",
  "title": "记忆复习",
  "state": {"selectedCourse": null},
  "body": {
    "type": "column",
    "children": [
      {"type": "text", "variant": "title", "text": "今日复习"},
      {"type": "stats", "source": "review.summary"},
      {"type": "list", "source": "review.dueCards", "itemTemplate": "flashcard.row"},
      {"type": "button", "text": "开始复习", "onPress": {
        "action": "review.start", "args": {"courseId": "${state.selectedCourse}"}
      }}
    ]
  }
}
```

上述 `${state.selectedCourse}` 是**拟议的安全数据绑定语法**，必须由宿主解析为结构化变量引用，不能用 JavaScript `eval` 求值；正式实现宜用对象路径引用取代字符串模板。

### 8.3 动作与状态模型

声明式 UI 的点击事件发送 `{pluginId, actionId, viewId, instanceId, args, contextRevision, requestId}` 给 Daemon。Daemon 复核插件状态、权限、目标对象版本与用户批准模式后执行；返回 `statePatch`、`navigation`、`viewPatch`、`notification`、`proposal` 中经过 schema 校验的结果。

本地 UI 临时状态归 Flutter；插件可复用的数据存 Go Daemon 的插件命名空间；课程/笔记正式数据仍由已有领域 API 保存。允许乐观 UI，但写数据最终以 Daemon 回应为准。异步请求应支持取消与幂等键，禁止重复点击导致双重写入。

### 8.4 资源与深度限制

建议针对不同组件类别定义限制：树深度、节点总数、文本长度、单次 payload 字节、渲染频率、虚拟列表项数、订阅数量和媒体引用尺寸。超限时拒绝并显示可诊断错误，不应静默截断造成误导。具体阈值需按低端 Android 实测确定，作为版本化协议的一部分。

## 9. L3 自定义 Web UI：强能力，强隔离

### 9.1 Web UI 的定位

支持插件提交打包后的静态页面资源，获得完整 HTML/CSS/JS 交互能力（包括 React/Vue 图表）。MoeJS 不负责绘制网页，**WebView/浏览器引擎负责 UI**，MoeJS/Go/Node 负责业务和工具。

Web UI 不能像 DSH React 一样直接以宿主原生组件运行；它是嵌入式独立渲染岛。跨端桥接必须以结构化消息通信，禁用任意脚本调用宿主原生对象。

### 9.2 资源加载与网络边界

- 安装时验证签名索引和哈希，插件静态资源进入只读包目录；文件路径不可越界。
- 页面分配插件专用 Origin（自定义受控 scheme 或隔离的本地只读 Origin），不能直接暴露 Daemon 的 token、存储端点和凭据。
- 使用严格 CSP、禁用不需要的窗口打开/导航/下载/剪贴板/设备权限；默认拒绝外部资源，显式获批的网络请求走 Daemon Broker。
- 限制 URL Scheme、外链跳转、嵌入 iframe、WebSocket、混合内容和深链接；禁止把 `127.0.0.1` 全部视为可信。
- 原生消息桥验证 `origin + pluginId + instanceId + nonce + schemaVersion + requestId`，每一条命令独立检查权限，不信任仅由 JS 提供的身份。
- iframe/WebView 隔离与进程隔离的可靠程度依赖平台实现；高风险插件仍需外部进程或更强 OS 沙箱。

### 9.3 WebView Bridge 示例

```json
{
  "type": "plugin.invoke",
  "apiVersion": 1,
  "requestId": "req-001",
  "pluginId": "virtual-lab",
  "instanceId": "page-039",
  "action": "canvas.objects.previewCreate",
  "args": {
    "courseId": "course-123",
    "objects": [{"type": "simulation", "title": "声线折射实验"}]
  }
}
```

服务端不可仅相信 `pluginId` 字段，必须根据受信任的页面会话与授予能力映射身份。对写操作返回待确认 Proposal；只有用户授权后才提交。永远不要给 WebView JS 一个可以代理任意 Daemon HTTP 请求的通配桥。

### 9.4 平台准入策略

Android/macOS 优先测试可用的官方或稳定 WebView；Windows/Linux 需就第三方 WebView 引擎、打包依赖、硬件加速、输入法、触控笔和窗口层级逐平台测评。**缺少稳定支持时，仍应能运行 L1 原生 UI，并向用户展示功能降级，而不是牺牲整个 App 的稳定性。**

## 10. 画布插件化：对象模型与交互约束

### 10.1 核心数据模型

画布不是“每个插件自己画一张独立画布”。所有插件生产的持久对象必须落在统一、版本化的 `CanvasObject` 契约：`id`、`type`、`schemaVersion`、`geometry`、`content`、`sourceAnchors`、`createdBy`、`revision`、`interactionPolicy`。引用可指向课程视频时间段、PDF 页和范围、课件文件、字幕片段及手写笔迹。

### 10.2 插件能扩展什么

- **工具 Tool**：框选讲解、公式符号解释、数据标注、声线模拟、概念关系提取。
- **渲染 Renderer**：某类型卡片的受控原生渲染器，或高自由度 WebView 弹出页。
- **操作 Action**：选择、复制、导出、加入复习、跟随原文、询问 Agent。
- **标注 Overlay**：临时高亮和引用热区；不能抢占手写笔迹命中测试，除非用户显式切换工具。
- **关系 Edge**：知识节点之间的规范化关系，具备删除悬挂边与同步一致性保证。

### 10.3 数据保存与撤销

所有插件发起的持久更改统一进入既有画布事务与撤销栈。新增对象可在规则允许时批量确认；覆盖原卡片、删除、编辑出处必须事前预览并根据宿主审批模式确认。失败、取消、文档 revision 不匹配时整批不提交，避免出现“生成了一半”的状态。

缺失插件的对象使用 **Fallback Renderer**：显示标题、来源、文本摘要、导出数据，保留对象和引用；不能因为卸载插件就删除这些对象。

## 11. Agent 与插件能力发现

### 11.1 能力目录（Capability Inspector）

提供 `plugins.listCapabilities`、`plugins.inspectService`、`plugins.inspectSlot` 等可读 API，输出当前的服务目录、实际提供者、插件状态、工具 Schema、成本标签、权限、可用范围和失败原因。

Agent 首先查询简短的 capability 摘要，再按需展开具体 Tool Schema，延续现有 `find_tools` 延迟暴露策略，避免大量插件占满上下文。

### 11.2 Agent 调用规则

- Agent 可**建议安装**插件，但不可在未获用户授权时静默安装或扩大权限。
- Agent 可根据当前上下文打开插件页或提交 UI 动作提案，但不能直接执行未信任的页面脚本。
- 任何写文件、覆盖画布、修改资料、上传课程内容、启动外部可执行文件、登录外部课程平台的行为仍由宿主独立授权。
- 工具结果必须包含 `traceId`、来源引用、处理时间与可读错误，避免模型将失败误判为成功。
- 不允许插件自行修改宿主审批策略、注入隐藏指令来获得高级权限。

### 11.3 典型调用链

```text
用户：把本节课的重要公式整理成可交互推导卡片
  → Agent 查询 Capability Inspector
  → 发现 transcript.search / formula.extract / derivation.build / canvas.previewWrite
  → 检查课程范围、模型/费用与用户授权
  → 调用插件提供的工具（MoeJS / Pi / Go 皆可能）
  → 统一返回 schema + sourceAnchors + preview
  → Flutter 展示预览 → 用户批准
  → Daemon 原子保存 + 撤销点 + trace log
  → Agent 汇报可检验的结果
```

## 12. MoeJS 运行时集成规范

### 12.1 首选适用任务

适用：字幕文本归一化、轻量公式/标签处理、事件 Hook、请求预处理、结构化数据转换、无需 Node/npm 大型依赖的小插件动作。不适用：重型视频解码、ASR 推理、GPU 任务、复杂 npm 生态、需要高可靠 OS 隔离的未知原生代码。

### 12.2 建议执行器接口

```go
// 伪接口：后续根据现有 Daemon 结构确定 package 与错误类型。
type PluginExecutor interface {
    Prepare(ctx context.Context, manifest Manifest) error
    Invoke(ctx context.Context, req Invocation) (Result, error)
    Dispose(ctx context.Context, pluginID string) error
    Capabilities() ExecutorCapabilities
}
```

MoeJS Adapter 采用“一次 Compile，多 Runtime 复用”的策略；池上限受设备内存和并发限制；每个 Runtime 限制内存、截止时间和可导入模块；Host API 白名单按插件实际授权创建。Runtime 内不得持有 Daemon Token、主 Vault 或全局数据库对象。

### 12.3 与 Node Pi 共存

现有 Pi 插件继续走 `agent/src/plugins.mjs` 的扩展桥；MoeJS 插件由 Go Adapter 执行；统一 Tool Registry 把两者都展示给 Agent。不要把已有 Pi 插件“自动转换”为 MoeJS：它们可能依赖 Node API、npm、Pi 生命周期和模块加载行为。

### 12.4 性能假设与基准

测试场景需覆盖：① 纯 JS 数据转换；② 100 个轻量 Hook 注册/调用；③ 真实字幕段落处理；④ Agent 一轮工具调用的端到端耗时；⑤ Android proot 下的冷启动/常驻内存。对比 Node 内联扩展、Go 原生函数、MoeJS 以及进程插件 RPC。

指标包括 `startup_ms`、`first_action_ms`、`invoke_p50/p95_ms`、`rss_delta_mb`、`cpu_ms`、`battery_proxy`、错误率与取消可靠性。**设定门槛而非先宣称加速**：若体验指标无可测改善或维护成本明显增加，就保留 MoeJS 为可选执行器，不作为默认迁移方向。

## 13. Manifest、服务与 UI 契约示例

### 13.1 目标 manifest（建议 Plugin API v3 草案）

```json
{
  "name": "@coursive/virtual-lab",
  "version": "1.0.0",
  "coursive": {
    "id": "virtual-lab",
    "api": "^3.0.0",
    "runtime": [
      {"kind": "moejs", "entry": "logic/index.js"},
      {"kind": "web-ui", "entry": "ui", "optional": true}
    ],
    "requires": [
      {"service": "transcript.read", "version": "^1", "optional": false},
      {"service": "canvas.objects.previewWrite", "version": "^1", "optional": true}
    ],
    "provides": [
      {"service": "acoustics.raytrace", "version": "1.0.0"}
    ],
    "contributes": {
      "navigation": [
        {"id": "virtual-lab", "title": "虚拟实验室", "page": "virtual-lab.main"}
      ],
      "pages": [
        {"id": "virtual-lab.main", "renderer": "webview", "entry": "ui/index.html",
         "fallback": "virtual-lab.native-help"}
      ],
      "slots": [
        {"target": "canvas.toolbar.tools", "id": "raytrace", "action": "raytrace.open"},
        {"target": "course.detail.sections", "id": "lab-summary", "view": "lab.summary"}
      ],
      "tools": ["acoustics.raytrace"]
    },
    "permissions": ["library.read", "canvas.previewWrite", "views"]
  }
}
```

**该样例是目标协议提案，不能直接投入当前 Coursive v1/v2 插件系统运行。** `runtime` 多入口、`contributes.navigation/slots` 等均是需要新增的字段；必须提供 JSON Schema、迁移器和兼容检查。

### 13.2 Coursive UI Slot 注册结构

```json
{
  "slot": "assistant.message.after",
  "pluginId": "formula-helper",
  "entryId": "explain-actions",
  "mode": "list",
  "order": 40,
  "scope": {"kind": "conversation"},
  "renderer": "declarative",
  "component": "formula.actions.v1"
}
```

### 13.3 必须稳定的跨进程消息包

`schemaVersion`、`requestId`、`pluginId`、`instanceId`、`action`、`args`、`context`、`deadlineMs`、`traceId`；所有响应携带 `ok`、`result` 或 `error:{code,message,details}`。版本不兼容立即拒绝并输出诊断，不猜测字段意义。为状态变更引入 `revision` 与幂等键。

## 14. 权限、安全与威胁模型

### 14.1 分层信任级别

| 级别 | 插件类型 | 运行策略 |
|---|---|---|
| T0 | Coursive 内置受审计模块 | 宿主原生接口，仍受领域审批约束 |
| T1 | 签名发布的纯声明式 UI | Flutter Schema Renderer，最小危险面 |
| T2 | 可审计的 MoeJS 脚本 | 宿主 API 白名单、限时/限内存、受控模块与网络 |
| T3 | WebView 交互插件 | 独立 Origin、CSP、严格 Bridge，权限按动作判定 |
| T4 | 外部原生/Node 可执行插件 | 独立进程；条件允许时 OS 沙箱/文件与网络限制 |

来源可信（签名、散列）不等于代码安全。签名只能确认发布者与内容未被篡改，不自动授予系统能力。

### 14.2 核心威胁与缓解

| 风险 | 防护要求 |
|---|---|
| JS 插件读取凭据或整个课程库 | 不注入 token；能力句柄按权限+上下文发放；限制查询范围 |
| Web UI 伪造消息/调用别的插件 | 受信任会话绑定 identity；每次请求服务端独立鉴权 |
| SSRF / 访问本地服务 | URL 解析、DNS/重定向复核、host allowlist、loopback 禁止默认通行 |
| XSS、资源跨域、恶意跳转 | CSP、独立 Origin、禁外链/导航默认策略、资源仅从验证包加载 |
| 插件无限循环、海量分配 | 执行截止时间、内存/输出上限、限流与取消 |
| 插件热卸载后仍监听事件 | Effect 回收、代际 token、在途请求废止 |
| 绕过 Agent 审批写入用户数据 | Proposal + 权限 Broker + 领域写守卫 + 原子事务 |
| 依赖投毒、升级变更权限 | 哈希/签名/锁版本、权限增量确认、可回滚安装 |
| 第三方 Provider 原生进程越权 | 不把可执行文件简单称为沙箱，优先独立进程和 OS 隔离 |

### 14.3 必须保留的 Coursive 安全不变量

Daemon Vault 的密钥永不通过插件 UI/HTTP API 原样返回；插件只能访问自己范围内被明确授权的数据；画布模型操作遵守预览、原子提交和撤销；下载镜像不带用户凭据；删除课程文件必须由用户确认；Agent 不可修改自身审批模式。

## 15. 跨平台、性能与可用性约束

### 15.1 平台矩阵

| 平台 | L1 原生 DSL | L2 Slot | L3 Web UI | 运行时注意事项 |
|---|---|---|---|---|
| Windows | 目标必须支持 | 目标必须支持 | 需要 WebView2 等实现测评 | 安装器、Origin、输入法、DPI |
| Linux | 目标必须支持 | 目标必须支持 | 先实验后开通 | WebKit/WPE/第三方实现、动态库分发、Wayland/X11 |
| macOS | 目标必须支持 | 目标必须支持 | 可优先验证 | WKWebView、签名/沙箱与包内资源 |
| Android | 目标必须支持 | 目标必须支持 | 可优先验证 | 触摸、触控笔、前后台、proot/Node 启动成本 |

现有 Android 通过 proot Ubuntu Workspace 运行 Daemon 与 Node Agent，不能因为纯 Go MoeJS 可用就认定整个 Agent Host 可以删除。需要测试 App 初启、插件运行和输入法/手写响应之间的资源竞争。

### 15.2 NFR 验收基线（建议目标，非实测承诺）

- 插件无内容时导航与原生路由开销不应显著超过现有客户端；新增 UI 的加载失败不阻塞核心页面。
- 复杂插件内容分页/虚拟化；主线程不得同步执行 JS Hook、网络请求或媒体转换。
- 页面切换、手写采样、播放器控制应优先保证流畅；以实际低端 Android 真机的 profile/release 测量为准。
- 插件异常可被单独识别与关闭；冷启动即便插件损坏也能进入本地课程库。
- 自定义 Web UI 不得增加默认安装包的显著体积或强制所有用户下载完整浏览器运行环境；按平台评估按需安装。

## 16. 兼容与迁移路线（推荐按 PR 拆分）

### P0 基线治理与契约收敛（阻塞后续）

检查已实施的 API v1 与已批准的 v2 规范差距：插件安装状态的单一事实源、`contributes` 结构、Provider/Agent 版本选择、授权迁移与持久化数据；输出契约矩阵与回滚方案。依照仓库 `AGENTS.md` 更新对应 ADR/Approved Spec，而不是直接在旧 `PluginHost` 上叠加一个新注册中心。

**验收出口：** `v1 → v2` 兼容策略明确，所有插件只有一个真实安装/启用状态，现有 Socratic 和来源 Provider 的用例通过。

### P1 动态导航 + Slot 内核

Flutter 新增 `PluginNavigationRegistry`、`PluginPageOutlet`、`UiSlotRegistry`、`UiSlotOutlet`、`PluginFallbackPage`；Go Daemon 提供不可执行的 UI Contribution 清单；在 Shell、课程详情、画布工具栏和助手回复中设置第一批 Slot。

**验收出口：** 一个样例插件无需编辑 `shell.dart` 业务分支即可显示/隐藏原生页面；禁用当前页不崩溃；多个 Slot 贡献排序稳定。

### P2 Declarative Renderer v1

提供 JSON Schema、原生组件白名单、动作桥、主题/i18n/无障碍支持、错误与未知组件回退。先做统计卡片、表单、图表、列表、分栏、按钮，紧接着做源引用组件。

**验收出口：** 复习插件可展示完整原生页面；更换主题/屏幕大小无裁剪、触控笔交互不退化；坏 payload 不导致 App 崩溃。

### P3 Service Registry + Capability Inspector

Go/Node 统一服务描述与可用性，支持 `requires/provides`、版本/作用域/权限判定；Agent 保留 `find_tools`，新加服务查询和结构化错误。转换现有 ASR、转录搜索和画布预览能力为服务 Adapter，**不立即迁移真实业务实现**。

**验收出口：** 两个模拟的 `transcript.read` Provider 能切换；依赖缺失诊断明确；Agent 只可调用获授权 Tool。

### P4 MoeJS 执行器（独立实验）

Go 中实现可选 MoeJS Adapter、运行环境池、Host API 白名单、取消与限额、性能监测；使用字幕预处理 Hook 和纯 JSON 转换做可复现比较。

**验收出口：** 与 Node 版本相同输入产生一致结果；无限循环能中止；权限缺失不能读取文件/网络；有完整性能报告。没有收益则保持可选。

### P5 L3 自定义 Web UI（按平台准入）

WebView 资源和独立 Origin、CSP、消息桥、受控 URL 加载、数据上传确认、调试工具，独立完成 Windows/Linux/macOS/Android 设备矩阵。发布条件由安全审查与性能报告决定。

**验收出口：** “虚拟实验室”插件在支持平台可动态安装、打开、保存来源锚定的仿真对象；受限平台给出原生替代页；安全用例全通过。

### P6 插件开发者体验与市场

SDK、manifest/schema 校验 CLI、模板项目、插件模拟宿主、回归夹具、静态检查、权限扫描、文档示例、插件兼容性公告。对外维护 API Version 与最小 Host Version。

## 17. 建议的 Parent Issue / Sub-Issue 清单

> 建议建立一个 Parent Epic：`feat(plugin-platform): introduce Coursive Plugin Fabric and UI Extension Protocol`；**Issue 标题按仓库的英文 Conventional 风格，正文中文并引用本 RFC**。以下是开发拆分建议，尚未创建 GitHub Issue。

| ID | 建议子任务 | 依赖 | 交付与测试 |
|---|---|---|---|
| PF-01 | Audit existing plugin v1/v2 contract and migration state | 无 | 对照报告、契约表、ADR 修订草案 |
| PF-02 | Introduce canonical capability and contribution schemas | PF-01 | JSON Schema、跨端 fixtures、版本检查 |
| PF-03 | Add dynamic navigation/page registry to Flutter shell | PF-02 | 动态页面、禁用回退、移动端扩展入口 |
| PF-04 | Add typed UI Slot Registry and lifecycle disposal | PF-02 | Slot 清单、作用域/排序/冲突测试 |
| PF-05 | Implement declarative Flutter UI renderer v1 | PF-03/04 | 组件白名单、错误边界、主题/i18n |
| PF-06 | Implement action bridge and UI state synchronization | PF-05 | 受控 invoke、幂等、取消、状态更新 |
| PF-07 | Add service dependency resolver and provider selection | PF-02 | `requires/provides`、版本/循环/降级测试 |
| PF-08 | Expose capabilities to the Pi agent safely | PF-07 | Inspector、延迟工具加载、权限门控 |
| PF-09 | Prototype MoeJS adapter inside Go Daemon | PF-07 | Runtime 池、限额、基准对比、隔离用例 |
| PF-10 | Add canvas plugin object/renderer fallback | PF-05/06 | 引用锚点、撤销、缺失插件的降级展示 |
| PF-11 | Prototype sandboxed Web UI renderer | PF-05/06 | Bridge、CSP、各平台验证矩阵 |
| PF-12 | Publish SDK, templates, market compatibility & QA | PF-03~11 | 插件样例、文档、CI 及发布检查 |

### 推荐分工

- **平台/Go**：PF-01、02、07、09；负责权威状态、服务网关和权限。
- **Flutter/UI**：PF-03、04、05、10；负责页面、Slot、组件和跨端体验。
- **Agent/Node**：PF-06、08；负责工具协议、Tool Registry、Agent Inspector、审批与审计。
- **安全与质量**：PF-11、12，横向覆盖每个阶段的安全回归。

## 18. 测试矩阵与产品验收

### 18.1 功能测试用例

1. **NAV-01** 安装插件新增导航页，热启用/禁用，无需重启、无越界导航。
2. **NAV-02** 手机端插件页超过底部导航容量时自动收纳，可手动固定。
3. **SLOT-01** 两个插件注入同一 `list` Slot，结果排序确定且卸载独立。
4. **SLOT-02** `keyed/chain` 渲染出错时回退默认原生组件。
5. **UI-01** 非法 DSL、递归深度超限、未知组件、错误事件都返回可解释的错误。
6. **UI-02** 深浅主题、多尺寸、无障碍语义、减少动效、输入法与手写模式可用。
7. **DATA-01** 插件所创建的画布对象跨启停保留；缺插件时 Fallback 可读。
8. **DATA-02** 插件造成的画布改动可撤销；失败/取消事务无部分提交。
9. **CAP-01** 替换 `transcript.read` Provider 后下游插件不用改动。
10. **CAP-02** 缺依赖、版本不兼容、权限不足显示准确恢复路径。
11. **AGENT-01** Agent 只能展开已启用、已授权能力，不能通过 prompt 绕开写保护。
12. **MOE-01** 轻量脚本无限循环可超时中断，内存超过限制可停止并报告。
13. **WEB-01** 恶意 Web UI 无法读 Daemon Token、不能未经授权访问 loopback。
14. **WEB-02** 跨站导航/恶意 URL/消息伪造/重放请求被拒绝。
15. **REC-01** 插件崩溃、错误升级、插件目录损坏仍能进入本地课程库。
16. **REG-01** 既有 Socratic、Agent 工具、Source Provider 及插件市场核心路径无回归。

### 18.2 必须检查的性能点

真实设备上分别在“无插件”“10 插件”“50 插件”“100 个轻量 Hook”场景下测：冷启动、切页面时延、内存、Frame Timeline、CPU、耗电近似、手写延迟、播放器丢帧、插件激活/卸载耗时。不得将单独 JS 微基准当作整体性能结论。低端 Android 的 release/profile 测试优先于模拟器/debug 测试。

### 18.3 发布门槛

- 通过原有 `go test`、Node 测试、Flutter widget test、仓库 gates；新增 schema 与跨端协议契约测试。
- 演示插件覆盖 L1 声明式页面、L2 Slot、服务依赖与安全回退。
- 所有 UI 写操作以宿主权限与撤销为准；恶意插件用例在 CI 中测试。
- Web UI 仅在完成单独安全评审和各平台验收后放行，不将实验功能伪装成通用正式支持。

## 19. 风险、权衡与未决问题

| 议题 | 推荐决定 | 待确认 |
|---|---|---|
| 是否引入 Cordis 代码库本身 | **不立即直接引入**；先借鉴服务/依赖/生命周期思想 | 跨语言契约是否需要 JS 运行时内部 Cordis 兼容层 |
| 是否直接替换 Pi 插件系统 | **否**；保留原扩展，并在外面增加 Fabric | 将来是否需 Pi 兼容适配器抽象 |
| MoeJS 是否所有插件默认执行 | **否**；只做轻量任务候选执行器 | 真实端到端性能与兼容收益 |
| Web UI 是否默认开放 | **否**；高级/试验级、平台准入 | Linux/Windows WebView 可靠性、组件资源成本 |
| 插件可否替换核心 UI | 首期仅指定可回退的 Slot | V2 再考虑部分 `hoc/chain` 替换点 |
| 移动端新分页如何排列 | 扩展中心+少量可固定入口 | 用户可配置的上限和导航层级 |
| 插件创作与商店是否允许任意代码 | 默认签名/权限/审核，强隔离高风险 | 社区分发政策、第三方审核制度 |
| 用户插件对象卸载后如何展示 | Fallback 永久可读，结构化数据保留 | 插件版本升级时 schema migration 处理 |

## 20. 首个端到端 MVP 定义

**推荐 MVP：`Coursive Review Lab`（学习复习插件）**。它同时验证“新增页面 + UI Slots + Service + Agent + 降级”，却不依赖复杂 WebView。

1. 在插件市场安装后，新增原生声明式“复习”页面；手机从扩展中心进入，桌面可固定在 NavigationRail。
2. 在课程详情注入“复习进度”卡片；在笔记工具栏增加“生成复习卡”。
3. 插件依赖 `library.read` 和可选的 `llm.generate`：无 LLM 时仍可运行手工/规则复习流程。
4. Agent 可看到 `review.generate` Tool，按照用户批准范围生成卡片，附课程来源引用。
5. 停用插件时，页面和所有 Slot 消失；用户已经保存的标准卡片及其来源锚点仍可查看/导出。
6. 在完整闭环验证后，再增加 MoeJS 的 `review.normalize` Hook 作为性能试点；最后才引入 Web UI 的虚拟实验室。

**MVP 完成定义**：至少一个平台完整跑通安装、授权、页面、Slot、Agent、保存、禁用、恢复；另至少一款桌面和 Android 设备通过基本回归；提供 demo、契约测试、性能报告和回滚说明。

## 21. 仓库落地文件建议与治理动作

建议新建或更新的内容，具体目录以当前仓库规则为准：

```text
docs/decisions/         # 新 ADR：Plugin Fabric vs Pi-only；UI DSL vs WebView

docs/specs/             # 新 Approved Spec 前置：Fabric Core / UI Extensions / Web UI Security

daemon/internal/        # 新 capability registry / permission broker / moejs adapter

app/lib/core/            # navigation registry / UI slot registry / render protocol
app/lib/features/plugins/ # declarative renderer / plugin full-page outlet

agent/src/              # Tool adapter / capability inspector / compatibility bridge

app/test/ + daemon/...  # fixtures、组件/权限/协议/迁移回归
```

仓库 `AGENTS.md` 规定跨 Daemon API 变更须同时更新 App 与 Agent 客户端；风险边界变更须先走 Approved Spec；结构性决策须生成 ADR；禁止未经授权提交/推送。文档最终归档建议先进入 `docs/specs/` 的 Draft，而不是绕过审查直接进入实施。

## 22. 外部参考与源码依据

**本次设计基于公开仓库与公开文档；以下链接便于审查和实现：**

1. [Coursive README.zh-CN](https://github.com/WALKERKILLER/Coursive/blob/main/README.zh-CN.md) — 产品定位与模块布局。
2. [Coursive Architecture](https://github.com/WALKERKILLER/Coursive/blob/main/docs/architecture.md) — Flutter / Daemon / Node 进程及边界。
3. [Coursive Plugin Platform Spec 0004](https://github.com/WALKERKILLER/Coursive/blob/main/docs/specs/0004-plugin-platform.md) — 现有 Pi 扩展、Action、声明式视图。
4. [Coursive Architecture Decision 0005](https://github.com/WALKERKILLER/Coursive/blob/main/docs/decisions/0005-plugins-as-pi-extensions-with-declarative-views.md) — Pi-only 方案取舍。
5. [Coursive Source Provider Spec 0024](https://github.com/WALKERKILLER/Coursive/blob/main/docs/specs/0024-source-provider-plugins.md) — 来源 Provider、Vault、JSON-RPC。
6. [Coursive Plugin Marketplace Spec 0025](https://github.com/WALKERKILLER/Coursive/blob/main/docs/specs/0025-plugin-marketplace.md) — 插件状态、签名索引和镜像规范。
7. [Coursive Flutter Shell](https://github.com/WALKERKILLER/Coursive/blob/main/app/lib/shell.dart) — 固定路由与多端导航现状。
8. [Coursive Plugin Views](https://github.com/WALKERKILLER/Coursive/blob/main/app/lib/features/plugins/plugin_views.dart) — 原生视图目录。
9. [Coursive Agent Plugin Host](https://github.com/WALKERKILLER/Coursive/blob/main/agent/src/plugins.mjs) — Pi 插件与宿主权限网关。
10. [DSH Cordis Primer](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/cordis-primer.md) — Service、Inject、Events、Effect。
11. [DSH Web Client Slots](https://github.com/deepseek-ai/deepseek-harness/blob/master/docs/subsystems/slots.md) — 多模式 UI Slot、页面/组件注册和生命周期。
12. [DSH Sidebar](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/client/ui-sidebar/README.md) — 新导航入口与主面板插件。
13. [MoeJS](https://github.com/MoeForge/moejs) — 纯 Go JS Runtime、隔离选项与性能方法。
14. [Flutter FAQ](https://docs.flutter.dev/resources/faq) — AOT、平台与渲染机制。
15. [Flutter Performance Profiling](https://docs.flutter.dev/perf/ui-performance) — 真机 Profile/Release 性能评估建议。

---

**最终决策建议：** 保留 Pi 与原生 Flutter 的优势，在其上新增“统一插件能力总线 + 可撤销 UI Slot + 声明式页面 + 可选 WebView + MoeJS 轻量执行器”，以一个完整的“复习插件 MVP”验证闭环，再考虑推广到全平台与插件生态。这是兼顾 DSH 风格可扩展性、Coursive 产品特征、开发成本与安全性的渐进方案。
