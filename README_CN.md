# CLI 代理 API

[English](README.md) | 中文 | [日本語](README_JA.md)

> **分叉说明。** 本仓库是 [Z-M-Huang](https://github.com/Z-M-Huang) 维护的 [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 分叉，附带额外功能（当前：**提示规则** — 出站请求的内容级注入/剥离；恢复版日志支持计划在 v0.2.0 上线），并以 `zhironghuang/cli-proxy-api` 重新发布 Docker 镜像。会定期合并上游改进。原始项目请访问上方链接。

一个为 CLI 提供 OpenAI/Gemini/Claude/Codex 兼容 API 接口的代理服务器。

现已支持通过 OAuth 登录接入 OpenAI Codex（GPT 系列）和 Claude Code。

您可以使用本地或多账户的CLI方式，通过任何与 OpenAI（包括Responses）/Gemini/Claude 兼容的客户端和SDK进行访问。

## 赞助说明

本分叉不主动招募或接受赞助。原始项目（[router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)）维护着完整的赞助商和合作方列表 — 当前的赞助商及优惠码请查阅上游 README。

## 功能特性

- 为 CLI 模型提供 OpenAI/Gemini/Claude/Codex 兼容的 API 端点
- 新增 OpenAI Codex（GPT 系列）支持（OAuth 登录）
- 新增 Claude Code 支持（OAuth 登录）
- 支持流式与非流式响应
- 函数调用/工具支持
- 多模态输入（文本、图片）
- 多账户支持与轮询负载均衡（Gemini、OpenAI、Claude）
- 简单的 CLI 身份验证流程（Gemini、OpenAI、Claude）
- 支持 Gemini AIStudio API 密钥
- 支持 AI Studio Build 多账户轮询
- 支持 Gemini CLI 多账户轮询
- 支持 Claude Code 多账户轮询
- 支持 OpenAI Codex 多账户轮询
- 通过配置接入上游 OpenAI 兼容提供商（例如 OpenRouter）
- 提示规则：在转发到上游前向系统提示或最近的用户消息注入常驻指令，或按正则剥离不需要的样板文本；按模型与来源格式过滤，跨请求保持幂等
- 可复用的 Go SDK（见 `docs/sdk-usage_CN.md`）

## 新手入门

CLIProxyAPI 用户手册： [https://help.router-for.me/](https://help.router-for.me/cn/)

## 管理 API 文档

请参见 [MANAGEMENT_API_CN.md](https://help.router-for.me/cn/management/api)

## 使用量统计

本分叉恢复了上游在 v6.10.0 中移除的内存使用量统计，并为每条记录新增了 `request_id` 字段，用于关联磁盘上的逐请求日志文件。统计数据展示在管理界面的"使用量"标签页中，并通过以下管理 API 暴露：

- `GET /v0/management/usage` — 当前快照
- `GET /v0/management/usage/export` — 备份/迁移
- `POST /v0/management/usage/import` — 从导出快照恢复
- `GET /v0/management/request-log-by-id/:id` — 获取逐请求日志文件（被"请求事件详情"模态框使用）

如需在内存存储之外进行外部持久化（如 SQLite、更长保留期），可选用 [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper)，它通过轮询管理 API 实现。

## Amp CLI 支持

CLIProxyAPI 已内置对 [Amp CLI](https://ampcode.com) 和 Amp IDE 扩展的支持，可让你使用自己的 Google/ChatGPT/Claude OAuth 订阅来配合 Amp 编码工具：

- 提供商路由别名，兼容 Amp 的 API 路径模式（`/api/provider/{provider}/v1...`）
- 管理代理，处理 OAuth 认证和账号功能
- 智能模型回退与自动路由
- 以安全为先的设计，管理端点仅限 localhost

当你需要某一类后端的请求/响应协议形态时，优先使用 provider-specific 路径，而不是合并后的 `/v1/...` 端点：

- 对于 messages 风格的后端，使用 `/api/provider/{provider}/v1/messages`。
- 对于按模型路径暴露生成接口的后端，使用 `/api/provider/{provider}/v1beta/models/...`。
- 对于 chat-completions 风格的后端，使用 `/api/provider/{provider}/v1/chat/completions`。

这些路径有助于选择协议表面，但当多个后端复用同一个客户端可见模型名时，它们本身并不能保证唯一的推理执行器。实际的推理路由仍然根据请求里的 model/alias 解析。若要严格钉住某个后端，请使用唯一 alias、前缀，或避免让多个后端暴露相同的客户端模型名。

**→ [Amp CLI 完整集成指南](https://help.router-for.me/cn/agent-client/amp-cli.html)**

## SDK 文档

- 使用文档：[docs/sdk-usage_CN.md](docs/sdk-usage_CN.md)
- 高级（执行器与翻译器）：[docs/sdk-advanced_CN.md](docs/sdk-advanced_CN.md)
- 认证: [docs/sdk-access_CN.md](docs/sdk-access_CN.md)
- 凭据加载/更新: [docs/sdk-watcher_CN.md](docs/sdk-watcher_CN.md)
- 自定义 Provider 示例：`examples/custom-provider`

## 贡献

请参阅 [CONTRIBUTING.md](./CONTRIBUTING.md)，其中包含本分叉特定的分支模型、上游同步工作流、定制范围和发布流程。整体流程遵循标准的 fork → 分支 → PR 模式；PR 提交到本仓库的 `dev` 分支，而非上游。

## 许可证

此项目根据 MIT 许可证授权 - 有关详细信息，请参阅 [LICENSE](LICENSE) 文件。
