export default {
  meta: {
    title: 'Enmascara tus API keys antes de que tu agente IA las filtre',
    description:
      'Veil es un proxy local que enmascara API keys, tokens y secretos antes de que Claude Code, Codex CLI u otro agente IA los envíen a la nube. 100% local, sin relay, open source.',
    defaultDescription:
      'Tu agente IA filtra API keys y secretos a la nube. Veil los enmascara antes de que salgan de tu máquina.',
  },

  header: {
    features: 'Funciones',
    trust: 'Confianza',
    howItWorks: 'Cómo funciona',
    install: 'Instalar',
  },

  a11y: {
    skipToContent: 'Saltar al contenido',
    menu: 'Menú',
    mainNav: 'Navegación principal',
    footerNav: 'Navegación del pie de página',
    switchLanguage: 'Cambiar idioma',
  },

  hero: {
    badge: 'Open source · Proxy local para enmascarar secretos de agentes IA',
    title: 'Tu agente de IA lee',
    titleAccent: 'tus secretos.',
    sub: 'Cada prompt que envías lleva API keys, contraseñas y URLs de bases de datos directo a la nube. Veil enmascara esos secretos antes de que salgan de tu máquina.',
    cta: 'Descargar Veil',
    github: 'Ver en GitHub',
    proofLocal: 'Solo en 127.0.0.1',
    proofNoCloud: 'Sin relay en la nube',
    proofLicense: 'Apache-2.0',
  },

  features: {
    eyebrow: 'Capacidades',
    title: 'Los secretos nunca llegan al modelo',
    titleAccent: 'en texto plano',
    sub: 'Si usas Claude Code o Codex, tus variables de entorno, cadenas de conexión y API keys viajan a la nube en cada request. Así es como Veil lo resuelve.',
    link: 'Empezar',
    cards: [
      {
        problem: 'Secretos pegados en los prompts',
        title: 'Los secretos se detectan automáticamente',
        description:
          'API keys, contraseñas, URLs de bases de datos, emails, direcciones IP — Veil los detecta y enmascara antes de que el request salga de localhost.',
      },
      {
        problem: 'Secretos reutilizados entre turnos',
        title: 'Mismo valor, mismo token seguro',
        description:
          'Veil asigna a cada secreto el mismo placeholder determinista, así el modelo puede razonar entre turnos sin ver nunca el valor real. Enmascaramiento reversible en local.',
      },
      {
        problem: 'Requests con formato desconocido',
        title: 'Los formatos desconocidos se bloquean',
        description:
          'Si Veil no reconoce el formato de un request, lo bloquea. Sin reenvíos silenciosos, sin fugas en texto plano.',
      },
      {
        problem: 'Otro servicio más que mantener',
        title: 'Es solo un proceso local',
        description:
          'Sin cuenta, sin dashboard, sin nube. Veil corre en 127.0.0.1 y solo reescribe el cuerpo del request y el response. Tus API keys pasan intactas.',
      },
    ],
  },

  trust: {
    eyebrow: 'Por qué confiar en Veil',
    title: 'Nada sale de tu máquina',
    titleAccent: 'sin enmascarar.',
    titleEnd: '',
    sub: 'Los secretos se enmascaran antes de salir y se restauran en local cuando llega el response. Sin magia, sin nube, sin necesidad de confiar en nada más allá de localhost.',
    ctaThreat: 'Leer el modelo de amenazas',
    ctaArch: 'Revisar la arquitectura',
    cards: [
      {
        id: 'shield',
        title: '100% local',
        description:
          'Veil escucha en 127.0.0.1. No hay relay en la nube, no hay servidor remoto, nada entre tú y tu proveedor excepto un proceso local.',
      },
      {
        id: 'lock',
        title: 'Bloquea lo que no puede analizar',
        description:
          'Los formatos de request no reconocidos nunca se reenvían. Si Veil no está seguro, detiene el request.',
      },
      {
        id: 'arrows',
        title: 'Tus API keys siguen siendo tuyas',
        description:
          'Veil nunca almacena ni accede a las credenciales de tu proveedor. Solo reescribe el cuerpo del request y del response para enmascarar secretos.',
      },
      {
        id: 'eye',
        title: 'Lee el código tú mismo',
        description:
          'Apache-2.0. Cada línea es auditable. Revisa el modelo de amenazas y los artefactos de cada release antes de confiar en él.',
      },
    ],
  },

  howItWorks: {
    eyebrow: 'Cómo funciona',
    title: 'Apunta tu agente a',
    titleAccent: 'un proxy local.',
    titleEnd: '',
    sub: 'Sin dashboard, sin cuenta. Cambia una variable de entorno para enrutar tu agente por localhost: tus herramientas y tu flujo de trabajo no cambian.',
    link: 'Ejecutar el proxy',
    steps: [
      {
        title: 'Inicia Veil',
        description:
          'Un comando levanta el proxy en localhost. Ese es tu nuevo límite de privacidad.',
      },
      {
        title: 'Apunta tu agente',
        description:
          'Cambia la URL base de Claude Code o Codex. Una variable de entorno.',
      },
      {
        title: 'Sigue trabajando',
        description:
          'Nada más cambia. Tus credenciales, tu flujo de trabajo, tus herramientas — todo igual. Veil solo enmascara los secretos en tránsito.',
      },
    ],
  },

  install: {
    eyebrow: 'Empezar',
    title: 'Instala y ejecuta con',
    titleAccent: 'un comando.',
    titleEnd: '',
    sub: 'Descarga el binario, configura una variable de entorno, listo. Para quitarlo, solo elimina la variable.',
    link: 'Descargar release',
    claudeCode: {
      title: 'Claude Code',
      guide: 'Ver guía',
      description: 'Inicia Veil, exporta una variable, lanza Claude Code.',
    },
    codex: {
      title: 'Codex CLI',
      guide: 'Ver guía',
      description:
        'Inicia Veil con el upstream de OpenAI y apunta Codex hacia él.',
    },
    source: {
      title: 'Compilar desde el código fuente',
      badge: 'Requiere Go 1.22+',
      description: 'Lee cada línea antes de confiar en él.',
    },
    copied: '¡Copiado!',
  },

  boundary: {
    eyebrow: 'Compatibilidad',
    title: 'Qué funciona hoy,',
    titleAccent: 'qué viene después.',
    titleEnd: '',
    sub: 'Veil es transparente con su cobertura. Si no puede enmascarar un formato de request, lo dice — o lo bloquea con fail-closed.',
    ctaDownload: 'Descargar última versión',
    ctaTypes: 'Ver tipos protegidos',
    supportedTitle: 'Funciona ahora (v0.1.0)',
    notYetTitle: 'Próximamente',
    supported: [
      'Claude Code (Anthropic Messages)',
      'Codex CLI (OpenAI Responses)',
      'Integraciones con Go SDK',
      'Campos de texto y tool-use en formatos soportados',
    ],
    notYet: [
      'OpenAI Chat Completions',
      'Gemini',
      'OCR, adjuntos, parsing de documentos',
      'Tráfico remoto de herramientas MCP',
    ],
  },

  coverage: {
    eyebrow: 'Cobertura',
    title: 'Qué detecta y enmascara Veil',
    sub: 'Veil detecta los datos sensibles que aparecen en prompts y tool calls reales, y reemplaza cada uno por un placeholder que preserva el formato antes de que el request salga de tu máquina.',
    optIn: 'Opcional',
    types: [
      { label: 'Secretos', example: 'API keys · tokens · contraseñas · DSN' },
      { label: 'Email', example: 'user@example.com' },
      { label: 'Teléfono', example: '+1 555 123 4567' },
      { label: 'Direcciones IP', example: '192.168.1.1 · 2001:db8::1' },
      { label: 'Tarjetas de pago', example: '4111 1111 1111 1111' },
      { label: 'Números de cuenta', example: 'IDs bancarios y financieros' },
      { label: 'URLs', example: 'https://internal.corp/api' },
      { label: 'Fechas', example: 'Desactivado por defecto', optIn: true },
      {
        label: 'Nombres y direcciones',
        example: 'Detección semántica opcional',
        optIn: true,
      },
    ],
  },

  security: {
    eyebrow: 'Modelo de seguridad',
    title: 'Garantías precisas, no promesas',
    sub: 'Veil es pequeño, local y auditable. Esto es exactamente lo que hace, y lo que no.',
    items: [
      {
        title: 'Solo local',
        desc: 'Se enlaza a 127.0.0.1. Sin relay, sin servidor remoto, y Veil no almacena ninguna de tus credenciales.',
      },
      {
        title: 'Fail-closed',
        desc: 'Errores de parsing, violaciones de política o endpoints no soportados bloquean el request en lugar de reenviarlo en texto plano. Sin filtraciones silenciosas.',
      },
      {
        title: 'Tokenización determinista',
        desc: 'El mismo secreto siempre genera el mismo placeholder dentro de una sesión, así el contexto multironda y el prompt caching sobreviven al enmascaramiento.',
      },
      {
        title: 'Reversible en local',
        desc: 'El proveedor ve placeholders; tu terminal, tus archivos y tus tool calls recuperan los valores reales.',
      },
    ],
  },

  faq: {
    eyebrow: 'Preguntas frecuentes',
    title: 'Preguntas comunes',
    items: [
      {
        q: '¿Veil añade latencia?',
        a: 'Corre en localhost y solo reescribe el cuerpo del request y del response, así que el overhead es un único salto local: insignificante comparado con el round-trip hasta tu proveedor.',
      },
      {
        q: '¿Cambia la salida del modelo?',
        a: 'No. Los placeholders son deterministas y preservan el formato, así que el modelo razona sobre valores estables y bien formados. Veil restaura los valores reales en el response antes de que tus herramientas los vean.',
      },
      {
        q: '¿Veil ve mis API keys?',
        a: 'Veil nunca almacena ni accede a las credenciales de tu proveedor. Solo reescribe el cuerpo del request y del response; tus API keys pasan intactas sin que Veil las lea.',
      },
      {
        q: '¿Qué agentes son compatibles?',
        a: 'En la v0.1.0: Claude Code (proxy Anthropic Messages) y Codex CLI (proxy OpenAI Responses), más integraciones con el Go SDK. OpenAI Chat Completions, Gemini y más están en el roadmap.',
      },
      {
        q: '¿Cómo lo quito?',
        a: 'Quita la variable de entorno. Veil es solo un proceso local: no hay cuenta, daemon ni background agent que desinstalar.',
      },
    ],
  },

  footer: {
    copyright: 'Colaboradores de Veil. Apache-2.0.',
    github: 'GitHub',
    releases: 'Releases',
    security: 'Seguridad',
  },

  notFound: {
    code: '404',
    title: 'Página no encontrada',
    message: 'La página que buscas no existe.',
    back: 'Volver al inicio',
  },
} as const;
