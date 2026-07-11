# CLI 代理 API

[English](README.md) | 中文 | [日本語](README_JA.md)

> **分叉说明。** 本仓库是 [Z-M-Huang](https://github.com/Z-M-Huang) 维护的 [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) 分叉，增加了提示规则、SQLite 使用量/请求历史持久化和可配置的提供商请求头，并以 `zhironghuang/cli-proxy-api` 重新发布 Docker 镜像。会定期合并上游改进。原始项目请访问上方链接。

一个为 CLI 提供 OpenAI/Gemini/Claude/Codex/Grok 兼容 API 接口的代理服务器。

现已支持通过 OAuth 登录接入 OpenAI Codex（GPT 系列）和 Claude Code。

您可以使用本地或多账户方式，通过兼容 OpenAI（包括 Responses）、Gemini、Claude 和 Grok 的客户端与 SDK 进行访问。

## 赞助说明

本分叉不主动招募或接受赞助。原始项目（[router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)）维护着完整的赞助商和合作方列表 — 当前的赞助商及优惠码请查阅上游 README。

## 功能特性

- 为 CLI 模型提供 OpenAI/Gemini/Claude/Codex/Grok 兼容的 API 端点
- 新增 OpenAI Codex（GPT 系列）支持（OAuth 登录）
- 新增 Claude Code 支持（OAuth 登录）
- 新增 Grok Build 支持（OAuth 登录）
- 在提供商支持时提供流式、非流式和 WebSocket 响应
- 函数调用/工具支持
- 多模态输入（包括提供商支持的图片和视频）
- 多账户支持与轮询负载均衡
- 支持用于路由、身份验证、执行和请求/响应拦截的动态原生插件
- 支持 Gemini AIStudio API 密钥
- 支持 AI Studio Build 多账户轮询
- 支持 Claude Code 多账户轮询
- 支持 OpenAI Codex 多账户轮询
- 支持 Grok Build 多账户轮询
- 通过配置接入上游 OpenAI 兼容提供商（例如 OpenRouter）
- 提示规则：在转发到上游前向系统提示或最近的用户消息注入常驻指令，或按正则剥离不需要的样板文本；按模型与来源格式过滤，跨请求保持幂等
- 可复用的 Go SDK（见 `docs/sdk-usage_CN.md`）

## 新手入门

CLIProxyAPI 用户手册： [https://help.router-for.me/](https://help.router-for.me/cn/)

## 管理 API 文档

请参见 [MANAGEMENT_API_CN.md](https://help.router-for.me/cn/management/api)

## 使用量统计

本分叉恢复了上游在 v6.10.0 中移除的使用量统计，并将使用量事件与逐请求历史持久化到 SQLite。每条记录包含 `request_id`，管理界面可据此关联请求详情。

- `GET /v0/management/usage` 和 `GET /v0/management/usage/overview` — 从持久化的 15 分钟汇总中读取统计快照
- `GET /v0/management/usage/events` — 按条件分页查询请求级事件
- `GET /v0/management/usage/export` — 导出保留期内的请求级备份/迁移快照
- `POST /v0/management/usage/import` — 从导出快照恢复
- `GET /v0/management/request-log-by-id/:id` — 获取持久化的逐请求历史

`usage-statistics-enabled` 控制事件记录，`request-log` 控制完整请求/响应历史，`usage-database-path` 控制数据库路径（默认 `./data/usage.sqlite`），`usage-event-retention-days` 控制请求级事件的保留天数（`0` 表示永久保留）。清理请求级事件后，汇总数据和去重键仍会保留，因此看板总量不会回退；事件列表和导出仅包含保留期内的请求。Docker Compose 会挂载 `./data`，因此容器重建后数据仍会保留。`logging-to-file` 仍仅用于轮转应用日志。

### [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper)

独立的 CLIProxyAPI 使用量持久化与可视化服务。它定期轮询管理 API 快照并写入 SQLite，随后提供聚合 API 与看板。

### [CLIProxyAPI Usage Dashboard](https://github.com/zhanglunet/cliproxyapi-usage-dashboard)

面向 CLIProxyAPI 的本地优先使用量与配额看板。它从 Redis 兼容使用量队列采集请求用量并写入 SQLite，按账号和模型可视化每日及最近时间窗口的用量，并在本地网页中显示 Codex 5h/7d 配额余量。

### [CPA-Manager](https://github.com/seakee/CPA-Manager)

面向 CLIProxyAPI 的完整管理中心，提供请求级监控和费用预估。CPA-Manager 可按账号、模型、渠道、延迟、状态和 token 用量追踪采集到的请求；支持可编辑模型价格与一键同步 LiteLLM 价格来估算费用；用 SQLite 持久化事件；并提供面向 Codex 账号池的批量巡检、配额识别、异常账号定位、清理建议与一键执行能力，适合多账号池的日常运维管理。

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
