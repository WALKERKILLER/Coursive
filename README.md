<a id="readme-top"></a>

<div align="right">
<strong>English</strong> | <a href="README.zh-CN.md">简体中文</a>
</div>

<div align="center">

<h1>Coursive</h1>

<p><strong>A local-first study canvas for recorded lectures</strong><br />
<sub>Weave subtitles, frames, notes and AI understanding into one knowledge network, written as freely as cursive</sub></p>

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
<img src="assets/brand/social/coursive-social-1280x640.png?v=3604f8dd" alt="Coursive Preview" width="92%" />
</a>

<p>
<sub><em>Read the <a href="docs/overview.md">Docs</a> · Browse the <a href="https://github.com/WALKERKILLER/coursive-plugins">Plugins</a> · Follow the <a href="docs/migration.md">Migration</a> · Join the <a href="https://github.com/WALKERKILLER/Coursive/discussions">Discussions</a></em></sub>
</p>
<br />

</div>

> **Things to note**: Coursive is moving into this repository module by module, and each part arrives with its full commit history once it has been refactored. Until then this repository holds the governance, documentation, brand assets and design contracts; there is no release to install yet.

### The Little Secret Behind the Name

**Coursive** is **Course** + **Cursive**:

- **Course**: everything a lecture leaves behind — the video, what was said, the slides on screen and the notes you took.
- **Cursive**: joined-up handwriting, where every letter flows into the next without lifting the pen.
- **Together**: subtitles, frames, notes and the assistant's understanding are not separate files but strokes of one continuous line, drawn on a canvas you write on as freely as you would by hand.

## What is Coursive

Coursive keeps your courses on your own device, turns them into searchable, linked material, and gives you a canvas and an assistant to think with:

- **Local-first library**: import lecture videos and courseware into folders you own; a rebuildable index makes everything searchable, and nothing requires an account.
- **Transcripts you can trust**: cloud or fully local speech recognition aligned to the video, with review suggestions you accept or reject.
- **Slides and frames**: courseware becomes searchable text, and sampled key frames put the screen at any moment one click away.
- **Study canvas**: an infinite canvas for handwriting, stylus ink, cards and excerpts, where every card links back to its timestamp, slide or note.
- **An assistant that cites its sources**: course-aware conversations, outlines, notes and quizzes; every action is recorded and undoable, under an approval mode you choose.
- **Plugins**: views, tools, skills and content sources install from a marketplace that picks the fastest GitHub mirror and verifies every download.
- **Your data, your sync**: export to Markdown tools, back up, or sync course folders across devices through storage you choose.

## Installation

### From Releases

Packages are attached to each release and pre-release on [Releases](https://github.com/WALKERKILLER/Coursive/releases):

- **Linux**: AppImage, deb, rpm, Arch package and tarball, for x64 and arm64.
- **Windows**: installer and portable zip.
- **macOS**: dmg and zip. The app is not notarized: the first time, right-click it and choose Open.
- **Android**: one APK per ABI (arm64-v8a, x86_64). Pre-release APKs are signed with a temporary key, so installing a later one needs an uninstall first, which removes the app's data.

### From Source

```bash
git clone https://github.com/WALKERKILLER/Coursive.git
cd Coursive
# Linux: needs Go 1.27, Node.js 22+, Flutter 3.47.7, ffmpeg and the GTK 3 and libmpv headers
(cd daemon && go build -trimpath -o bin/coursive-daemon ./cmd/coursive-daemon)
(cd agent && npm ci)
(cd app && flutter build linux --release)
./app/build/linux/x64/release/bundle/coursive
```

Toolchains, the other platforms and the test commands are in [docs/development.md](docs/development.md) and [AGENTS.md](AGENTS.md#commands).

## Directory Structure

```text
Coursive/
├── .agents/skills/     # review, decision and governance workflows for contributors and agents
├── .github/            # CI, issue and pull request templates
├── .repo-seed/         # governance manifest
├── agent/              # assistant host (Node.js)
├── app/                # Flutter app for desktop and Android
├── assets/brand/       # logo master and every exported icon
├── daemon/             # local daemon (Go): library, jobs, vault, plugins and sources
├── docs/
│   ├── decisions/      # architecture and process decisions (MADR)
│   ├── postmortems/    # incident write-ups linked to guardrails
│   └── specs/          # change contracts: Draft → Approved → Implemented
└── scripts/            # governance gates and the brand export script
```

[docs/architecture.md](docs/architecture.md) shows how the app, the daemon and the assistant host fit together.

## Plugins & Sign-in

Coursive has no built-in account and no built-in sign-in; it opens straight into your local library.

Anything that talks to an external course platform is a **source plugin**: install one and its sign-in, course list and downloads appear; remove it and they disappear while your library stays. The daemon seals the credentials a source needs and they never leave your device ([contract](docs/specs/0024-source-provider-plugins.md)).

The index, the mirror list and the official plugins live in [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins). Coursive speed-tests the available GitHub mirrors and downloads through the fastest one, checking every file against the signed index, so a mirror can make a download faster but never change it ([contract](docs/specs/0025-plugin-marketplace.md)).

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md) first; human and AI contributors follow the same rules, and every commit runs `node scripts/run-gates.mjs`. Plugin authors start in [coursive-plugins](https://github.com/WALKERKILLER/coursive-plugins).

## Community

**Feedback:**

- Report bugs and suggest features in [Issues](https://github.com/WALKERKILLER/Coursive/issues/new/choose).
- Ask questions and share setups in [Discussions](https://github.com/WALKERKILLER/Coursive/discussions).
- Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).

**Thanks:**

- Everyone whose commits travel with the migrated history.

**Built With:**

- [Flutter](https://flutter.dev), [Go](https://go.dev) and [Node.js](https://nodejs.org).
- [Pi](https://github.com/earendil-works/pi) for the assistant runtime and [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) for local speech recognition.

Coursive is released under the [MIT License](LICENSE).

---

<div align="right">
<a href="#readme-top">↑ Back to top</a>
</div>
