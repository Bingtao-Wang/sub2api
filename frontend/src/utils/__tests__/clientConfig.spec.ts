import { describe, expect, it } from 'vitest'
import { buildClaudeCodeConfigFiles, buildCodexConfigFiles } from '../clientConfig'

describe('buildCodexConfigFiles', () => {
  it('builds Codex config and auth files with selected base URL, API key, and model', () => {
    const files = buildCodexConfigFiles({
      baseUrl: 'https://api.example.com/v1',
      apiKey: 'sk-live',
      model: 'gpt-5.4',
      os: 'windows',
      configTomlHint: 'hint',
    })

    const config = files.find((file) => file.path.endsWith('config.toml'))
    const auth = files.find((file) => file.path.endsWith('auth.json'))

    expect(config).toBeDefined()
    expect(config?.path).toBe('%userprofile%\\.codex/config.toml')
    expect(config?.hint).toBe('hint')
    expect(config?.content).toContain('model = "gpt-5.4"')
    expect(config?.content).toContain('review_model = "gpt-5.4"')
    expect(config?.content).toContain('base_url = "https://api.example.com/v1"')
    expect(config?.content).toContain('requires_openai_auth = true')
    expect(config?.content).toContain('[features]\ngoals = true')

    expect(auth?.content).toContain('"OPENAI_API_KEY": "sk-live"')
  })

  it('adds WebSocket feature flags when requested', () => {
    const files = buildCodexConfigFiles({
      baseUrl: 'https://api.example.com/v1',
      apiKey: 'sk-live',
      websocket: true,
    })

    const config = files[0].content
    expect(config).toContain('supports_websockets = true')
    expect(config).toContain('[features]\nresponses_websockets_v2 = true\ngoals = true')
  })
})

describe('buildClaudeCodeConfigFiles', () => {
  it('builds Claude Code settings with selected base URL, API key, and model', () => {
    const files = buildClaudeCodeConfigFiles({
      baseUrl: 'https://api.example.com',
      apiKey: 'sk-claude',
      model: 'claude-opus-4-8',
      os: 'windows',
      includeShellEnv: false,
      settingsHint: 'hint',
    })

    expect(files).toHaveLength(1)
    expect(files[0].path).toBe('%userprofile%\\.claude\\settings.json')
    expect(files[0].hint).toBe('hint')
    expect(files[0].content).toContain('"ANTHROPIC_BASE_URL": "https://api.example.com"')
    expect(files[0].content).toContain('"ANTHROPIC_AUTH_TOKEN": "sk-claude"')
    expect(files[0].content).toContain('"ANTHROPIC_MODEL": "claude-opus-4-8"')
    expect(files[0].content).toContain('"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"')
  })

  it('keeps the shell environment snippet available for the use-key modal', () => {
    const files = buildClaudeCodeConfigFiles({
      baseUrl: 'https://api.example.com',
      apiKey: 'sk-claude',
      os: 'powershell',
    })

    expect(files[0].path).toBe('PowerShell')
    expect(files[0].content).toContain('$env:ANTHROPIC_BASE_URL="https://api.example.com"')
    expect(files[1].path).toBe('%userprofile%\\.claude\\settings.json')
  })
})
