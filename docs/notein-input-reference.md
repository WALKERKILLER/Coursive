# Notein 输入行为：APK 静态参考

本文是输入交互的证据索引，不是 Notein 源码或运行时一致性声明。Coursive 的实际规则见[画布输入](drawing-controls.md)，取舍见[笔长按决策](decisions/0027-stylus-card-holds.md)。

## 分析对象

用户提供的 Notein 官方 APK，versionName `1.2.607.0`、versionCode `346`，SHA-256 `3a5b703f06174117741e3c4b9f27ecabefe7d02e9be65084c344e67a372353f6`。通过 REA MCP 的 `inspect_android_class`、`inspect_android_method` 和 `trace_android_references` 定向检查，由 JADX headless MCP `0.7.1` 解码。下表的方法名保留混淆后的名称以便复核；Java 反编译失败的图片拖动方法使用 DEX/smali 回退。

## 可确认的机制

| 问题 | 静态证据 | 结论及边界 |
| --- | --- | --- |
| 长按在哪里命中对象 | `com.orion.editor.OooO00o$OooO0o0.onDown/onLongPress` | 落点收集对象候选；长按按图片、形状、文本、引用分类处理。命中图片/形状时取消工具的临时绘制，再打开对象编辑。顶层 `OooO00o.onLongPress` 本身不实现这套逻辑。 |
| 笔和手指是否同一路由 | `com.orion.editor.PDFView.onTouchEvent` | 区分 tool type、输入模式、编辑模式与多个 GestureDetector；笔和倒置笔进入专用工具处理，长按对象选中状态阻止继续向绘图分支分发。不能把某一分支解释成所有模式都同样工作。 |
| 选中后怎么拖 | `ImageSizeAndRotationEditor.onFinishInflate` → `o0OO00o0.OooOOO0.onTouch` → `ImageEditorPanel.onMove` | 图片内容注册独立触摸监听。DOWN 记录屏幕落点并阻止父级拦截，MOVE 用相对落点位移更新对象位置，UP 完成操作。此监听没有按笔/手指筛选，也没有再次长按才允许平移的门槛。 |
| 已选图片再次长按 | 同一 `onTouch` 与 `onFinishInflate` | 同时安排 1000 毫秒回调；位移任一轴超过 20 个原始屏幕坐标单位时取消等待。长点击处理器可发起系统 `startDragAndDrop`。这是另一条跨视图拖放路径，不能与画布内平移混为一谈。 |
| 停笔如何触发成形 | `com.orion.penkit.tools.OooO00o.OooO0o` → `com.orion.penkit.tools.OooO0o.OooO0o` | 先积累压力点；MOVE 中检测末端稳定，单轴偏移小于 5 个工具坐标单位，稳定超过 600 毫秒设置识别标志。识别有结果后创建形状、选择锚点，继续移动调整锚点，抬笔提交。不能将这里的 5 直接称为屏幕逻辑像素。 |
| 怎样识别形状 | `o0O000o0.OooO00o.OooO00o` | 使用路径长度比例构造近似精度，并调用 OpenCV 轮廓近似，结合顶点数、闭合和边角关系分类；存在三角形、矩形/正方形等分支。Coursive 沿用自己的几何拟合器，不移植此实现，也不承诺同一识别率。 |

对象编辑的开启入口 `o0O000oo.OooOO0o.OoooO00` 会拒绝固定图片，再把对象交给类型相应的编辑器。这支持把“可选中”和“允许修改”分别检查，而不是仅凭选中描边允许移动。

## REA 证据索引

- 对象长按入口：`ev_e063e932553b781197cfed00c5ffa62e3aa2de2d68a3ec510d0cb34fcb6487cc` 所在 `drag-20261107-long-down.json`；对应 `drag-20261107-long-c.json`。
- 对象编辑分发：`drag-20261107-activate-editor.json`；接口调用引用：`drag-20261107-edit-open-refs.json`。
- 图片内容监听：`drag-20261107-selected-image-listeners.json`；DEX 拖动路径：`drag-20261107-selected-image-drag.json`；位置更新：`drag-20261107-image-move.json`。
- 绘图停留检测：`drag-20261107-tool-input.json`；几何识别：`drag-20261107-shape-recognition.json`；笔工具处理：`tool-type0-input.txt`。

证据文件保存在调查工作区，不进入课程同步库；仓库只保留行为总结和 APK 摘要。混淆及反编译警告意味着静态证据不能替代真实手写笔测试，尤其不能证明所有输入模式、掌拒或不同 Notein 版本行为。没有复制 APK 实现代码或美术资源。
