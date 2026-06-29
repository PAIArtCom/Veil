export default {
  meta: {
    title: 'Veil · AI 编程助手密钥脱敏代理 | 防止 Claude Code / Codex 泄露 API Key',
    description:
      'Veil 是开源的大模型脱敏代理，拦截 Claude Code、Codex CLI 发出的 LLM prompt，在 API key 离开本机前完成格式保留脱敏，失败即拦截，无云端中继。',
    defaultDescription:
      '你的 AI 编程助手每次请求都在把 API key 和 PII 上传大模型云端。Veil 是本地脱敏代理，拦住数据防泄漏。',
  },

  header: {
    features: '功能',
    trust: '信任',
    howItWorks: '工作原理',
    install: '安装',
  },

  a11y: {
    skipToContent: '跳到主要内容',
    menu: '菜单',
    mainNav: '主导航',
    footerNav: '页脚导航',
    switchLanguage: '切换语言',
  },

  hero: {
    badge: '开源 · AI 编程助手密钥脱敏代理',
    title: '你的 AI 助手正在读取',
    titleAccent: '你的密钥。',
    sub: '你发出的每一条 LLM prompt，都带着 API key、密码和数据库连接串直奔大模型云端。Veil 是本地脱敏代理，在数据离开本机前完成 PII 脱敏——大模型只看到格式保留的安全占位符。',
    cta: '下载 Veil',
    github: '在 GitHub 上查看',
    proofLocal: '运行在 127.0.0.1',
    proofNoCloud: '无云端中继',
    proofLicense: 'Apache-2.0',
  },

  features: {
    eyebrow: '功能',
    title: '密钥绝不以明文',
    titleAccent: '进入模型',
    sub: '用 Claude Code 或 Codex CLI 的你，每次请求都在把 env 变量、连接串和 API key 送进大模型。Veil 作为本地脱敏代理拦截这些数据——下面是它的处理方式。',
    link: '开始使用',
    cards: [
      {
        problem: '被粘进 prompt 的密钥',
        title: '密钥自动拦截脱敏',
        description:
          'API key、密码、数据库连接串、邮箱、IP 地址——Veil 在请求离开 localhost 之前全部检测并脱敏。',
      },
      {
        problem: '多轮对话中反复出现的密钥',
        title: '同一个值，同一个安全 token',
        description:
          'Veil 对每个密钥做确定性 token 化——同一个值始终映射到同一占位符，大模型可以跨轮次推理引用关系，但永远接触不到真实值。',
      },
      {
        problem: '识别不了的请求格式',
        title: '失败即拦截，绝不静默泄露',
        description:
          'Veil 认不出的请求格式，一律失败即拦截（fail closed）——不会静默放行，不会把明文 prompt 或 API key 转发出去。',
      },
      {
        problem: '又一个要维护的服务',
        title: '只是一个本地进程',
        description:
          '不用注册，没有控制台，没有云端。Veil 作为本地脱敏代理运行在 127.0.0.1，只改写请求和响应正文中的敏感数据。你的 API key 凭据原样透传给上游。',
      },
    ],
  },

  trust: {
    eyebrow: '凭什么信任 Veil',
    title: '任何东西离开你的机器之前，',
    titleAccent: '都已脱敏。',
    titleEnd: '',
    sub: '出站完成大模型数据防泄漏，入站自动还原真实值。没有黑魔法，没有云端，除了 localhost 不需要信任任何人。',
    ctaThreat: '阅读威胁模型',
    ctaArch: '审查架构',
    cards: [
      {
        id: 'shield',
        title: '100% 本地',
        description:
          'Veil 只绑定 127.0.0.1。没有云端中继，没有远程服务器，你和 API 提供商之间只有一个本地进程。',
      },
      {
        id: 'lock',
        title: '失败即拦截（fail closed）',
        description:
          '无法识别的请求格式、解析出错的 prompt、不支持的端点——Veil 一律拦截，绝不把明文数据转发给大模型。',
      },
      {
        id: 'arrows',
        title: 'API key 始终是你的',
        description:
          'Veil 不存储、不接触你的 API 凭据。它只改写请求和响应的正文内容。',
      },
      {
        id: 'eye',
        title: '自己看源码',
        description:
          'Apache-2.0 许可。每一行都可审计。看完威胁模型和发布产物，再决定信不信。',
      },
    ],
  },

  howItWorks: {
    eyebrow: '工作原理',
    title: '把你的助手指向',
    titleAccent: '本地 proxy。',
    titleEnd: '',
    sub: '没有控制台，无需注册。改一个环境变量，让 Claude Code 或 Codex CLI 经由本地脱敏代理发起请求——你的工具和工作流完全不变。',
    link: '启动 proxy',
    steps: [
      {
        title: '启动 Veil',
        description:
          '一条命令在 localhost 启动 proxy。这就是你的隐私边界。',
      },
      {
        title: '配置你的助手',
        description:
          '把 Claude Code 或 Codex CLI 的 base URL 改一下。一个环境变量的事。',
      },
      {
        title: '继续写代码',
        description:
          '其他什么都不用变。你的凭据、工作流、工具链——全部照旧。Veil 只在传输过程中脱敏内容。',
      },
    ],
  },

  install: {
    eyebrow: '开始使用',
    title: '一条命令',
    titleAccent: '即可上手。',
    titleEnd: '',
    sub: '选择你的平台，二进制几秒内就到位。然后把你的 AI 助手指向代理即可。',
    link: '全部版本',
    curl: { label: 'macOS 和 Linux', hint: 'curl — 无需其他依赖' },
    npm: { label: 'npm', hint: 'macOS · Linux · Windows 通用' },
    brew: { label: 'Homebrew', hint: 'tap PAIArtCom/veil' },
    winps: { label: 'Windows', hint: 'PowerShell — 自动加入 PATH' },
    agentsLabel: '然后接入你的 AI 助手',
    sourceLink: '更多方式（go install、源码编译）',
    claudeCode: {
      title: 'Claude Code',
      guide: '查看指南',
      description:
        '启动 Veil 本地代理，export 一个变量，打开 Claude Code——Claude Code 密钥脱敏即刻生效。',
    },
    codex: {
      title: 'Codex CLI',
      guide: '查看指南',
      description:
        '用 OpenAI 上游启动 Veil 本地代理，把 Codex CLI 的请求指向 127.0.0.1——API key 自动脱敏，无需改动代码。',
    },
    copied: '已复制！',
  },

  boundary: {
    eyebrow: '兼容性',
    title: '现在能用什么，',
    titleAccent: '接下来做什么。',
    titleEnd: '',
    sub: 'Veil 对脱敏覆盖范围实话实说——当前支持 Claude Code 和 Codex CLI 的大模型网关流量。保护不了的格式，要么失败即拦截，要么明确告诉你。',
    ctaDownload: '下载最新版本',
    ctaTypes: '查看受保护类型',
    supportedTitle: '现已支持 (v0.1.0)',
    notYetTitle: '即将支持',
    supported: [
      'Claude Code (Anthropic Messages)',
      'Codex CLI (OpenAI Responses)',
      'Go SDK 集成',
      '已支持格式中的文本和工具调用字段',
    ],
    notYet: [
      'OpenAI Chat Completions',
      'Gemini',
      'OCR、附件、文档解析',
      '远程 MCP 工具流量',
    ],
  },

  coverage: {
    eyebrow: '覆盖范围',
    title: 'Veil 能识别并脱敏的数据',
    sub: 'Veil 识别 LLM prompt 和工具调用中的 PII 及凭据，在数据离开本机前完成格式保留脱敏——每一项都替换为确定性、可还原的安全占位符。',
    optIn: '可选开启',
    types: [
      { label: '密钥', example: 'API key · token · 密码 · DSN' },
      { label: '邮箱', example: 'user@example.com' },
      { label: '电话', example: '+1 555 123 4567' },
      { label: 'IP 地址', example: '192.168.1.1 · 2001:db8::1' },
      { label: '银行卡', example: '4111 1111 1111 1111' },
      { label: '账号', example: '银行与金融标识' },
      { label: 'URL', example: 'https://internal.corp/api' },
      { label: '日期', example: '默认关闭', optIn: true },
      { label: '姓名与地址', example: '可选的语义检测', optIn: true },
    ],
  },

  security: {
    eyebrow: '安全模型',
    title: '精确的安全保证，而非模糊承诺',
    sub: 'Veil 小巧、本地、可审计。下面是它确切会做、以及不会做的事。',
    items: [
      {
        title: '100% 本地运行',
        desc: '绑定到 127.0.0.1。没有云端中继、没有远程服务器，Veil 不存储你的任何 API key 或 PII 数据。',
      },
      {
        title: '失败即拦截',
        desc: '解析错误、检测错误、策略冲突或不支持的端点——一律失败即拦截（fail closed），而不是把明文 prompt 或 API key 泄露给上游大模型。',
      },
      {
        title: '确定性 token 化',
        desc: '同一个值在同一作用域内做确定性 token 化，始终映射到同一占位符。多轮上下文和 LLM prompt 缓存在脱敏后依然有效，不影响大模型推理。',
      },
      {
        title: '本地可逆',
        desc: '模型只看到占位符；你的终端、文件和工具调用拿回的是真实值。',
      },
    ],
  },

  faq: {
    eyebrow: 'FAQ',
    title: '常见问题',
    items: [
      {
        q: 'Veil 会增加延迟吗？',
        a: '它运行在 localhost，只重写请求和响应的 body，开销仅是一次本地跳转——相比到提供商的网络往返可以忽略不计。',
      },
      {
        q: '脱敏会影响大模型的输出质量吗？',
        a: '不会。占位符是确定性且保留格式的，模型基于稳定、规范的值进行推理。Veil 会在响应里先还原真实值，再交给你的工具。',
      },
      {
        q: 'Veil 会看到我的 API key 吗？',
        a: 'Veil 从不存储或接触你的提供商凭据。它只重写请求和响应 body 中的内容，你的 API key 原样透传。',
      },
      {
        q: '支持哪些 AI 编程助手和大模型客户端？',
        a: 'v0.1.0 支持 Claude Code（Anthropic Messages API）和 Codex CLI（OpenAI Responses API），以及 Go SDK 集成。这两个 AI 编程助手的 API key 脱敏和 PII 脱敏均已覆盖。OpenAI Chat Completions、Gemini 等大模型客户端在路线图上。',
      },
      {
        q: '如何移除？',
        a: '取消那个环境变量即可。Veil 只是一个本地脱敏代理进程——没有要卸载的账号、后台 agent 或守护进程，不留任何痕迹。',
      },
    ],
  },

  footer: {
    copyright: 'Veil 贡献者。Apache-2.0。',
    github: 'GitHub',
    releases: '版本发布',
    security: '安全',
  },

  notFound: {
    code: '404',
    title: '页面未找到',
    message: '你访问的页面不存在。',
    back: '返回首页',
  },
} as const;
