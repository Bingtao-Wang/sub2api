import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CustomPageView from '../CustomPageView.vue'

const { appStore } = vi.hoisted(() => ({
  appStore: {
    publicSettingsLoaded: true,
    cachedPublicSettings: { custom_menu_items: [{ id: 'docs', url: 'https://example.com/docs' }] },
  },
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: 'docs' } }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: false, user: { id: 7 }, token: 'test-token' }) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/api/client', () => ({ buildApiUrl: (path: string) => `/api/v1${path}` }))

const wrappers: ReturnType<typeof mount>[] = []

function mountPage() {
  const wrapper = mount(CustomPageView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
  })
  wrappers.push(wrapper)
  return wrapper
}

describe('custom page open button', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings.custom_menu_items = [{ id: 'docs', url: 'https://example.com/docs' }]
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.unstubAllGlobals()
  })

  it.each([undefined, false, true])('honors the per-menu hide button setting %s while keeping the iframe', (hidden) => {
    Object.assign(appStore.cachedPublicSettings.custom_menu_items[0], { hide_open_button: hidden })
    const wrapper = mountPage()
    expect(wrapper.find('.custom-embed-toolbar').exists()).toBe(hidden !== true)
    expect(wrapper.get('iframe').attributes('src')).toContain('https://example.com/docs')
  })

  it('keeps the open link in a toolbar above the iframe with auth parameters intact', () => {
    const wrapper = mountPage()
    const toolbar = wrapper.get('.custom-embed-toolbar')
    const button = toolbar.get<HTMLAnchorElement>('a').element
    const iframe = wrapper.get('iframe')

    expect(toolbar.element.nextElementSibling).toBe(iframe.element)
    expect(button.href).toBe(iframe.attributes('src'))
    expect(button.href).toContain('user_id=7')
    expect(button.href).toContain('token=test-token')
    expect(button.target).toBe('_blank')
    expect(button.rel).toBe('noopener noreferrer')
  })

  it('keeps Markdown pages separate from the embedded-page controls', async () => {
    appStore.cachedPublicSettings.custom_menu_items = [{ id: 'docs', url: 'md:guide' }]
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, text: async () => '# Guide' }))
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.find('.custom-embed-toolbar').exists()).toBe(false)
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('.markdown-page-content h1').text()).toBe('Guide')
  })
})
