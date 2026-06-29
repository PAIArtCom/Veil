<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="web/public/logo-dark.svg">
    <img src="web/public/logo.svg" alt="Veil" width="170">
  </picture>
</p>

<p align="center">
  <strong>面向 AI 编程助手的本地脱敏代理。</strong>
</p>

[English](README.md) | 简体中文

Veil 是面向 AI 编程助手的本地脱敏代理。在 Claude Code 或 Codex 请求离开本机前，它把 API key、数据库连接串和 PII 替换成确定性、可逆的安全占位符，并在响应返回本地后还原真实值。模型厂商只看到占位符，你的工具和工作流保持不变。

| 状态 | 许可证 | 平台 |
|---|---|---|
| v0.1.2 | Apache-2.0 | macOS · Linux · Windows（amd64 / arm64） |

## 安装

Release 安装器依赖对应版本的 GitHub Release 产物已经发布。如果你正在使用尚未发布的
checkout，请使用[源码编译](#源码编译)。

### macOS 和 Linux

```sh
curl -fsSL https://veil.paiart.com/install.sh | sh
```

安装指定版本或自定义目录：

```sh
curl -fsSL https://veil.paiart.com/install.sh | VEIL_VERSION=v0.1.2 sh
curl -fsSL https://veil.paiart.com/install.sh | VEIL_INSTALL_DIR="$HOME/bin" sh
```

### npm / Node.js

```sh
npm install -g @paiart/veil
```

macOS、Linux、Windows 通用。安装时从对应版本的 GitHub Release 自动下载当前平台二进制。

### macOS — Homebrew

```sh
brew tap PAIArtCom/veil
brew install veil
```

稳定版本的 release automation 会把 formula 发布到
[`PAIArtCom/homebrew-veil`](https://github.com/PAIArtCom/homebrew-veil)。

### Windows — PowerShell

```powershell
irm https://veil.paiart.com/install.ps1 | iex
```

二进制文件安装到 `%USERPROFILE%\.veil\bin`，并自动加入用户 `PATH`。

### Go install

```sh
go install github.com/PAIArtCom/Veil/cmd/veil@latest
```

### 源码编译

```sh
git clone https://github.com/PAIArtCom/Veil.git
cd Veil
go build -o ./bin/veil ./cmd/veil
```

各平台预编译包见 [releases 页面](https://github.com/PAIArtCom/Veil/releases/latest)。

---

## 让 AI 帮你装

不想手动操作？把下面这段话发给你的 AI 助手，让它替你完成所有步骤：

```
帮我安装配置 Veil（https://github.com/PAIArtCom/Veil）。它是一个本地代理，在 prompt 发给模型厂商之前把 API 密钥、数据库密码等敏感信息替换成占位符，响应回来后在本地自动还原。

请完成以下步骤：
① 检查是否安装了 Go，没有就先装；
② git clone 仓库并编译，把可执行文件放到 ~/bin/veil 并确保它在 PATH 里；
③ 执行 `veil service install` 安装并启动后台服务；
④ 创建或更新 `~/.claude/settings.json`，把 `env.ANTHROPIC_BASE_URL` 设为 `http://127.0.0.1:8787`；
⑤ 用测试值 postgresql://app:s3cr3t@localhost:5432/mydb 验证脱敏生效；
⑥ 最后告诉我平时怎么用。

遇到任何错误请自行诊断修复后继续，不需要中途问我。
```

完成后重启 Claude Code 即可生效。Codex 接入方式不同，参考[快速开始](#跑起来)手动配置。

## 效果如何

```
# 没用 Veil，厂商实际收到：
"...连接 postgresql://app:s3cr3t@db.internal:5432/mydb..."
"export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"

# 用了 Veil，厂商收到：
"...连接 PAIArtVeil_SECRET_a1b2c3d4..."
"export AWS_ACCESS_KEY_ID=PAIArtVeil_SECRET_e5f6g7h8..."
# 真实值在本地还原，你的终端和文件始终看到的是真实值
```

## 跑起来

[安装](#安装) Veil 后，先安装一次后台服务，再把 AI 工具指向本地 base URL。之后不需要每次手动运行 `veil proxy`。

```sh
veil service install
veil status
```

`veil service install` 会在 macOS 上创建 `launchd` 用户服务，在 Linux 上创建
`systemd --user` 服务，在 Windows 上创建 Task Scheduler 任务，登录后自动在后台运行本地代理。
改过服务参数后，用 `veil restart` 重启。

**Claude Code 接入：**

写入 `~/.claude/settings.json`：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:8787"
  }
}
```

**Codex 接入：**

```sh
veil service install --force --upstream https://api.openai.com
```

在 `~/.codex/config.toml` 里加上：

```toml
model_provider = "veil"

[model_providers.veil]
name     = "Veil"
base_url = "http://127.0.0.1:8787/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

就这些了。你的 API 凭证还是正常走——Veil 只对请求和响应里的敏感数据做脱敏。

**Codex + OpenRouter：**

OpenRouter 要走 Responses API 形态。Veil 的 upstream 写 `https://openrouter.ai/api`，
这样 Codex 发到本地的 `/v1/responses` 会被转发成 OpenRouter 的
`/api/v1/responses`。

后台服务可以保持默认 upstream，把 OpenRouter 直接写进本地 base URL：

```toml
model_provider = "veil-openrouter"

[model_providers.veil-openrouter]
name     = "Veil OpenRouter"
base_url = "http://127.0.0.1:8787/veil/upstream=https://openrouter.ai/api/v1"
wire_api = "responses"
env_key  = "OPENAI_API_KEY"
```

Codex 会追加 `/responses`，Veil 最终转发到
`https://openrouter.ai/api/v1/responses`。不需要 URL 转义，也不需要额外的 upstream 命令。

不要把 Chat Completions 客户端直接指向 Veil。当前版本没有 Chat Completions
wire adapter，不支持的端点会失败即拦截，而不是透明转发明文。

## 能保护哪些数据

出站前自动检测脱敏，响应回来后在本地还原：

| 类型 | 示例 |
|---|---|
| **密钥 / 凭证** | API key、token、密码、连接字符串 |
| **邮箱** | `user@example.com` |
| **电话** | `+86 138 0000 0000` |
| **IP 地址** | `192.168.1.1`、`2001:db8::1` |
| **银行卡号** | `4111 1111 1111 1111` |
| **账户号码** | 金融类账户标识符 |
| **URL** | `https://internal.company.com/api` |
| **日期** | 默认关闭，按需在规则文件里开启 |
| **姓名 / 地址** | 默认关闭，需启用 L2 语义检测 |

v0.1.2 已支持：
- **Claude Code** — Anthropic Messages API（`/v1/messages`）
- **Codex CLI** — OpenAI Responses API（`/v1/responses`）
- **OpenRouter** — 通过 Codex CLI 和 OpenRouter Responses（`/api/v1/responses`）
- **Go SDK** — `github.com/PAIArtCom/Veil`

暂不支持：Chat Completions 客户端、Gemini、远程 MCP egress 分类、OCR、文档解析和附件改写。

## 原理

```
你的编程工具
  → Veil 在本地把敏感字段换成占位符
  → 厂商只看到 PAIArtVeil_<TYPE>_<id> 形式的占位符
  → 响应里的占位符原样带回来
  → Veil 在本地还原真实值
  → 终端、文件、工具调用拿到的都是真实值
```

**为什么可信：**

- **只在本地跑** — 代理只监听 `127.0.0.1`，不经过任何中间服务器，Veil 也不存储你的凭证。
- **出错就拦** — 解析失败、检测出错、规则不通过、端点不支持，请求直接阻断，不会把明文发出去。
- **占位符是确定性的** — 同一内容在同一会话里始终映射到同一占位符，多轮对话和 prompt cache 不受影响。
- **还原只在本地** — 厂商永远看不到真实值；你的工具和写出来的文件里都是真实值。
- **不收集任何数据** — 引擎不回传任何信息。

## 自定义规则

用本地规则文件按数据类型配置处理方式：

```json
{
  "default_operator": "token",
  "types": {
    "EMAIL": {"operator": "ignore"},
    "SECRET": {"operator": "block"}
  }
}
```

通过 `--policy /path/to/policy.json`、环境变量 `VEIL_POLICY`，或直接放到
`~/.veil/policy.json` 自动加载。

| 处理方式 | 效果 |
|---|---|
| `token` | 替换为可逆占位符（默认） |
| `ignore` | 原样放行 |
| `block` | 直接拒绝这条请求 |

## 确认是否生效

用一个假值测，**别用真实密钥**：

```
postgresql://app:s3cr3t@localhost:5432/mydb
```

让 AI 工具在本地任务里用这个字符串，然后检查：

- 发给厂商的内容是 `PAIArtVeil_...` 占位符，不是原值
- 本地工具调用收到的是还原后的连接字符串
- AI 写出来的文件里没有未还原的 `PAIArtVeil_` 占位符
- 代理仍然只监听 `127.0.0.1`

详细步骤见：[Claude Code 指南](docs/guides/claude-code.md) ·
[Codex 指南](docs/guides/codex.md)

## 快速导航

| 我想做的事 | 去哪里 |
|---|---|
| 让 Claude Code 走 Veil | [Claude Code 接入指南](docs/guides/claude-code.md) |
| 让 Codex 走 Veil | [Codex CLI 接入指南](docs/guides/codex.md) |
| 让 Codex 通过 OpenRouter 走 Veil | [OpenRouter 接入指南](docs/guides/openrouter.md) |
| 安装、升级、运维 | [部署文档](docs/guides/deployment.md) |
| 把 Veil 嵌进 Go 网关 | [SDK 集成指南](docs/sdk/integration-guide.md) |
| 搞清楚安全边界 | [威胁模型](docs/architecture/threat-model.md) |
| 报 bug 或安全问题 | [反馈渠道](SUPPORT.md) · [安全策略](SECURITY.md) |

## Veil 和 PAIArt 的关系

Veil 是开源的脱敏引擎，个人用免费。PAIArt 是面向团队的商业管控平台，提供集中下发规则和合规审计。

| | Veil（本仓库） | PAIArt |
|---|---|---|
| 是什么 | 本地引擎、SDK 和参考代理 | 组织管控平台 |
| 适合谁 | 个人开发者、网关集成商 | 安全团队、合规负责人 |
| 规则管理 | 本地 JSON 文件 | 集中下发，全员统一 |
| 审计 | 自己接 `AuditSink` | 合规看板、SIEM 导出 |
| 许可证 | Apache-2.0 | 商业授权 |

个人用的部分开源；团队管控的部分收费。详见[开源边界说明](docs/product/open-core-boundary.md)。

## 文档

| 主题 | 文档 |
|---|---|
| 使用指南 | [部署](docs/guides/deployment.md)、[Claude Code](docs/guides/claude-code.md)、[Codex CLI](docs/guides/codex.md)、[OpenRouter](docs/guides/openrouter.md) |
| 概念说明 | [脱敏模型](docs/concepts/redaction-model.md)、[占位符规范](docs/concepts/token-spec.md)、[检测层](docs/concepts/detection-layers.md) |
| SDK | [接口约定](docs/sdk/contract.md)、[API 参考](docs/sdk/api-reference.md)、[集成指南](docs/sdk/integration-guide.md)、[`examples/embed`](examples/embed/) |
| 架构 | [系统总览](docs/architecture/overview.md)、[威胁模型](docs/architecture/threat-model.md)、[架构决策](docs/architecture/decisions/README.md) |
| 项目信息 | [路线图](docs/product/roadmap.md)、[开源边界](docs/product/open-core-boundary.md)、[反馈渠道](SUPPORT.md)、[安全](SECURITY.md)、[更新日志](CHANGELOG.md) |
