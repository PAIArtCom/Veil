# OpenCloak

[English](README.md) | 简体中文

> LLM 时代的去标识化层 —— 在不向模型厂商泄露密钥与隐私的前提下，放心使用 AI 编码工具。

OpenCloak 位于你的开发工具（Claude Code、Codex、Copilot、Cursor……）与 LLM 之间。请求
离开本机前，它把敏感值**确定性地替换为可逆 token**；响应回来时再**还原**。模型永远看
不到真实数据 —— 但你的终端、文件、以及 agent 的工具调用，全部使用真实值运行。

> **状态：v0.1.0 release candidate 强化中。** 文本引擎、Anthropic Messages wire
> 掩码/还原、流式还原、本地 Claude Code 代理、SDK 内嵌参考集成、OpenAI Responses
> wire adapter、以及本地 policy 文件均已实现并通过测试。Claude Code 路径已通过真实
> 流量验收；Codex/OpenAI Responses 目前是离线验证通过，仍需真实 Codex 受控验收后
> 才能声明 release-candidate ready。

---

## 痛点

AI 编码 agent 会把你的代码、配置和 shell 上下文流式发送给第三方 LLM。API key、token、
连接串、个人数据默认随之外泄。面对它，组织要么封禁工具（损失生产力），要么默许泄露。

OpenCloak 取消这个取舍：保住生产力、堵住泄露 —— 本地完成、无可感知延迟、且不破坏 agent。

## 快速开始

```sh
go build -o ./bin/opencloak ./cmd/opencloak
./bin/opencloak version
./bin/opencloak proxy --help
./bin/opencloak proxy --addr 127.0.0.1:8788
```

Claude Code:

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

可选本地 policy 文件支持 `token`、`ignore`、`block`，通过
`--policy /path/to/policy.json`、`OPENCLOAK_POLICY` 或 `~/.opencloak/policy.json`
加载。`redact`、`format_preserving` 和非空 `rule_sets` 在 v0.1.0 中仍会 fail closed。

## 工作原理（一图）

```
  你的开发工具  (Claude Code / Codex / …)
       │  含真实密钥与 PII 的请求
       ▼
  ┌──────────────────────────────────────────────┐
  │  OpenCloak   (独立代理 或 内嵌库)               │
  │  ① 检测  → ② 掩码 → 可逆 token                  │
  │     例如  sk-live-abc…  →  CLK_SECRET_7f3a…    │
  └──────────────────────────────────────────────┘
       │  脱敏后的请求 —— 厂商看不到真实数据
       ▼
  LLM 厂商  (Anthropic / OpenAI / …)
       │  响应与工具调用引用 CLK_SECRET_7f3a…
       ▼
  ┌──────────────────────────────────────────────┐
  │  OpenCloak   ③ 还原 token → 真实值              │
  └──────────────────────────────────────────────┘
       │  真实值 —— 工具、文件、终端均正常工作
       ▼
  你的开发工具
```

三个性质让它既安全又无感：

- **只有两个转换点** —— 去往 LLM 的方向掩码，返回的方向还原。本地的一切（工具执行、
  写文件、终端显示）都不动。见[脱敏模型](docs/concepts/redaction-model.md)。
- **确定性、可逆、类型感知的 token**（`CLK_<TYPE>_<id>`）—— 同一个值永远映射到同一个
  token，因此 prompt 缓存保持命中、多轮上下文保持连贯。见 [token 规范](docs/concepts/token-spec.md)。
- **分层检测** —— L1 模式匹配（密钥、结构化 PII）先行；可选的 L2 本地 NER 模型
  （人名、地址）后续。见[检测层](docs/concepts/detection-layers.md)。

## 两种运行方式

OpenCloak 是**一套引擎、不同外壳**（见[架构总览](docs/architecture/overview.md)）：

1. **独立本地代理** —— 把 CLI 的 base URL 指向它（Claude Code 用 `ANTHROPIC_BASE_URL`；
   Codex 通过自定义 `model_providers` 条目走 OpenAI Responses；Codex 路径仍需真实验收后
   才能声明 release-accepted）。凭证原样透传，只改写请求体。
2. **可嵌入 Go 库** —— 把引擎放进你自己的网关，在你的请求/响应接缝处调用。SDK 是
   **通用的**，并由仓库内维护的参考集成验证 —— 不为任何单一网关定制。见
   [SDK 契约](docs/sdk/contract.md) 与 [`examples/embed`](examples/embed/)。

## OpenCloak vs Cloakia

| | **OpenCloak**（本仓库 · Apache-2.0） | **Cloakia**（商业） |
|---|---|---|
| 是什么 | 本地引擎 + SDK + 参考代理 | 组织管控平面 |
| 给谁 | 个人开发者 —— 免费、到处可嵌 | 安全与合规团队 |

开源原则：**个人价值开源；组织管控收费。** 完整拆分见
[开源切割线](docs/product/open-core-boundary.md)。

## 文档

从**[文档地图](docs/README.md)**开始。重点：

- [产品策略](docs/product/strategy.md) · [路线图](docs/product/roadmap.md)
- [架构总览](docs/architecture/overview.md) · [威胁模型](docs/architecture/threat-model.md) ·
  [决策记录](docs/architecture/decisions/README.md)
- [SDK 契约](docs/sdk/contract.md) · [网关集成调研](docs/research/gateway-integration-survey.md)
- [Claude Code 指南](docs/guides/claude-code.md) · [Codex CLI 指南](docs/guides/codex.md) ·
  [部署指南](docs/guides/deployment.md)

## 许可证

[Apache-2.0](LICENSE)。
