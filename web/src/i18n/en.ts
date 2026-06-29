export default {
  meta: {
    title: 'Stop leaking secrets to AI providers',
    description:
      'Veil is a local de-identification proxy that masks secrets and PII before Claude Code or Codex traffic leaves your machine.',
    defaultDescription:
      'Local de-identification for AI coding agents.',
  },

  header: {
    features: 'Features',
    trust: 'Trust',
    howItWorks: 'How it works',
    install: 'Install',
  },

  a11y: {
    skipToContent: 'Skip to content',
    menu: 'Menu',
    mainNav: 'Main navigation',
    footerNav: 'Footer navigation',
    switchLanguage: 'Switch language',
  },

  hero: {
    badge: 'Open source · Secret masking for AI coding agents',
    title: 'Your AI agent reads',
    titleAccent: 'your secrets.',
    sub: 'Every prompt you send carries API keys, passwords, and database URLs straight to the cloud. Veil masks them before they leave your machine.',
    cta: 'Download Veil',
    github: 'View on GitHub',
    proofLocal: 'Runs on 127.0.0.1',
    proofNoCloud: 'No cloud relay',
    proofLicense: 'Apache-2.0',
  },

  features: {
    eyebrow: 'Capabilities',
    title: 'Secrets never reach the model',
    titleAccent: 'in the clear',
    sub: 'If you use Claude Code or Codex, your env vars, connection strings, and API keys are going to the cloud on every request. Here\'s how Veil handles that.',
    link: 'Get started',
    cards: [
      {
        problem: 'Secrets pasted into prompts',
        title: 'Secrets get caught automatically',
        description:
          'API keys, passwords, database URLs, emails, IP addresses — Veil detects and masks them before the request leaves localhost.',
      },
      {
        problem: 'Secrets reused across turns',
        title: 'Same value, same safe token',
        description:
          'Veil maps each secret to a stable placeholder, so the model can reason across turns without ever seeing the real value.',
      },
      {
        problem: 'Edge-case request formats',
        title: 'Unknown formats get blocked',
        description:
          'If Veil doesn\'t recognize a request format, it blocks it. No silent passthrough, no plaintext leaks.',
      },
      {
        problem: 'Yet another service to run',
        title: 'It\'s just a local process',
        description:
          'No account, no dashboard, no cloud. Veil runs on 127.0.0.1 and only touches the request and response body. Your API keys pass through untouched.',
      },
    ],
  },

  trust: {
    eyebrow: 'Why trust Veil',
    title: 'Nothing leaves your machine',
    titleAccent: 'unmasked.',
    titleEnd: '',
    sub: 'Secrets get masked before they go out, and restored when responses come back. That\'s it. No magic, no cloud, no trust required beyond localhost.',
    ctaThreat: 'Read the threat model',
    ctaArch: 'Review architecture',
    cards: [
      {
        id: 'shield',
        title: '100% local',
        description:
          'Veil binds to 127.0.0.1. There is no cloud relay, no remote server, nothing between you and your provider except a local process.',
      },
      {
        id: 'lock',
        title: 'Blocks what it can\'t parse',
        description:
          'Unrecognized request formats never get forwarded. If Veil isn\'t sure, it stops the request.',
      },
      {
        id: 'arrows',
        title: 'Your API keys stay yours',
        description:
          'Veil never stores or touches your provider credentials. It only rewrites request and response content.',
      },
      {
        id: 'eye',
        title: 'Read the source yourself',
        description:
          'Apache-2.0. Every line is auditable. Check the threat model and release artifacts before you trust it.',
      },
    ],
  },

  howItWorks: {
    eyebrow: 'How it works',
    title: 'Point your agent at',
    titleAccent: 'a local proxy.',
    titleEnd: '',
    sub: 'No dashboard, no account. Change one environment variable to route your agent through localhost — your tools and workflow stay exactly the same.',
    link: 'Run the proxy',
    steps: [
      {
        title: 'Start Veil',
        description:
          'One command starts the proxy on localhost. That\'s your new privacy boundary.',
      },
      {
        title: 'Point your agent',
        description:
          'Change the base URL for Claude Code or Codex. One environment variable.',
      },
      {
        title: 'Keep working',
        description:
          'Nothing else changes. Your credentials, your workflow, your tools — all the same. Veil just masks the content in transit.',
      },
    ],
  },

  install: {
    eyebrow: 'Get started',
    title: 'Install and run in',
    titleAccent: 'one command.',
    titleEnd: '',
    sub: 'Pick your platform — the binary lands on your machine in seconds. Then point your agent at the proxy.',
    link: 'All releases',
    curl: { label: 'macOS & Linux', hint: 'curl — no dependencies' },
    npm: { label: 'npm', hint: 'macOS · Linux · Windows' },
    brew: { label: 'Homebrew', hint: 'tap PAIArtCom/veil' },
    winps: { label: 'Windows', hint: 'PowerShell — auto-adds to PATH' },
    agentsLabel: 'Then wire up your agent',
    sourceLink: 'More options (go install, build from source)',
    claudeCode: {
      title: 'Claude Code',
      guide: 'View guide',
      description:
        'Start Veil, export one variable, launch Claude.',
    },
    codex: {
      title: 'Codex CLI',
      guide: 'View guide',
      description:
        'Start Veil with the OpenAI upstream, point Codex at it.',
    },
    copied: 'Copied!',
  },

  boundary: {
    eyebrow: 'Compatibility',
    title: 'What works today,',
    titleAccent: 'what\'s next.',
    titleEnd: '',
    sub: 'Veil is honest about its coverage. If it can\'t protect a format, it says so — or blocks it.',
    ctaDownload: 'Download latest release',
    ctaTypes: 'See protected types',
    supportedTitle: 'Works now (v0.1.0)',
    notYetTitle: 'Coming soon',
    supported: [
      'Claude Code (Anthropic Messages)',
      'Codex CLI (OpenAI Responses)',
      'Go SDK integrations',
      'Text and tool-use fields in supported formats',
    ],
    notYet: [
      'OpenAI Chat Completions',
      'Gemini',
      'OCR, attachments, document parsing',
      'Remote MCP tool traffic',
    ],
  },

  coverage: {
    eyebrow: 'Coverage',
    title: 'What Veil detects and masks',
    sub: 'Veil recognizes the sensitive data that shows up in real prompts and tool calls, and replaces each with a format-preserving placeholder before it leaves your machine.',
    optIn: 'Opt-in',
    types: [
      { label: 'Secrets', example: 'API keys · tokens · passwords · DSNs' },
      { label: 'Email', example: 'user@example.com' },
      { label: 'Phone', example: '+1 555 123 4567' },
      { label: 'IP addresses', example: '192.168.1.1 · 2001:db8::1' },
      { label: 'Payment cards', example: '4111 1111 1111 1111' },
      { label: 'Account numbers', example: 'Bank & financial IDs' },
      { label: 'URLs', example: 'https://internal.corp/api' },
      { label: 'Dates', example: 'Off by default', optIn: true },
      { label: 'Names & addresses', example: 'Opt-in semantic detection', optIn: true },
    ],
  },

  security: {
    eyebrow: 'Security model',
    title: 'Precise guarantees, not promises',
    sub: 'Veil is small, local, and auditable. Here is exactly what it does — and does not — do.',
    items: [
      { title: 'Local only', desc: 'Binds to 127.0.0.1. No relay, no remote server, and Veil stores none of your credentials.' },
      { title: 'Fail closed', desc: 'Parsing errors, detection errors, policy violations, or unsupported endpoints block the request rather than forwarding plaintext.' },
      { title: 'Deterministic tokens', desc: 'The same value maps to the same placeholder within a scope, so multi-turn context and prompt caching survive masking.' },
      { title: 'Reversible locally', desc: 'The provider sees placeholders; your terminal, files, and tool calls get the real values back.' },
    ],
  },

  faq: {
    eyebrow: 'FAQ',
    title: 'Common questions',
    items: [
      { q: 'Does Veil add latency?', a: 'It runs on localhost and only rewrites the request and response body, so the overhead is a single local hop — negligible next to the network round-trip to your provider.' },
      { q: 'Will it change the model’s output?', a: 'No. Placeholders are deterministic and format-preserving, so the model reasons over stable, well-formed values. Veil restores the real values in the response before your tools see them.' },
      { q: 'Does Veil see my API keys?', a: 'Veil never stores or touches your provider credentials. It only rewrites content in the request and response body; your API keys pass through untouched.' },
      { q: 'Which agents are supported?', a: 'Claude Code (Anthropic Messages) and Codex CLI (OpenAI Responses) in v0.1.0, plus Go SDK integrations. OpenAI Chat Completions, Gemini, and more are on the roadmap.' },
      { q: 'How do I remove it?', a: 'Unset the environment variable. Veil is just a local process — there is no account, agent, or daemon to uninstall.' },
    ],
  },

  footer: {
    copyright: 'Veil contributors. Apache-2.0.',
    github: 'GitHub',
    releases: 'Releases',
    security: 'Security',
  },

  notFound: {
    code: '404',
    title: 'Page not found',
    message: "The page you're looking for doesn't exist.",
    back: 'Back to home',
  },
} as const;
