import { mount, flushPromises } from '@vue/test-utils'
import { describe, expect, it, vi, beforeEach } from 'vitest'

const { listKeys, fetchPublicSettings, showError, copyToClipboard } = vi.hoisted(() => ({
  listKeys: vi.fn(),
  fetchPublicSettings: vi.fn(),
  showError: vi.fn(),
  copyToClipboard: vi.fn().mockResolvedValue(true),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'tutorial.customEndpoint') return `Custom endpoint ${params?.n}`
        return key
      }
    })
  }
})

vi.mock('@/api/keys', () => ({
  keysAPI: {
    list: listKeys,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: null,
    fetchPublicSettings,
    showError,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

import UsageTutorialView from '../UsageTutorialView.vue'

function openAIKey(overrides = {}) {
  return {
    id: 1,
    user_id: 1,
    key: 'sk-openai',
    name: 'OpenAI Key',
    group_id: 1,
    status: 'active',
    ip_whitelist: [],
    ip_blacklist: [],
    last_used_at: null,
    quota: 0,
    quota_used: 0,
    expires_at: null,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    group: { id: 1, name: 'OpenAI', platform: 'openai' },
    rate_limit_5h: 0,
    rate_limit_1d: 0,
    rate_limit_7d: 0,
    usage_5h: 0,
    usage_1d: 0,
    usage_7d: 0,
    window_5h_start: null,
    window_1d_start: null,
    window_7d_start: null,
    reset_5h_at: null,
    reset_1d_at: null,
    reset_7d_at: null,
    ...overrides,
  }
}

function mountView() {
  return mount(UsageTutorialView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: { template: '<span />' },
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('UsageTutorialView', () => {
  beforeEach(() => {
    listKeys.mockReset()
    fetchPublicSettings.mockReset()
    showError.mockReset()
    copyToClipboard.mockClear()
  })

  it('shows an empty state when no active OpenAI key is available', async () => {
    listKeys.mockResolvedValue({
      items: [openAIKey({ id: 2, status: 'inactive' })],
      total: 1,
      page: 1,
      page_size: 1000,
      pages: 1,
    })
    fetchPublicSettings.mockResolvedValue({
      api_base_url: 'https://api.example.com/v1',
      custom_endpoints: [],
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('tutorial.emptyTitle')
    expect(wrapper.text()).toContain('keys.createKey')
  })

  it('renders selected API key and endpoint in Codex config', async () => {
    listKeys.mockResolvedValue({
      items: [
        openAIKey(),
        openAIKey({ id: 2, key: 'sk-gemini', name: 'Gemini Key', group: { id: 2, name: 'Gemini', platform: 'gemini' } }),
      ],
      total: 2,
      page: 1,
      page_size: 1000,
      pages: 1,
    })
    fetchPublicSettings.mockResolvedValue({
      api_base_url: 'https://default.example.com/v1',
      custom_endpoints: [
        { name: 'Overseas', endpoint: 'https://overseas.example.com/v1', description: 'fast' },
      ],
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('winget install --id OpenJS.NodeJS.LTS -e')
    expect(wrapper.text()).toContain('node -v')
    expect(wrapper.text()).toContain('npm -v')
    expect(wrapper.text()).toContain('npm install -g @openai/codex')
    expect(wrapper.text()).toContain('base_url = "https://default.example.com/v1"')
    expect(wrapper.text()).toContain('"OPENAI_API_KEY": "sk-openai"')
    expect(wrapper.text()).not.toContain('sk-gemini')

    const overseasButton = wrapper.findAll('button').find((button) => button.text().includes('Overseas'))
    expect(overseasButton).toBeDefined()
    await overseasButton!.trigger('click')

    expect(wrapper.text()).toContain('base_url = "https://overseas.example.com/v1"')
  })

  it('renders selected API key and endpoint in Claude Code config', async () => {
    listKeys.mockResolvedValue({
      items: [
        openAIKey(),
        openAIKey({
          id: 3,
          key: 'sk-claude',
          name: 'Claude Key',
          group: { id: 3, name: 'Claude', platform: 'anthropic' },
        }),
      ],
      total: 2,
      page: 1,
      page_size: 1000,
      pages: 1,
    })
    fetchPublicSettings.mockResolvedValue({
      api_base_url: 'https://default.example.com',
      custom_endpoints: [
        { name: 'Claude Overseas', endpoint: 'https://claude-overseas.example.com', description: 'fast' },
      ],
    })

    const wrapper = mountView()
    await flushPromises()

    const maxButton = wrapper.findAll('button').find((button) => button.text().includes('tutorial.maxOfficial'))
    expect(maxButton).toBeDefined()
    await maxButton!.trigger('click')

    expect(wrapper.text()).toContain('npm install -g @anthropic-ai/claude-code')
    expect(wrapper.text()).toContain('claude --version')
    expect(wrapper.text()).toContain('notepad C:\\Users\\<用户名>\\.claude\\settings.json')
    expect(wrapper.text()).toContain('"ANTHROPIC_BASE_URL": "https://default.example.com"')
    expect(wrapper.text()).toContain('"ANTHROPIC_AUTH_TOKEN": "sk-claude"')
    expect(wrapper.text()).toContain('"ANTHROPIC_MODEL": "claude-opus-4-8"')
    expect(wrapper.text()).not.toContain('"OPENAI_API_KEY": "sk-openai"')

    const overseasButton = wrapper.findAll('button').find((button) => button.text().includes('Claude Overseas'))
    expect(overseasButton).toBeDefined()
    await overseasButton!.trigger('click')

    expect(wrapper.text()).toContain('"ANTHROPIC_BASE_URL": "https://claude-overseas.example.com"')
  })

  it('copies code block content', async () => {
    listKeys.mockResolvedValue({
      items: [openAIKey()],
      total: 1,
      page: 1,
      page_size: 1000,
      pages: 1,
    })
    fetchPublicSettings.mockResolvedValue({
      api_base_url: 'https://api.example.com/v1',
      custom_endpoints: [],
    })

    const wrapper = mountView()
    await flushPromises()

    const copyButton = wrapper.findAll('button').find((button) => button.text() === 'keys.useKeyModal.copy')
    expect(copyButton).toBeDefined()
    await copyButton!.trigger('click')

    expect(copyToClipboard).toHaveBeenCalled()
  })
})
