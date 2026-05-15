# CLI Proxy API

English | [中文](README_CN.md) | [日本語](README_JA.md)

> **Fork notice.** This is [Z-M-Huang's](https://github.com/Z-M-Huang) fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). It carries extra features (currently: **Prompt Rules** — content-level inject/strip on outgoing requests; revived logging support is planned for v0.2.0) and republishes the docker image at `zhironghuang/cli-proxy-api`. Upstream improvements are merged in periodically. For the original project, follow the upstream link.

A proxy server that provides OpenAI/Gemini/Claude/Codex compatible API interfaces for CLI.

It now also supports OpenAI Codex (GPT models) and Claude Code via OAuth.

So you can use local or multi-account CLI access with OpenAI(include Responses)/Gemini/Claude-compatible clients and SDKs.

## Sponsorship

This fork does not solicit or accept sponsorships. The original project ([router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)) maintains a list of sponsors and partner offers — see the upstream README for the current list and any associated promo codes.

## Overview

- OpenAI/Gemini/Claude compatible API endpoints for CLI models
- OpenAI Codex support (GPT models) via OAuth login
- Claude Code support via OAuth login
- Amp CLI and IDE extensions support with provider routing
- Streaming and non-streaming responses
- Function calling/tools support
- Multimodal input support (text and images)
- Multiple accounts with round-robin load balancing (Gemini, OpenAI, Claude)
- Simple CLI authentication flows (Gemini, OpenAI, Claude)
- Generative Language API Key support
- AI Studio Build multi-account load balancing
- Gemini CLI multi-account load balancing
- Claude Code multi-account load balancing
- OpenAI Codex multi-account load balancing
- OpenAI-compatible upstream providers via config (e.g., OpenRouter)
- Prompt Rules: inject standing instructions into outgoing system prompts or last user messages, or strip unwanted boilerplate (regex), scoped by model and source format and idempotent across requests
- Reusable Go SDK for embedding the proxy (see `docs/sdk-usage.md`)

## Getting Started

CLIProxyAPI Guides: [https://help.router-for.me/](https://help.router-for.me/)

## Management API

see [MANAGEMENT_API.md](https://help.router-for.me/management/api)

## Usage Statistics

This fork restores the usage tracking that upstream removed in v6.10.0 and persists it to SQLite. Each usage row includes `request_id` so the management UI can correlate usage events with persisted request histories.

- `GET /v0/management/usage` — current snapshot
- `GET /v0/management/usage/events` — paginated persisted events
- `GET /v0/management/usage/export` — backup/migration
- `POST /v0/management/usage/import` — restore from a previous export
- `GET /v0/management/request-log-by-id/:id` — fetch the persisted per-request history (used by the Request Event Detail modal)

Persistent storage is controlled by:

- `usage-statistics-enabled` — captures usage events when true
- `request-log` — captures full request/response histories to SQLite when true
- `usage-database-path` — SQLite database path, default `./data/usage.sqlite`

When using Docker Compose, `./data` is mounted to `/CLIProxyAPI/data` so the SQLite database survives container rebuilds and restarts.

Application/runtime logs remain separate. `logging-to-file` still writes rotating application logs such as `main.log`; request histories are no longer written or served from request `.log` files.

### [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper)

Standalone usage persistence and visualization service for CLIProxyAPI. This fork now integrates the same SQLite persistence idea directly into CPA, without requiring the separate Redis/RESP queue consumer.

### [CLIProxyAPI Usage Dashboard](https://github.com/zhanglunet/cliproxyapi-usage-dashboard)

Local-first usage and quota dashboard for CLIProxyAPI. It consumes the Redis-compatible usage queue into SQLite, visualizes daily and recent-window usage by account and model, and shows Codex 5h/7d quota remaining in a local web UI.

### [CPA-Manager](https://github.com/seakee/CPA-Manager)

Full CLIProxyAPI management center with request-level monitoring and cost estimates. CPA-Manager tracks collected requests by account, model, channel, latency, status, and token usage; estimates cost with editable model prices and one-click LiteLLM price sync; persists events in SQLite; and provides Codex account-pool operations with batch inspection, quota detection, unhealthy account discovery, cleanup suggestions, and one-click execution for day-to-day multi-account maintenance.

## Amp CLI Support

CLIProxyAPI includes integrated support for [Amp CLI](https://ampcode.com) and Amp IDE extensions, enabling you to use your Google/ChatGPT/Claude OAuth subscriptions with Amp's coding tools:

- Provider route aliases for Amp's API patterns (`/api/provider/{provider}/v1...`)
- Management proxy for OAuth authentication and account features
- Smart model fallback with automatic routing
- **Model mapping** to route unavailable models to alternatives (e.g., `claude-opus-4.5` → `claude-sonnet-4`)
- Security-first design with localhost-only management endpoints

When you need the request/response shape of a specific backend family, use the provider-specific paths instead of the merged `/v1/...` endpoints:

- Use `/api/provider/{provider}/v1/messages` for messages-style backends.
- Use `/api/provider/{provider}/v1beta/models/...` for model-scoped generate endpoints.
- Use `/api/provider/{provider}/v1/chat/completions` for chat-completions backends.

These routes help you select the protocol surface, but they do not by themselves guarantee a unique inference executor when the same client-visible model name is reused across multiple backends. Inference routing is still resolved from the request model/alias. For strict backend pinning, use unique aliases, prefixes, or otherwise avoid overlapping client-visible model names.

**→ [Complete Amp CLI Integration Guide](https://help.router-for.me/agent-client/amp-cli.html)**

## SDK Docs

- Usage: [docs/sdk-usage.md](docs/sdk-usage.md)
- Advanced (executors & translators): [docs/sdk-advanced.md](docs/sdk-advanced.md)
- Access: [docs/sdk-access.md](docs/sdk-access.md)
- Watcher: [docs/sdk-watcher.md](docs/sdk-watcher.md)
- Custom Provider Example: `examples/custom-provider`

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for the fork-specific branch model, upstream-sync workflow, customization surface, and release process. The general flow follows the standard fork → branch → PR pattern; PRs land in this repo's `dev` branch, not upstream.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
