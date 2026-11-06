<a id="readme-top"></a>

<div align="right">
<a href="README.md">English</a> | <strong>简体中文</strong>
</div>

<div align="center">

<h1>Coursive</h1>

<p><strong>面向录播课程的本地优先学习画布</strong><br />
<sub>把字幕、画面、笔记和 AI 的理解编织成知识网络，像连笔书写一样自由</sub></p>

<p>
<a href="https://github.com/WALKERKILLER/Coursive/stargazers"><img height="22" src="https://m3-markdown-badges.vercel.app/stars/3/3/WALKERKILLER/Coursive" alt="Stars" /></a>
&nbsp;
<a href="https://flutter.dev"><img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/Flutter/flutter3.svg" alt="Flutter" /></a>
&nbsp;
<a href="https://go.dev"><img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/Go/go3.svg" alt="Go" /></a>
&nbsp;
<a href="https://nodejs.org"><img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/NodeJS/nodejs3.svg" alt="Node.js" /></a>
&nbsp;
<a href="LICENSE"><img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/LicenceMIT/licencemit3.svg" alt="MIT" /></a>
</p>

<p>
<img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/Linux/linux3.svg" alt="Linux" />
&nbsp;
<img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/Windows/windows3.svg" alt="Windows" />
&nbsp;
<img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/macOS/macos3.svg" alt="macOS" />
&nbsp;
<img height="22" src="https://ziadoua.github.io/m3-Markdown-Badges/badges/Android/android3.svg" alt="Android" />
</p>

<a href="assets/brand/social/coursive-social-1280x640.png?v=3604f8dd">
<img src="assets/brand/social/coursive-social-1280x640.png?v=3604f8dd" alt="Coursive 预览" width="92%" />
</a>

<p>
<sub><em>阅读<a href="docs/overview.md">文档</a> · 浏览<a href="https://github.com/WALKERKILLER/coursive-plugins">插件</a> · 关注<a href="docs/migration.md">迁移进度</a> · 参与<a href="https://github.com/WALKERKILLER/Coursive/discussions">讨论</a></em></sub>
</p>
<br />

</div>

> **注意**：Coursive 正在逐个模块迁入本仓库。每个部分重构完成后，代码会连同完整的提交历史一起迁入。在此之前，本仓库只包含治理规则、文档、品牌素材和设计契约，暂时没有可安装的版本。

## 名字的由来

**Coursive** 由 **Course** 和 **Cursive** 组合而成：

- **Course**（课程）：一节课留下的全部内容，包括视频、讲课内容、屏幕上的课件和你的笔记。
- **Cursive**（连笔书写）：一气呵成的手写体，笔尖不离纸面，每个字母都连着下一个字母。
- **合在一起**：字幕、画面、笔记和助手的理解不是互不相干的文件，而是同一条连续的笔画。它们写在一张画布上，你可以像手写一样自由地书写。

## Coursive 是什么

Coursive 把课程保存在你自己的设备上，变成可搜索、相互链接的资料，并配有一张画布和一位助手，帮助你思考：

- **本地优先的课程库**：把课程视频和课件导入你自己的文件夹。可重建的索引让所有内容都能搜索，不需要任何账号。
- **可信的字幕**：支持云端或完全本地的语音识别，字幕与视频对齐。校对建议由你决定采纳或忽略。
- **课件与画面**：课件被解析为可搜索的文本。视频按关键帧采样，任何时刻的屏幕内容一点即达。
- **学习画布**：一张无限画布，承载手写、触控笔笔迹、卡片和摘录。每张卡片都连回对应的时间点、课件页或笔记。
- **会引用出处的助手**：了解课程内容的对话、大纲、笔记和练习题。每项操作都有记录、可以撤销，审批方式由你选择。
- **插件**：视图、工具、技能和内容来源都从插件市场安装。下载时自动选择最快的 GitHub 镜像，并校验每个文件。
- **数据由你掌握**：可以导出到 Markdown 工具、备份，或通过你选择的存储在多台设备间同步课程文件夹。

## 安装

### 从 Releases 安装

从首个预发布版本起，安装包会发布在 [Releases](https://github.com/WALKERKILLER/Coursive/releases) 页面：

- **Linux**：AppImage、deb、rpm、Arch 软件包和 tar 包，支持 x64 和 arm64。
- **Windows**：安装程序和免安装 zip 包。
- **macOS**：dmg 和 zip 包。
- **Android**：每种 ABI 一个 APK（arm64-v8a、x86_64）。

### 从源码构建

```bash
git clone https://github.com/WALKERKILLER/Coursive.git
cd Coursive
# Linux：需要 Go 1.27、Node.js 22+、Flutter 3.47.7、ffmpeg 以及 GTK 3 和 libmpv 开发包
(cd daemon && go build -trimpath -o bin/coursive-daemon ./cmd/coursive-daemon)
(cd agent && npm ci)
(cd app && flutter build linux --release)
./app/build/linux/x64/release/bundle/coursive
```

工具链、其他平台和测试命令见 [docs/development.md](docs/development.md) 与 [AGENTS.md](AGENTS.md#commands)。

## 目录结构

```text
Coursive/
├── .agents/skills/     # 供贡献者和智能体使用的审查、决策与治理流程
├── .github/            # CI、Issue 模板和 Pull Request 模板
├── .repo-seed/         # 治理清单
├── agent/              # 助手宿主（Node.js）
├── app/                # 桌面端与 Android 的 Flutter 应用
├── assets/brand/       # Logo 原图和全部导出图标
├── daemon/             # 本机守护进程（Go）：课程库、任务、凭据库、插件与来源
├── docs/
│   ├── decisions/      # 架构与流程决策（MADR）
│   ├── postmortems/    # 事故复盘，每篇都连到防护措施
│   └── specs/          # 变更契约：Draft → Approved → Implemented
└── scripts/            # 治理检查和品牌素材导出脚本
```

应用、守护进程和助手宿主三者的关系见 [docs/architecture.md](docs/architecture.md)。

## 插件与登录

Coursive 没有内置账号，也没有内置登录，启动后直接进入本地课程库。

所有连接外部课程平台的功能都由**来源插件**提供。安装后，它的登录页、课程列表和下载功能才会出现；卸载后这些功能随之消失，课程库中的内容保持不变。来源插件需要的凭据由守护进程加密保存，不会离开你的设备（[契约](docs/specs/0024-source-provider-plugins.md)）。

插件索引、镜像列表和官方插件维护在 [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins) 仓库。Coursive 会测量可用 GitHub 镜像的速度，通过最快的镜像下载，并对照签名索引校验每个文件。镜像只能让下载变快，无法改变下载的内容（[契约](docs/specs/0025-plugin-marketplace.md)）。

## 参与贡献

请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 和 [AGENTS.md](AGENTS.md)。人类贡献者和 AI 遵循同一套规则，每次提交前都要运行 `node scripts/run-gates.mjs`。插件作者请从 [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins) 开始。

## 社区

**反馈**：

- 在 [Issues](https://github.com/WALKERKILLER/Coursive/issues/new/choose) 报告问题、提出建议。
- 在 [Discussions](https://github.com/WALKERKILLER/Coursive/discussions) 提问、分享用法。
- 安全漏洞请按 [SECURITY.md](SECURITY.md) 私下报告。

**致谢**：

- 感谢每一位提交随迁移历史一同保留的贡献者。

**技术栈**：

- [Flutter](https://flutter.dev)、[Go](https://go.dev) 和 [Node.js](https://nodejs.org)。
- 助手运行时使用 [Pi](https://github.com/earendil-works/pi)，本地语音识别使用 [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx)。

Coursive 以 [MIT 许可证](LICENSE)发布。

---

<div align="right">
<a href="#readme-top">↑ 回到顶部</a>
</div>
