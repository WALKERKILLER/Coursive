# Canvas draft and PDF ink lifecycle gaps

Artifact-Version: 1

## Executive summary

画布请求的发送准备跨越本地保存与模型启动两个异步阶段，停止和退出未覆盖准备阶段；PDF 笔迹复制遗漏页码，导致副本跨页显示。

## Summary

保存失败会清空尚未发送的问题。保存中退出或点击停止仍可能发起模型请求。复制第 2 页笔迹得到 page=0 的副本，翻页后不能保持原页隔离。

## Timeline

2026-11-07：沿发送、保存、退出和复制路径审查，四个新增回归场景在修复前失败；增加迟到任务取消与保存中新草稿的验证。

## Root cause

发送方法在首个保存前清空输入，保存返回后未检查页面是否退出或用户是否取消；停止按钮依赖尚未获得的 run ID。复制笔迹显式重建对象时未传递 page，默认值零使笔迹失去分页绑定。

## Guardrails

[对话生命周期测试](../../app/test/study_conversation_lifecycle_test.dart)在真实组件中控制保存和任务创建的完成时机，验证草稿恢复、准备阶段取消、退出后不启动模型、迟到任务取消以及后续草稿保留。[输入模式测试](../../app/test/study_input_modes_test.dart)通过实际框选与副本操作，重新加载保存文档后验证 PDF 页码隔离。
