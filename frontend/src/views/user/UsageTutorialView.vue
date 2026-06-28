<template>
  <AppLayout>
    <div class="tutorial-shell">
      <section class="tutorial-main">
        <div class="tutorial-toolbar">
          <div class="tutorial-row">
            <span class="tutorial-label">{{ t('tutorial.channel') }}</span>
            <button
              v-for="option in tutorialOptions"
              :key="option.id"
              type="button"
              class="tutorial-pill"
              :class="{ 'tutorial-pill-active': selectedClient === option.id }"
              @click="selectedClient = option.id"
            >
              {{ option.channelLabel }}
            </button>
          </div>

          <div class="tutorial-row">
            <span class="tutorial-label">{{ t('tutorial.model') }}</span>
            <select v-model="selectedModel" class="tutorial-select">
              <option v-for="model in modelOptions" :key="model" :value="model">
                {{ model }}
              </option>
            </select>
          </div>

          <div class="tutorial-row">
            <span class="tutorial-label">{{ t('tutorial.client') }}</span>
            <button
              v-for="option in tutorialOptions"
              :key="option.id"
              type="button"
              class="tutorial-pill"
              :class="{ 'tutorial-pill-active': selectedClient === option.id }"
              @click="selectedClient = option.id"
            >
              <Icon name="terminal" size="sm" />
              {{ option.clientLabel }}
            </button>
          </div>

          <div class="tutorial-row">
            <span class="tutorial-label">{{ t('tutorial.apiKey') }}</span>
            <select v-model.number="selectedKeyId" class="tutorial-select tutorial-key-select">
              <option v-if="availableKeys.length === 0" :value="0">
                {{ noAvailableKeyText }}
              </option>
              <option v-for="key in availableKeys" :key="key.id" :value="key.id">
                {{ key.name }} · {{ maskApiKey(key.key) }}
              </option>
            </select>
          </div>

          <div class="tutorial-mobile-endpoints">
            <EndpointSelector
              :endpoints="endpointOptions"
              :selected-id="selectedEndpointId"
              @select="selectedEndpointId = $event"
            />
          </div>
        </div>

        <div v-if="loading" class="tutorial-state">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
          <span>{{ t('common.loading') }}</span>
        </div>

        <div v-else-if="availableKeys.length === 0" class="tutorial-empty">
          <Icon name="key" size="xl" class="text-gray-400" />
          <h2>{{ emptyTitle }}</h2>
          <p>{{ emptyDescription }}</p>
          <router-link to="/keys" class="btn btn-primary">
            <Icon name="plus" size="sm" />
            {{ t('keys.createKey') }}
          </router-link>
        </div>

        <div v-else class="tutorial-content">
          <TutorialStep :index="1" :title="t('tutorial.steps.node')">
            <p class="tutorial-step-note">{{ t('tutorial.nodeInstall.chooseMethod') }}</p>
            <CodeBlock
              v-for="command in nodeInstallCommands"
              :key="command.path"
              :path="command.path"
              :content="command.content"
              :hint="command.hint"
            />
            <p class="tutorial-step-note">{{ t('tutorial.nodeInstall.verify') }}</p>
            <CodeBlock path="Terminal" :content="'node -v\nnpm -v'" />
            <a
              href="https://nodejs.org/en/download"
              target="_blank"
              rel="noopener noreferrer"
              class="tutorial-doc-link"
            >
              {{ t('tutorial.nodeInstall.officialDownload') }}
            </a>
          </TutorialStep>

          <TutorialStep :index="2" :title="installStepTitle">
            <CodeBlock path="Terminal" :content="installCommand" />
          </TutorialStep>

          <TutorialStep :index="3" :title="verifyStepTitle">
            <CodeBlock path="Terminal" :content="verifyCommand" />
          </TutorialStep>

          <TutorialStep :index="4" :title="createConfigDirStepTitle">
            <CodeBlock :path="directoryCommand.path" :content="directoryCommand.content" />
            <p class="tutorial-step-note">{{ t('tutorial.steps.openConfigFile') }}</p>
            <CodeBlock :path="openConfigCommand.path" :content="openConfigCommand.content" />
          </TutorialStep>

          <TutorialStep :index="5" :title="writeConfigStepTitle">
            <CodeBlock
              v-for="file in configFiles"
              :key="file.path"
              :path="file.path"
              :content="file.content"
              :hint="file.hint"
            />
          </TutorialStep>

          <TutorialStep :index="6" :title="startStepTitle">
            <CodeBlock path="Terminal" :content="startCommand" />
          </TutorialStep>
        </div>
      </section>

      <aside class="tutorial-endpoints">
        <EndpointSelector
          :endpoints="endpointOptions"
          :selected-id="selectedEndpointId"
          @select="selectedEndpointId = $event"
        />
      </aside>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import { useAppStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { buildClaudeCodeConfigFiles, buildCodexConfigFiles, type ClientConfigFile, type CodexConfigOs } from '@/utils/clientConfig'
import { maskApiKey } from '@/utils/maskApiKey'
import type { ApiKey, CustomEndpoint, PublicSettings } from '@/types'

interface EndpointOption {
  id: string
  name: string
  endpoint: string
  description: string
}

interface TutorialCommand {
  path: string
  content: string
  hint?: string
}

type TutorialClient = 'codex' | 'claude'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const apiKeys = ref<ApiKey[]>([])
const publicSettings = ref<PublicSettings | null>(appStore.cachedPublicSettings)
const selectedClient = ref<TutorialClient>('codex')
const selectedModel = ref('gpt-5.5')
const selectedKeyId = ref(0)
const selectedEndpointId = ref('default')
const selectedOs = ref<CodexConfigOs>('windows')
const copiedPath = ref('')

const codexModelOptions = ['gpt-5.5', 'gpt-5.4', 'gpt-5.4-mini', 'gpt-5.3-codex', 'codex-mini-latest']
const claudeModelOptions = ['claude-opus-4-8', 'claude-opus-4-7', 'claude-sonnet-4-6', 'claude-sonnet-4-5', 'claude-fable-5']

const tutorialOptions = computed(() => [
  {
    id: 'codex' as const,
    channelLabel: t('tutorial.openaiOfficial'),
    clientLabel: 'Codex',
  },
  {
    id: 'claude' as const,
    channelLabel: t('tutorial.maxOfficial'),
    clientLabel: 'Claude Code',
  },
])

const modelOptions = computed(() =>
  selectedClient.value === 'claude' ? claudeModelOptions : codexModelOptions
)

const nodeInstallCommands = computed<TutorialCommand[]>(() => selectedOs.value === 'windows'
  ? [
      {
        path: 'PowerShell',
        content: 'winget install --id OpenJS.NodeJS.LTS -e',
        hint: t('tutorial.nodeInstall.windowsHint'),
      },
    ]
  : [
      {
        path: 'macOS Terminal',
        content: 'brew install node',
        hint: t('tutorial.nodeInstall.macosHint'),
      },
      {
        path: 'Ubuntu / Debian Terminal',
        content: 'curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -\nsudo apt-get install -y nodejs',
        hint: t('tutorial.nodeInstall.linuxHint'),
      },
    ]
)

const openAIKeys = computed(() =>
  apiKeys.value.filter((key) => key.status === 'active' && key.group?.platform === 'openai')
)

const claudeKeys = computed(() =>
  apiKeys.value.filter((key) => key.status === 'active' && key.group?.platform === 'anthropic')
)

const availableKeys = computed(() =>
  selectedClient.value === 'claude' ? claudeKeys.value : openAIKeys.value
)

const selectedKey = computed(() =>
  availableKeys.value.find((key) => key.id === selectedKeyId.value) ?? availableKeys.value[0] ?? null
)

const noAvailableKeyText = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.noAvailableClaudeKey') : t('tutorial.noAvailableKey')
)

const emptyTitle = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.emptyClaudeTitle') : t('tutorial.emptyTitle')
)

const emptyDescription = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.emptyClaudeDescription') : t('tutorial.emptyDescription')
)

const endpointOptions = computed<EndpointOption[]>(() => {
  const settings = publicSettings.value
  const items: EndpointOption[] = []
  const defaultEndpoint = settings?.api_base_url?.trim() || window.location.origin
  items.push({
    id: 'default',
    name: t('tutorial.defaultEndpoint'),
    endpoint: defaultEndpoint,
    description: t('tutorial.defaultEndpointDescription'),
  })

  const customEndpoints = settings?.custom_endpoints ?? []
  customEndpoints.forEach((endpoint: CustomEndpoint, index: number) => {
    if (!endpoint.endpoint?.trim()) return
    items.push({
      id: `custom-${index}`,
      name: endpoint.name || t('tutorial.customEndpoint', { n: index + 1 }),
      endpoint: endpoint.endpoint.trim(),
      description: endpoint.description || endpoint.endpoint.trim(),
    })
  })
  return items
})

const selectedEndpoint = computed(() =>
  endpointOptions.value.find((endpoint) => endpoint.id === selectedEndpointId.value) ?? endpointOptions.value[0]
)

const selectedBaseUrl = computed(() => selectedEndpoint.value?.endpoint || window.location.origin)

const configFiles = computed<ClientConfigFile[]>(() => {
  if (!selectedKey.value) return []
  if (selectedClient.value === 'claude') {
    return buildClaudeCodeConfigFiles({
      baseUrl: selectedBaseUrl.value,
      apiKey: selectedKey.value.key,
      model: selectedModel.value,
      os: selectedOs.value,
      includeShellEnv: false,
      settingsHint: t('tutorial.claudeSettingsHint'),
    })
  }

  return buildCodexConfigFiles({
    baseUrl: selectedBaseUrl.value,
    apiKey: selectedKey.value.key,
    model: selectedModel.value,
    os: selectedOs.value,
    configTomlHint: t('tutorial.configTomlHint'),
  })
})

const directoryCommand = computed(() => selectedOs.value === 'windows'
  ? { path: 'Command Prompt', content: `mkdir C:\\Users\\<用户名>\\${selectedClient.value === 'claude' ? '.claude' : '.codex'}` }
  : { path: 'Terminal', content: `mkdir -p ~/${selectedClient.value === 'claude' ? '.claude' : '.codex'}` }
)

const openConfigCommand = computed(() => selectedOs.value === 'windows'
  ? {
      path: 'Command Prompt',
      content: selectedClient.value === 'claude'
        ? 'notepad C:\\Users\\<用户名>\\.claude\\settings.json'
        : 'notepad C:\\Users\\<用户名>\\.codex\\config.toml',
    }
  : {
      path: 'Terminal',
      content: selectedClient.value === 'claude'
        ? 'nano ~/.claude/settings.json'
        : 'nano ~/.codex/config.toml',
    }
)

const installStepTitle = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.steps.installClaude') : t('tutorial.steps.installCodex')
)

const verifyStepTitle = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.steps.verifyClaude') : t('tutorial.steps.verifyCodex')
)

const createConfigDirStepTitle = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.steps.createClaudeConfigDir') : t('tutorial.steps.createConfigDir')
)

const writeConfigStepTitle = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.steps.writeClaudeConfig') : t('tutorial.steps.writeConfig')
)

const startStepTitle = computed(() =>
  selectedClient.value === 'claude' ? t('tutorial.steps.startClaude') : t('tutorial.steps.startCodex')
)

const installCommand = computed(() =>
  selectedClient.value === 'claude' ? 'npm install -g @anthropic-ai/claude-code' : 'npm install -g @openai/codex'
)

const verifyCommand = computed(() =>
  selectedClient.value === 'claude' ? 'claude --version' : 'codex --version'
)

const startCommand = computed(() =>
  selectedClient.value === 'claude' ? 'claude' : 'codex'
)

watch(selectedClient, () => {
  selectedModel.value = modelOptions.value[0]
  selectedKeyId.value = 0
  copiedPath.value = ''
})

watch(availableKeys, (keys) => {
  if (keys.length === 0) {
    selectedKeyId.value = 0
    return
  }
  if (!keys.some((key) => key.id === selectedKeyId.value)) {
    selectedKeyId.value = keys[0].id
  }
}, { immediate: true })

watch(endpointOptions, (endpoints) => {
  if (!endpoints.some((endpoint) => endpoint.id === selectedEndpointId.value)) {
    selectedEndpointId.value = endpoints[0]?.id ?? 'default'
  }
}, { immediate: true })

async function loadPageData() {
  loading.value = true
  try {
    const [keysResponse, settingsResponse] = await Promise.all([
      keysAPI.list(1, 1000, { status: 'active', sort_by: 'created_at', sort_order: 'desc' }),
      appStore.fetchPublicSettings(),
    ])
    apiKeys.value = keysResponse.items
    publicSettings.value = settingsResponse ?? appStore.cachedPublicSettings
  } catch (error) {
    console.error('Failed to load tutorial data:', error)
    appStore.showError(t('tutorial.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function copyBlock(content: string, path: string) {
  const success = await copyToClipboard(content, t('keys.copied'))
  if (!success) return
  copiedPath.value = path
  window.setTimeout(() => {
    if (copiedPath.value === path) copiedPath.value = ''
  }, 1600)
}

const TutorialStep = defineComponent({
  name: 'TutorialStep',
  props: {
    index: { type: Number, required: true },
    title: { type: String, required: true },
  },
  setup(props, { slots }) {
    return () => h('section', { class: 'tutorial-step' }, [
      h('div', { class: 'tutorial-step-title' }, [
        h('span', { class: 'tutorial-step-index' }, `${props.index}.`),
        h('span', props.title),
      ]),
      slots.default?.(),
    ])
  },
})

const CodeBlock = defineComponent({
  name: 'TutorialCodeBlock',
  props: {
    path: { type: String, required: true },
    content: { type: String, required: true },
    hint: { type: String, default: '' },
  },
  setup(props) {
    return () => h('div', { class: 'tutorial-code-wrap' }, [
      props.hint
        ? h('p', { class: 'tutorial-code-hint' }, props.hint)
        : null,
      h('div', { class: 'tutorial-code' }, [
        h('div', { class: 'tutorial-code-header' }, [
          h('span', { class: 'tutorial-code-path' }, props.path),
          h('button', {
            type: 'button',
            class: [
              'tutorial-copy',
              copiedPath.value === props.path ? 'tutorial-copy-done' : '',
            ],
            onClick: () => copyBlock(props.content, props.path),
          }, copiedPath.value === props.path ? t('keys.copied') : t('keys.useKeyModal.copy')),
        ]),
        h('pre', { class: 'tutorial-pre' }, [
          h('code', props.content),
        ]),
      ]),
    ])
  },
})

const EndpointSelector = defineComponent({
  name: 'TutorialEndpointSelector',
  props: {
    endpoints: { type: Array as () => EndpointOption[], required: true },
    selectedId: { type: String, required: true },
  },
  emits: ['select'],
  setup(props, { emit }) {
    return () => h('div', { class: 'tutorial-endpoint-card' }, [
      h('h2', { class: 'tutorial-endpoint-title' }, t('tutorial.endpointTitle')),
      h('div', { class: 'tutorial-endpoint-list' }, props.endpoints.map((endpoint) =>
        h('button', {
          key: endpoint.id,
          type: 'button',
          class: [
            'tutorial-endpoint-option',
            endpoint.id === props.selectedId ? 'tutorial-endpoint-active' : '',
          ],
          onClick: () => emit('select', endpoint.id),
        }, [
          h('span', { class: 'tutorial-endpoint-name' }, endpoint.name),
          h('span', { class: 'tutorial-endpoint-url' }, endpoint.endpoint),
          endpoint.description
            ? h('span', { class: 'tutorial-endpoint-description' }, endpoint.description)
            : null,
        ])
      )),
      h('div', { class: 'tutorial-os-toggle' }, [
        h('button', {
          type: 'button',
          class: selectedOs.value === 'windows' ? 'tutorial-os-active' : '',
          onClick: () => { selectedOs.value = 'windows' },
        }, 'Windows'),
        h('button', {
          type: 'button',
          class: selectedOs.value === 'unix' ? 'tutorial-os-active' : '',
          onClick: () => { selectedOs.value = 'unix' },
        }, 'macOS / Linux'),
      ]),
    ])
  },
})

onMounted(loadPageData)
</script>

<style>
.tutorial-shell {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 280px;
  gap: 0;
  min-height: calc(100vh - 64px - 4rem);
  overflow: hidden;
  border: 1px solid theme('colors.gray.200');
  border-radius: 1rem;
  background: theme('colors.white');
}

.dark .tutorial-shell {
  border-color: theme('colors.dark.700');
  background: theme('colors.dark.900');
}

.tutorial-main {
  min-width: 0;
  overflow: auto;
}

.tutorial-toolbar {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  flex-wrap: wrap;
  gap: 1rem 1.5rem;
  border-bottom: 1px solid theme('colors.gray.200');
  background: rgba(255, 255, 255, 0.95);
  padding: 1.25rem 1.5rem;
  backdrop-filter: blur(12px);
}

.dark .tutorial-toolbar {
  border-color: theme('colors.dark.700');
  background: rgba(17, 24, 39, 0.94);
}

.tutorial-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}

.tutorial-label {
  flex: 0 0 auto;
  font-size: 0.875rem;
  color: theme('colors.gray.500');
}

.tutorial-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  border: 1px solid theme('colors.gray.200');
  border-radius: 0.625rem;
  padding: 0.5rem 0.875rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: theme('colors.gray.600');
  background: theme('colors.white');
}

.tutorial-pill-active {
  border-color: theme('colors.primary.300');
  color: theme('colors.primary.700');
  background: theme('colors.primary.50');
  box-shadow: 0 0 0 1px theme('colors.primary.200');
}

.dark .tutorial-pill {
  border-color: theme('colors.dark.600');
  background: theme('colors.dark.800');
  color: theme('colors.dark.200');
}

.dark .tutorial-pill-active {
  border-color: theme('colors.primary.500');
  color: theme('colors.primary.300');
  background: rgb(20 184 166 / 0.12);
}

.tutorial-select {
  min-height: 2.5rem;
  max-width: 15rem;
  border: 1px solid theme('colors.gray.200');
  border-radius: 0.625rem;
  background: theme('colors.white');
  padding: 0.5rem 2rem 0.5rem 0.75rem;
  font-size: 0.875rem;
  color: theme('colors.gray.800');
}

.tutorial-key-select {
  max-width: 22rem;
}

.dark .tutorial-select {
  border-color: theme('colors.dark.600');
  background: theme('colors.dark.800');
  color: theme('colors.dark.100');
}

.tutorial-content {
  max-width: 900px;
  padding: 1.75rem 1.5rem 3rem;
}

.tutorial-step {
  margin-bottom: 1.5rem;
}

.tutorial-step-title {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
  font-size: 0.9375rem;
  font-weight: 600;
  color: theme('colors.gray.700');
}

.dark .tutorial-step-title {
  color: theme('colors.dark.200');
}

.tutorial-step-index {
  color: theme('colors.gray.500');
}

.tutorial-step-note {
  margin: 0.75rem 0;
  font-size: 0.875rem;
  color: theme('colors.gray.500');
}

.tutorial-doc-link {
  display: inline-flex;
  margin-top: 0.875rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: theme('colors.primary.600');
}

.tutorial-doc-link:hover {
  color: theme('colors.primary.700');
  text-decoration: underline;
  text-underline-offset: 4px;
}

.dark .tutorial-doc-link {
  color: theme('colors.primary.300');
}

.tutorial-code-wrap {
  margin-top: 0.75rem;
}

.tutorial-code-hint {
  margin-bottom: 0.375rem;
  font-size: 0.75rem;
  color: theme('colors.amber.600');
}

.tutorial-code {
  overflow: hidden;
  border: 1px solid theme('colors.gray.200');
  border-radius: 0.875rem;
  background: theme('colors.gray.50');
}

.dark .tutorial-code {
  border-color: theme('colors.dark.700');
  background: theme('colors.dark.950');
}

.tutorial-code-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  border-bottom: 1px solid theme('colors.gray.200');
  padding: 0.625rem 0.875rem;
}

.dark .tutorial-code-header {
  border-color: theme('colors.dark.700');
}

.tutorial-code-path {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: theme('colors.gray.500');
}

.tutorial-copy {
  flex: 0 0 auto;
  border-radius: 0.5rem;
  background: theme('colors.gray.100');
  padding: 0.25rem 0.625rem;
  font-size: 0.75rem;
  font-weight: 600;
  color: theme('colors.gray.600');
}

.tutorial-copy-done {
  background: theme('colors.green.100');
  color: theme('colors.green.700');
}

.dark .tutorial-copy {
  background: theme('colors.dark.800');
  color: theme('colors.dark.200');
}

.tutorial-pre {
  overflow-x: auto;
  padding: 0.875rem 1rem;
  font-size: 0.8125rem;
  line-height: 1.75;
  color: theme('colors.gray.800');
}

.dark .tutorial-pre {
  color: theme('colors.dark.100');
}

.tutorial-endpoints {
  border-left: 1px solid theme('colors.gray.200');
  padding: 1.5rem 1rem;
}

.dark .tutorial-endpoints {
  border-color: theme('colors.dark.700');
}

.tutorial-endpoint-title {
  margin-bottom: 0.75rem;
  font-size: 0.875rem;
  font-weight: 700;
  color: theme('colors.gray.500');
}

.tutorial-endpoint-list {
  display: grid;
  gap: 0.625rem;
}

.tutorial-endpoint-option {
  position: relative;
  display: grid;
  width: 100%;
  gap: 0.25rem;
  border: 1px solid theme('colors.gray.200');
  border-radius: 0.875rem;
  padding: 0.875rem;
  text-align: left;
  background: theme('colors.white');
}

.tutorial-endpoint-active {
  border-color: theme('colors.primary.300');
  background: theme('colors.primary.50');
  box-shadow: 0 0 0 1px theme('colors.primary.200');
}

.dark .tutorial-endpoint-option {
  border-color: theme('colors.dark.700');
  background: theme('colors.dark.800');
}

.dark .tutorial-endpoint-active {
  border-color: theme('colors.primary.500');
  background: rgb(20 184 166 / 0.12);
}

.tutorial-endpoint-name {
  font-size: 0.875rem;
  font-weight: 700;
  color: theme('colors.gray.700');
}

.tutorial-endpoint-url,
.tutorial-endpoint-description {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: theme('colors.gray.500');
}

.dark .tutorial-endpoint-name {
  color: theme('colors.dark.100');
}

.tutorial-os-toggle {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.25rem;
  margin-top: 1rem;
  border-radius: 0.75rem;
  background: theme('colors.gray.100');
  padding: 0.25rem;
}

.tutorial-os-toggle button {
  border-radius: 0.625rem;
  padding: 0.5rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: theme('colors.gray.500');
}

.tutorial-os-toggle .tutorial-os-active {
  background: theme('colors.white');
  color: theme('colors.gray.900');
  box-shadow: theme('boxShadow.sm');
}

.dark .tutorial-os-toggle {
  background: theme('colors.dark.800');
}

.dark .tutorial-os-toggle .tutorial-os-active {
  background: theme('colors.dark.700');
  color: theme('colors.white');
}

.tutorial-mobile-endpoints {
  display: none;
  width: 100%;
}

.tutorial-state,
.tutorial-empty {
  display: flex;
  min-height: 360px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  padding: 2rem;
  text-align: center;
  color: theme('colors.gray.500');
}

.tutorial-empty h2 {
  font-size: 1rem;
  font-weight: 700;
  color: theme('colors.gray.900');
}

.tutorial-empty p {
  max-width: 28rem;
  font-size: 0.875rem;
}

.dark .tutorial-empty h2 {
  color: theme('colors.white');
}

@media (max-width: 1023px) {
  .tutorial-shell {
    display: block;
    min-height: auto;
    overflow: visible;
  }

  .tutorial-main {
    overflow: visible;
  }

  .tutorial-toolbar {
    position: static;
  }

  .tutorial-row {
    width: 100%;
  }

  .tutorial-select {
    max-width: none;
    flex: 1 1 auto;
  }

  .tutorial-content {
    padding: 1.25rem 1rem 2rem;
  }

  .tutorial-endpoints {
    display: none;
  }

  .tutorial-mobile-endpoints {
    display: block;
  }
}
</style>
