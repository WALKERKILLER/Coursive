# Restored canvas history failed before a model request

Artifact-Version: 1

## Executive summary

画布第一轮回答成功，后续追问却显示通用模型请求错误。SDK 在估算恢复的历史消息时读取缺失的 `usage.totalTokens`，尚未发出网络请求就失败。

## Summary

基础连通性和单轮工具调用测试通过。带历史纯文本回答的请求可稳定复现同一异常，因此重新配置密钥不能解决问题。

## Timeline

2026-11-07：收到连续追问失败截图；先验证基础请求及工具往返，再用合成历史消息复现本地 SDK 异常；增加真实 SDK 边界回归测试并修复。

## Root cause

持久对话只保存问题与回答，恢复为 SDK assistant 消息时未提供 usage。既有编排测试替换了 completeSimple，未执行 SDK 的上下文估算。恢复消息需提供全零 usage，让 SDK 根据文本估算上下文；该占位不写入费用账本、不虚构历史费用。

## Guardrails

[画布助手回归测试](../../agent/src/canvas-agent.test.mjs)调用所发布 SDK 的上下文估算函数，验证历史回复与新问题可被估算，且未知的历史费用保持零。既有测试继续验证历史不能重新引入已移除的资料。
