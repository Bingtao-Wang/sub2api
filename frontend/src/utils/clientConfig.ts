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

  if (includeShellEnv) {
    switch (os) {
      case 'cmd':
      case 'windows':
        files.push({
          path: 'Command Prompt',
          content: `set ANTHROPIC_BASE_URL=${baseUrl}
set ANTHROPIC_AUTH_TOKEN=${apiKey}
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
set CLAUDE_CODE_ATTRIBUTION_HEADER=0`,
        })
        break
      case 'powershell':
        files.push({
          path: 'PowerShell',
          content: `$env:ANTHROPIC_BASE_URL="${baseUrl}"
$env:ANTHROPIC_AUTH_TOKEN="${apiKey}"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
$env:CLAUDE_CODE_ATTRIBUTION_HEADER=0`,
        })
        break
      default:
        files.push({
          path: 'Terminal',
          content: `export ANTHROPIC_BASE_URL="${baseUrl}"
export ANTHROPIC_AUTH_TOKEN="${apiKey}"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
export CLAUDE_CODE_ATTRIBUTION_HEADER=0`,
        })
    }
  }

  const settingsPath = os === 'unix'
    ? '~/.claude/settings.json'
    : '%userprofile%\\.claude\\settings.json'
  const modelLine = model
    ? `,
    "ANTHROPIC_MODEL": "${model}"`
    : ''

  files.push({
    path: settingsPath,
    content: `{
  "env": {
    "ANTHROPIC_BASE_URL": "${baseUrl}",
    "ANTHROPIC_AUTH_TOKEN": "${apiKey}",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"${modelLine}
  }
}`,
    hint: settingsHint,
  })

  return files
}
