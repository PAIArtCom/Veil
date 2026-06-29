export default {
  meta: {
    title: 'Veil — AIエージェントのAPIキー漏洩を防ぐローカルプロキシ',
    description:
      'Claude CodeやCodex CLIのプロンプトに含まれるAPIキー・シークレットを、クラウドへ送信する前にローカルでマスク。fail-closedな設計でLLMへの情報漏洩を防ぐオープンソースプロキシ。',
    defaultDescription:
      'AIコーディングエージェントがAPIキーや認証情報をLLMへ送信するたびに情報漏洩リスクがあります。Veilはローカルプロキシでシークレットをマスクしてからクラウドへ転送します。',
  },

  header: {
    features: '機能',
    trust: '信頼性',
    howItWorks: '仕組み',
    install: 'インストール',
  },

  a11y: {
    skipToContent: 'コンテンツにスキップ',
    menu: 'メニュー',
    mainNav: 'メインナビゲーション',
    footerNav: 'フッターナビゲーション',
    switchLanguage: '言語を切り替える',
  },

  hero: {
    badge: 'オープンソース · AIコーディングエージェント向けAPIキー漏洩対策',
    title: 'AIエージェントが送っている、',
    titleAccent: 'あなたのAPIキーを。',
    sub: 'Claude CodeやCodex CLIがプロンプトを送るたびに、APIキー・パスワード・データベースURLがそのままLLMへ渡っています。Veilはローカルプロキシとして動作し、マシンの外に出る前にシークレットをマスクします。',
    cta: 'Veilをダウンロード',
    github: 'GitHubで見る',
    proofLocal: '127.0.0.1で動作',
    proofNoCloud: 'クラウド中継なし',
    proofLicense: 'Apache-2.0',
  },

  features: {
    eyebrow: '機能',
    title: 'APIキーとシークレットは',
    titleAccent: 'LLMに渡さない',
    sub: 'Claude CodeやCodex CLIを使っていれば、環境変数・接続文字列・APIキーがリクエストのたびにLLMプロバイダーへ送られています。これがLLMプロンプト情報漏洩のリスクです。Veilはこう対処します。',
    link: 'はじめる',
    cards: [
      {
        problem: 'プロンプトに含まれるAPIキーやシークレット',
        title: 'シークレットを自動検出してマスク',
        description:
          'APIキー、パスワード、データベースURL、メールアドレス、IPアドレス。リクエストがlocalhostを出る前にVeilが検出してマスクします。',
      },
      {
        problem: 'ターンをまたいで再利用されるシークレット',
        title: '決定論的トークンで一貫したマスキング',
        description:
          '同じシークレットには常に同じプレースホルダーを割り当てます。マルチターン会話でもプロンプトキャッシュでも、LLMは一貫した値で推論できます。',
      },
      {
        problem: '想定外のリクエスト形式',
        title: 'fail-closedで未知の形式はブロック',
        description:
          'Veilが認識できないリクエスト形式は転送しません。サイレントに素通りすることも、平文で漏れることもありません。',
      },
      {
        problem: '管理対象が増えるという負担',
        title: 'ただのローカルプロセス',
        description:
          'アカウント不要、ダッシュボードなし、クラウドなし。Veilは127.0.0.1で動き、リクエストとレスポンスの本文だけを処理します。APIキーはそのまま通過します。',
      },
    ],
  },

  trust: {
    eyebrow: 'Veilを信頼できる理由',
    title: 'マスクされていないシークレットは',
    titleAccent: '外に出ない。',
    titleEnd: '',
    sub: '送信前にシークレットをマスクし、レスポンスが返ったらローカルで復元する。それだけです。クラウド中継なし、余計なサービスなし、localhost以外への信頼は不要。',
    ctaThreat: '脅威モデルを読む',
    ctaArch: 'アーキテクチャを確認',
    cards: [
      {
        id: 'shield',
        title: '100%ローカル',
        description:
          'Veilは127.0.0.1にバインドします。クラウド中継もリモートサーバーもなし。あなたとプロバイダーの間にあるのはローカルプロセスだけです。',
      },
      {
        id: 'lock',
        title: '解析できなければfail-closed',
        description:
          '認識できないリクエスト形式は転送されません。Veilが確信を持てなければ、リクエストを停止します。',
      },
      {
        id: 'arrows',
        title: 'APIキーはあなたの手元に',
        description:
          'Veilはプロバイダーの認証情報を保存も操作もしません。書き換えるのはリクエストとレスポンスの本文だけです。',
      },
      {
        id: 'eye',
        title: 'ソースを自分の目で確認',
        description:
          'Apache-2.0。すべてのコードが監査可能です。信頼する前に、脅威モデルとリリース成果物をご自身で確認してください。',
      },
    ],
  },

  howItWorks: {
    eyebrow: '仕組み',
    title: 'エージェントを',
    titleAccent: 'ローカルプロキシに向ける',
    titleEnd: '',
    sub: 'アカウントもダッシュボードも不要。Claude CodeまたはCodex CLIの接続先を環境変数ひとつでlocalhostに向けるだけ。APIキーのマスキングはVeilが自動で行います。ツールもワークフローもそのままです。',
    link: 'プロキシを実行',
    steps: [
      {
        title: 'Veilを起動',
        description:
          'コマンドひとつでlocalhostにローカルプロキシが起動します。ここがAPIキー漏洩対策の境界線です。',
      },
      {
        title: 'エージェントを向ける',
        description:
          'Claude CodeまたはCodex CLIのベースURLを変更するだけ。環境変数ひとつです。',
      },
      {
        title: 'あとはいつも通り',
        description:
          'それ以外は何も変わりません。認証情報もワークフローもツールもそのまま。Veilは通信の中身をマスクするだけです。',
      },
    ],
  },

  install: {
    eyebrow: 'はじめよう',
    title: '',
    titleAccent: 'コマンドひとつ',
    titleEnd: 'で試せる。',
    sub: 'バイナリをダウンロードして環境変数をひとつ設定するだけ。AIエージェントのAPIキー漏洩対策がすぐに始まります。元に戻したいときは変数を解除するだけです。',
    link: 'リリースをダウンロード',
    claudeCode: {
      title: 'Claude Code',
      guide: 'ガイドを見る',
      description:
        'Veilを起動し、変数をひとつ設定して、Claude Codeを起動。APIキーのシークレットマスキングが有効になります。',
    },
    codex: {
      title: 'Codex CLI',
      guide: 'ガイドを見る',
      description:
        'OpenAIをアップストリームにVeilを起動し、Codex CLIを向けるだけ。APIキーのLLMへの漏洩を防ぎます。',
    },
    source: {
      title: 'ソースからビルド',
      badge: 'Go 1.22+が必要',
      description: '信頼する前に、すべての行を読めます。',
    },
    copied: 'コピーしました！',
  },

  boundary: {
    eyebrow: '互換性',
    title: '今できること、',
    titleAccent: 'これから対応すること。',
    titleEnd: '',
    sub: 'Veilはカバレッジを正直に示します。Claude CodeとCodex CLIのリクエストは保護対象です。対応できない形式は明示するか、fail-closedでブロックします。',
    ctaDownload: '最新リリースをダウンロード',
    ctaTypes: '保護対象の型を確認',
    supportedTitle: '対応済み（v0.1.0）',
    notYetTitle: '近日対応予定',
    supported: [
      'Claude Code（Anthropic Messages）',
      'Codex CLI（OpenAI Responses）',
      'Go SDKインテグレーション',
      'サポート対象形式のテキスト・tool-useフィールド',
    ],
    notYet: [
      'OpenAI Chat Completions',
      'Gemini',
      'OCR・添付ファイル・ドキュメント解析',
      'リモートMCPツールトラフィック',
    ],
  },

  coverage: {
    eyebrow: '対応範囲',
    title: 'Veilが検出・マスキングする対象',
    sub: 'Veilはプロンプトやツール呼び出しに含まれるAPIキー・シークレット・個人情報を認識し、LLMへ送信する前にフォーマット保持型のプレースホルダーへ置き換えます。',
    optIn: 'オプトイン',
    types: [
      { label: 'シークレット', example: 'APIキー · トークン · パスワード · DSN' },
      { label: 'メール', example: 'user@example.com' },
      { label: '電話番号', example: '+1 555 123 4567' },
      { label: 'IPアドレス', example: '192.168.1.1 · 2001:db8::1' },
      { label: 'クレジットカード', example: '4111 1111 1111 1111' },
      { label: '口座番号', example: '銀行・金融のID' },
      { label: 'URL', example: 'https://internal.corp/api' },
      { label: '日付', example: 'デフォルトでは無効', optIn: true },
      { label: '氏名・住所', example: 'オプトインの意味解析', optIn: true },
    ],
  },

  security: {
    eyebrow: 'セキュリティモデル',
    title: '約束ではなく、明確な保証',
    sub: 'Veilは小さく、ローカルで、監査可能です。何をして、何をしないかを正確に示します。',
    items: [
      {
        title: 'ローカルのみ',
        desc: '127.0.0.1にバインドします。リレーもリモートサーバーもなく、Veilは認証情報を一切保存しません。',
      },
      {
        title: 'fail-closed（フェイルクローズド）',
        desc: '解析エラー、検出エラー、ポリシー違反、非対応エンドポイントは、平文を転送せずリクエストをブロックします。',
      },
      {
        title: '決定論的トークン化',
        desc: '同じ値はスコープ内で同じプレースホルダーに対応するため、マルチターンの文脈やプロンプトキャッシュがマスキング後も保たれます。',
      },
      {
        title: 'ローカルで復元',
        desc: 'プロバイダーが見るのはプレースホルダーのみ。あなたのターミナル・ファイル・ツール呼び出しには実際の値が戻ります。',
      },
    ],
  },

  faq: {
    eyebrow: 'FAQ',
    title: 'よくある質問',
    items: [
      {
        q: 'Veilを使うとAPIのレイテンシは増えますか？',
        a: 'localhostで動作し、リクエストとレスポンスのボディを書き換えるだけなので、オーバーヘッドはローカルの1ホップ分のみ。プロバイダーまでのネットワーク往復に比べれば無視できる程度です。',
      },
      {
        q: 'シークレットをマスクするとLLMの出力品質は変わりますか？',
        a: '変わりません。Veilのプレースホルダーは決定論的かつフォーマット保持型なので、LLMは整合性のある値で推論できます。レスポンス内のプレースホルダーはローカルで元の値に復元されてからツールに渡されるため、ワークフローへの影響はありません。',
      },
      {
        q: 'VeilはプロバイダーのAPIキーを保存・参照しますか？',
        a: 'Veilはプロバイダーの認証情報を保存も参照もしません。リクエストとレスポンスのボディの内容のみを書き換え、APIキーはそのまま通過します。',
      },
      {
        q: 'どのAIコーディングエージェントに対応していますか？',
        a: 'v0.1.0ではClaude Code（Anthropic Messages）とCodex CLI（OpenAI Responses）、さらにGo SDK連携に対応。OpenAI Chat CompletionsやGeminiなどはロードマップにあります。',
      },
      {
        q: 'Veilをアンインストールするには？',
        a: '環境変数を解除するだけです。Veilは単なるローカルプロセスで、アンインストールが必要なアカウント・エージェント・デーモンはありません。',
      },
    ],
  },

  footer: {
    copyright: 'Veil 貢献者。Apache-2.0。',
    github: 'GitHub',
    releases: 'リリース',
    security: 'セキュリティ',
  },

  notFound: {
    code: '404',
    title: 'ページが見つかりません',
    message: 'お探しのページは存在しません。',
    back: 'ホームに戻る',
  },
} as const;
