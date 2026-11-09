# Review Lab

An example v3 plugin that exercises every Plugin Fabric surface the app ships: a declarative page (复习卡), a course-detail slot, bridge actions that keep their state in the daemon, an agent tool (`make_cards`) and a write (存为笔记) that runs only after the user approves it. It is not in the [catalog](../../catalog.json), so it never installs by itself; the [guide](../../../../docs/plugin-v3-guide.md) walks through it.

```sh
node agent/scripts/plugin-sim.mjs agent/plugins/_examples/review-lab          # validate, install, run sim/cases.json
node agent/scripts/plugin-sim.mjs agent/plugins/_examples/review-lab --pack dist
```

To try it in the app, turn on 允许第三方插件 and install from this folder (从文件夹安装…). [review-lab-e2e.mjs](../../../scripts/review-lab-e2e.mjs) runs the same flow against a real daemon and assistant host.
