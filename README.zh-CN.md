# Veil

[English](README.md) | 简体中文

在使用 AI 编码 agent 时，避免把真实密钥和结构化 PII 发送给模型厂商。

Veil 是一个本地去标识化引擎和 loopback 代理，面向 Claude Code、Codex 等
AI 编码工具。它会在受支持的文本和工具 I/O 字段离开本机前，把敏感值替换成
确定性、可逆的 token；响应回到本地后，再把 token 还原成真实值。

| 状态 | 许可证 | 适合谁 |
|---|---|---|
| v0.1.0 release | [Apache-2.0](LICENSE) | 个人开发者和网关集成者 |

## 从这里开始

| 我想要... | 阅读 |
|---|---|
| 让 Claude Code 通过 Veil 运行 | [Claude Code 设置](docs/guides/claude-code.md) |
| 让 Codex 通过 Veil 运行 | [Codex CLI 设置](docs/guides/codex.md) |
| 安装、升级或运维代理 | [部署指南](docs/guides/deployment.md) |
| 把 Veil 嵌入 Go 网关 | [SDK 集成指南](docs/sdk/integration-guide.md) |
| 理解安全边界 | [威胁模型](docs/architecture/threat-model.md) |
| 报告 bug 或漏洞 | [支持说明](SUPPORT.md) / [安全策略](SECURITY.md) |

## 快速开始

从干净 checkout 构建代理：

```sh
git clone https://github.com/PAIArtCom/Veil.git
cd Veil
go build -o ./bin/veil ./cmd/veil
./bin/veil version
./bin/veil proxy --help
```

让 Claude Code 通过 Veil：

```sh
./bin/veil proxy --addr 127.0.0.1:8788
```

另开一个 shell：

```sh
export ANTHROPIC_BASE_URL=http://127.0.0.1:8788
claude
```

让 Codex 通过 Veil：

```sh
./bin/veil proxy --addr 127.0.0.1:8788 --upstream https://api.openai.com
```

然后配置 `~/.codex/config.toml`：

```toml
model_provider = "veil"

[model_providers.veil]
name     = "Veil"
base_url = "http://127.0.0.1:8788/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

```sh
export OPENAI_API_KEY=...
codex
```

## Veil 保护什么

Veil v0.1.0 保护受支持的 provider-native 文本字段、prompt 文本、工具调用参数、
工具结果，以及流式文本/工具参数还原：

- Claude Code 通过 Anthropic Messages (`/v1/messages`)
- Codex CLI 通过 OpenAI Responses (`/v1/responses`)
- 使用公共 `github.com/PAIArtCom/Veil` 包的 Go SDK 集成

它不覆盖所有 provider surface。v0.1.0 不支持 OpenAI Chat Completions、Gemini、
remote MCP egress classification、OCR、文档解析、附件改写、媒体/文档 payload
重生成、provider thinking/control trace，也不能防护已被攻破的本机。

## 工作原理

```text
你的编码工具
  -> Veil 掩码受支持的敏感字段
  -> 模型厂商看到 PAIArtVeil_<TYPE>_<id> token
  -> 模型响应中继续引用 token
  -> Veil 在本地还原真实值
  -> 终端、文件和工具调用使用真实值
```

核心性质：

- **本地运行**：独立代理只绑定 loopback，不保存 provider 凭证。
- **Fail closed**：解析、检测、掩码、policy 或不支持的 provider 出错时，阻止明文出站。
- **确定性 token**：同一 scope 内，同一值映射到同一个 `PAIArtVeil_<TYPE>_<id>` token。
- **本地可逆**：模型看到 token；本地工具和文件拿到还原后的真实值。

## 本地 Policy

本地 policy 文件可以按类型选择行为：

```json
{
  "default_operator": "token",
  "types": {
    "EMAIL": {"operator": "ignore"},
    "SECRET": {"operator": "block"}
  }
}
```

通过 `--policy /path/to/policy.json`、`VEIL_POLICY` 或 `~/.veil/policy.json`
加载。v0.1.0 支持 `token`、`ignore`、`block`；`redact`、`format_preserving`、
未知字段和非空 `rule_sets` 都会 fail closed。

## 验证你的设置

只使用一次性测试值，不要使用真实密钥。例如让 agent 在本地任务中使用
`postgresql://app:s3cr3t@localhost:5432/mydb`，然后确认：

- 发往 provider 的文本里是 `PAIArtVeil_...` token，而不是测试值；
- 本地工具调用收到的是还原后的真实值；
- agent 写入的文件里没有未还原的 `PAIArtVeil_` token；
- 代理仍然只监听 `127.0.0.1`。

更完整的检查见：[Claude Code](docs/guides/claude-code.md) 和
[Codex CLI](docs/guides/codex.md)。

## 文档

| 主题 | 文档 |
|---|---|
| 用户指南 | [部署](docs/guides/deployment.md), [Claude Code](docs/guides/claude-code.md), [Codex CLI](docs/guides/codex.md) |
| 概念 | [脱敏模型](docs/concepts/redaction-model.md), [Token 规范](docs/concepts/token-spec.md), [检测层](docs/concepts/detection-layers.md) |
| SDK | [契约](docs/sdk/contract.md), [API 参考](docs/sdk/api-reference.md), [集成指南](docs/sdk/integration-guide.md), [`examples/embed`](examples/embed/) |
| 架构 | [总览](docs/architecture/overview.md), [威胁模型](docs/architecture/threat-model.md), [ADR](docs/architecture/decisions/README.md) |
| 项目 | [路线图](docs/product/roadmap.md), [开源边界](docs/product/open-core-boundary.md), [支持](SUPPORT.md), [安全](SECURITY.md), [Changelog](CHANGELOG.md) |

## Veil vs PAIArt

| | Veil（本仓库） | PAIArt |
|---|---|---|
| 是什么 | 本地引擎、SDK、参考代理 | 组织管控平面 |
| 给谁 | 个人开发者和网关集成者 | 安全与合规团队 |
| 许可证 | Apache-2.0 | 商业 |

开源原则：个人价值开源；组织管控收费。见
[开源边界](docs/product/open-core-boundary.md)。
