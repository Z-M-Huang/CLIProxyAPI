# CLI Proxy API

English | [中文](README_CN.md) | [日本語](README_JA.md)

> **Fork notice.** This is [Z-M-Huang's](https://github.com/Z-M-Huang) fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). It adds Prompt Rules, persistent SQLite usage/request history, and configurable provider headers, and republishes the Docker image as `zhironghuang/cli-proxy-api`. Upstream improvements are merged periodically. For the original project, follow the upstream link.

A proxy server that provides OpenAI/Gemini/Claude/Codex/Grok compatible API interfaces for CLI.

It now also supports OpenAI Codex (GPT models) and Claude Code via OAuth.

So you can use local or multi-account access with OpenAI (including Responses), Gemini, Claude, and Grok-compatible clients and SDKs.

## Sponsorship

This fork does not solicit or accept sponsorships. The original project ([router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)) maintains a list of sponsors and partner offers — see the upstream README for the current list and any associated promo codes.

## Overview

- OpenAI/Gemini/Claude/Grok compatible API endpoints for CLI models
- OpenAI Codex support (GPT models) via OAuth login
- Claude Code support via OAuth login
- Grok Build support via OAuth login
- Streaming, non-streaming, and WebSocket responses where supported
- Function calling/tools support
- Multimodal input support, including images and video where supported
- Multiple accounts with round-robin load balancing
- Dynamic native plugins for routing, auth, execution, and request/response interception
- Generative Language API Key support
- AI Studio Build multi-account load balancing
- Claude Code multi-account load balancing
- OpenAI Codex multi-account load balancing
- Grok Build multi-account load balancing
- OpenAI-compatible upstream providers via config (e.g., OpenRouter)
- Prompt Rules: inject standing instructions into outgoing system prompts or last user messages, or strip unwanted boilerplate (regex), scoped by model and source format and idempotent across requests
- Reusable Go SDK for embedding the proxy (see `docs/sdk-usage.md`)

## Getting Started

CLIProxyAPI Guides: [https://help.router-for.me/](https://help.router-for.me/)

## Management API

see [MANAGEMENT_API.md](https://help.router-for.me/management/api)

## Usage Statistics

This fork restores the usage tracking that upstream removed in v6.10.0 and persists it to SQLite. Each usage row includes `request_id` so the management UI can correlate usage events with persisted request histories.

- `GET /v0/management/usage` and `GET /v0/management/usage/overview` — aggregate snapshot from durable 15-minute rollups
- `GET /v0/management/usage/events` — filtered, paginated request-level events
- `GET /v0/management/usage/export` — retained request-level backup/migration snapshot
- `POST /v0/management/usage/import` — restore from a previous export
- `GET /v0/management/request-log-by-id/:id` — fetch the persisted per-request history (used by the Request Event Detail modal)

Persistent storage is controlled by:

- `usage-statistics-enabled` — captures usage events when true
- `request-log` — captures full request/response histories to SQLite when true
- `usage-database-path` — SQLite database path, default `./data/usage.sqlite`
- `usage-event-retention-days` — days to keep request-level events; `0` keeps them indefinitely

Rollups and deduplication keys are retained after request-level event pruning, so dashboard totals remain stable. Request-event pages and exports contain only events still inside the configured raw-event retention window.

When using Docker Compose, `./data` is mounted to `/CLIProxyAPI/data` so the SQLite database survives container rebuilds and restarts.

Application/runtime logs remain separate. `logging-to-file` still writes rotating application logs such as `main.log`; request histories are no longer written or served from request `.log` files.

### [CPA Usage Keeper](https://github.com/Willxup/cpa-usage-keeper)

Standalone usage persistence and visualization service for CLIProxyAPI. This fork now integrates the same SQLite persistence idea directly into CPA, without requiring the separate Redis/RESP queue consumer.

### [CLIProxyAPI Usage Dashboard](https://github.com/zhanglunet/cliproxyapi-usage-dashboard)

Local-first usage and quota dashboard for CLIProxyAPI. It consumes the Redis-compatible usage queue into SQLite, visualizes daily and recent-window usage by account and model, and shows Codex 5h/7d quota remaining in a local web UI.

### [CPA-Manager](https://github.com/seakee/CPA-Manager)

Full CLIProxyAPI management center with request-level monitoring and cost estimates. CPA-Manager tracks collected requests by account, model, channel, latency, status, and token usage; estimates cost with editable model prices and one-click LiteLLM price sync; persists events in SQLite; and provides Codex account-pool operations with batch inspection, quota detection, unhealthy account discovery, cleanup suggestions, and one-click execution for day-to-day multi-account maintenance.

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
