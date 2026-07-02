export type CodexConfigOs = 'unix' | 'windows'
export type ClaudeCodeConfigOs = 'unix' | 'windows' | 'cmd' | 'powershell'

export interface ClientConfigFile {
  path: string
  content: string
  hint?: string
}

export interface BuildCodexConfigOptions {
  baseUrl: string
  apiKey: string
  model?: string
  os?: CodexConfigOs
  websocket?: boolean
  configTomlHint?: string
}

export interface BuildClaudeCodeConfigOptions {
  baseUrl: string
  apiKey: string
  model?: string
  os?: ClaudeCodeConfigOs
  includeShellEnv?: boolean
  settingsHint?: string
}

const claudeCodeDefaultEnv = [
  ['ANTHROPIC_DEFAULT_FABLE_MODEL', 'claude-fable-5[1M]'],
  ['ANTHROPIC_DEFAULT_FABLE_MODEL_NAME', 'claude-fable-5'],
  ['ANTHROPIC_DEFAULT_SONNET_MODEL', 'claude-sonnet-5'],
  ['ANTHROPIC_DEFAULT_SONNET_MODEL_NAME', 'claude-sonnet-5'],
  ['ANTHROPIC_DEFAULT_OPUS_MODEL', 'claude-opus-4-8'],
  ['ANTHROPIC_DEFAULT_OPUS_MODEL_NAME', 'claude-opus-4-8'],
  ['CLAUDE_CODE_ATTRIBUTION_HEADER', '0'],
  ['CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC', '1'],
  ['CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS', '1'],
  ['ENABLE_TOOL_SEARCH', 'true'],
] as const

export function buildCodexConfigFiles(options: BuildCodexConfigOptions): ClientConfigFile[] {
  const {
    baseUrl,
    apiKey,
    model = 'gpt-5.5',
    os = 'unix',
    websocket = false,
    configTomlHint,
  } = options

  const configDir = os === 'windows' ? '%userprofile%\\.codex' : '~/.codex'
  const websocketLines = websocket
    ? 'supports_websockets = true\n'
    : ''
  const featureLines = websocket
    ? 'responses_websockets_v2 = true\ngoals = true'
    : 'goals = true'

  const configContent = `model_provider = "OpenAI"
model = "${model}"
review_model = "${model}"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}"
wire_api = "responses"
${websocketLines}requires_openai_auth = true

[features]
${featureLines}`

  const authContent = `{
  "OPENAI_API_KEY": "${apiKey}"
}`

  return [
    {
      path: `${configDir}/config.toml`,
      content: configContent,
      hint: configTomlHint,
    },
    {
      path: `${configDir}/auth.json`,
      content: authContent,
    },
  ]
}

export function buildClaudeCodeConfigFiles(options: BuildClaudeCodeConfigOptions): ClientConfigFile[] {
  const {
    baseUrl,
    apiKey,
    model,
    os = 'unix',
    includeShellEnv = true,
    settingsHint,
  } = options

  const files: ClientConfigFile[] = []
  const envEntries = [
    ['ANTHROPIC_BASE_URL', baseUrl],
    ['ANTHROPIC_AUTH_TOKEN', apiKey],
    ...(model ? [['ANTHROPIC_MODEL', model] as const] : []),
    ...claudeCodeDefaultEnv,
  ] as const

  if (includeShellEnv) {
    switch (os) {
      case 'cmd':
      case 'windows':
        files.push({
          path: 'Command Prompt',
          content: envEntries.map(([key, value]) => `set ${key}=${value}`).join('\n'),
        })
        break
      case 'powershell':
        files.push({
          path: 'PowerShell',
          content: envEntries.map(([key, value]) => `$env:${key}="${value}"`).join('\n'),
        })
        break
      default:
        files.push({
          path: 'Terminal',
          content: envEntries.map(([key, value]) => `export ${key}="${value}"`).join('\n'),
        })
    }
  }

  const settingsPath = os === 'unix'
    ? '~/.claude/settings.json'
    : '%userprofile%\\.claude\\settings.json'
  const settingsEnv = envEntries
    .map(([key, value], index) => `    "${key}": ${JSON.stringify(value)}${index === envEntries.length - 1 ? '' : ','}`)
    .join('\n')

  files.push({
    path: settingsPath,
    content: `{
  "env": {
${settingsEnv}
  }
}`,
    hint: settingsHint,
  })

  return files
}
