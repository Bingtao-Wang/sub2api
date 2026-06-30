<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-end gap-3 xl:flex-nowrap">
            <div class="relative w-full md:w-80 xl:w-96 xl:shrink-0">
              <label class="input-label">{{ t('admin.affiliates.hierarchy.rootUser') }}</label>
              <div class="relative">
                <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input
                  v-model="rootSearch"
                  type="text"
                  class="input pl-10"
                  :placeholder="t('admin.affiliates.hierarchy.rootSearchPlaceholder')"
                  @focus="rootPickerOpen = true"
                  @input="debounceRootSearch"
                />
              </div>
              <div
                v-if="rootPickerOpen && roots.length > 0"
                class="absolute z-20 mt-2 max-h-72 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-900"
              >
                <button
                  v-for="root in roots"
                  :key="root.user_id"
                  type="button"
                  class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-dark-800"
                  @click="selectRoot(root)"
                >
                  <span class="min-w-0">
                    <span class="block truncate text-sm font-medium text-gray-900 dark:text-white">
                      {{ root.email || root.username || ('#' + root.user_id) }}
                    </span>
                  <span class="block truncate text-xs text-gray-500 dark:text-dark-400">
                    #{{ root.user_id }} · {{ root.aff_code || '-' }} · {{ formatPercent(root.effective_rebate_rate_percent) }}
                  </span>
                </span>
                  <span class="flex shrink-0 flex-col items-end gap-1 text-xs">
                    <span class="text-gray-500 dark:text-dark-400">
                      {{ t('admin.affiliates.hierarchy.teamSizeShort', { count: root.team_size }) }}
                    </span>
                    <span
                      class="rounded-full px-2 py-0.5"
                      :class="root.agent_access_enabled ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'"
                    >
                      {{ root.agent_access_enabled ? t('admin.affiliates.hierarchy.agentEnabled') : t('admin.affiliates.hierarchy.agentDisabled') }}
                    </span>
                  </span>
                </button>
              </div>
            </div>

            <div class="w-[calc(50%-0.375rem)] sm:w-40 xl:w-36 xl:shrink-0">
              <label class="input-label">{{ t('admin.affiliates.hierarchy.startAt') }}</label>
              <input v-model="filters.start_at" type="date" class="input" @change="loadHierarchy" />
            </div>
            <div class="w-[calc(50%-0.375rem)] sm:w-40 xl:w-36 xl:shrink-0">
              <label class="input-label">{{ t('admin.affiliates.hierarchy.endAt') }}</label>
              <input v-model="filters.end_at" type="date" class="input" @change="loadHierarchy" />
            </div>
            <div class="w-full sm:w-28 xl:w-28 xl:shrink-0">
              <label class="input-label">{{ t('admin.affiliates.hierarchy.maxDepth') }}</label>
              <input v-model.number="filters.max_depth" type="number" min="1" max="20" class="input" @change="loadHierarchy" />
            </div>
            <div class="relative w-full md:w-64 xl:min-w-56 xl:flex-1">
              <label class="input-label">{{ t('admin.affiliates.hierarchy.nodeSearch') }}</label>
              <Icon name="search" size="md" class="absolute left-3 top-[2.35rem] text-gray-400" />
              <input
                v-model="filters.search"
                type="text"
                class="input pl-10"
                :placeholder="t('admin.affiliates.hierarchy.nodeSearchPlaceholder')"
                @input="debounceHierarchySearch"
              />
            </div>
            <button class="btn btn-secondary px-2 md:px-3" :disabled="loading || !selectedRoot" :title="t('common.refresh')" @click="loadHierarchy">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary ml-auto shrink-0" @click="openAgentDialog">
              <Icon name="plus" size="sm" />
              <span>{{ t('admin.affiliates.hierarchy.addAgent') }}</span>
            </button>
          </div>

          <div v-if="selectedRoot" class="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <SummaryStat :label="t('admin.affiliates.hierarchy.nodeCount')" :value="String(summary.node_count)" />
            <SummaryStat :label="t('admin.affiliates.hierarchy.teamSize')" :value="String(summary.team_size)" />
            <SummaryStat :label="t('admin.affiliates.hierarchy.teamRecharge')" :value="formatMoney(summary.team_recharge_amount)" />
            <SummaryStat :label="t('admin.affiliates.hierarchy.selfRecharge')" :value="formatMoney(summary.self_recharge_amount)" />
            <SummaryStat :label="t('admin.affiliates.hierarchy.rebateAmount')" :value="formatMoney(summary.rebate_amount)" />
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="nodes" :loading="loading" row-key="user_id">
          <template #cell-user="{ row }">
            <div class="flex items-center gap-2" :style="{ paddingLeft: `${Math.min(row.depth, 12) * 18}px` }">
              <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                {{ row.depth }}
              </span>
              <div class="min-w-0">
                <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                  {{ row.email || row.username || ('#' + row.user_id) }}
                </div>
                <div class="truncate text-xs text-gray-500 dark:text-dark-400">#{{ row.user_id }}</div>
              </div>
            </div>
          </template>
          <template #cell-depth="{ row }">
            <span class="tabular-nums text-sm text-gray-700 dark:text-dark-200">{{ row.depth }}</span>
          </template>
          <template #cell-parent="{ row }">
            <span v-if="row.inviter_id" class="text-sm text-gray-700 dark:text-dark-200">
              {{ row.parent_email || row.parent_username || ('#' + row.inviter_id) }}
            </span>
            <span v-else class="text-sm text-gray-400">-</span>
          </template>
          <template #cell-aff_code="{ row }">
            <span class="font-mono text-sm text-gray-700 dark:text-dark-200">{{ row.aff_code || '-' }}</span>
          </template>
          <template #cell-effective_rebate_rate_percent="{ row }">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                {{ formatPercent(row.effective_rebate_rate_percent) }}
              </span>
              <span
                class="rounded-full px-2 py-0.5 text-xs"
                :class="row.rebate_rate_custom ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'"
              >
                {{ row.rebate_rate_custom ? t('admin.affiliates.hierarchy.customRate') : t('admin.affiliates.hierarchy.globalRate') }}
              </span>
            </div>
          </template>
          <template #cell-direct_invite_count="{ row }">
            <span class="tabular-nums">{{ row.direct_invite_count }}</span>
          </template>
          <template #cell-team_size="{ row }">
            <span class="tabular-nums">{{ row.team_size }}</span>
          </template>
          <template #cell-self_recharge_amount="{ row }">
            <span class="tabular-nums">{{ formatMoney(row.self_recharge_amount) }}</span>
          </template>
          <template #cell-team_recharge_amount="{ row }">
            <span class="tabular-nums font-medium">{{ formatMoney(row.team_recharge_amount) }}</span>
          </template>
          <template #cell-rebate_amount="{ row }">
            <span class="tabular-nums font-medium text-primary-700 dark:text-primary-300">{{ formatMoney(row.rebate_amount) }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button class="btn btn-secondary btn-sm" :title="t('common.edit')" @click="openRateDialog(row)">
                <Icon name="edit" size="sm" />
              </button>
              <button
                class="btn btn-secondary btn-sm"
                :disabled="savingAccessUserId === row.user_id"
                :title="isAgentEnabled(row.user_id) ? t('admin.affiliates.hierarchy.disableAgent') : t('admin.affiliates.hierarchy.enableAgent')"
                @click="toggleNodeAgentAccess(row)"
              >
                <Icon v-if="savingAccessUserId === row.user_id" name="refresh" size="sm" class="animate-spin" />
                <Icon v-else name="users" size="sm" />
              </button>
            </div>
          </template>
          <template #empty>
            <div class="flex flex-col items-center">
              <Icon name="users" size="xl" class="mb-4 h-12 w-12 text-gray-400 dark:text-dark-500" />
              <p class="text-lg font-medium text-gray-900 dark:text-gray-100">
                {{ selectedRoot ? t('admin.affiliates.hierarchy.emptyNodes') : t('admin.affiliates.hierarchy.selectRootFirst') }}
              </p>
            </div>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="rateDialogOpen"
      :title="t('admin.affiliates.hierarchy.editRate')"
      width="normal"
      @close="closeRateDialog"
    >
      <div v-if="editingNode" class="space-y-4">
        <div class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="font-mono text-sm text-gray-900 dark:text-white">#{{ editingNode.user_id }}</div>
          <div class="mt-1 truncate text-sm font-medium text-gray-900 dark:text-white">{{ editingNode.email || editingNode.username || '-' }}</div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.affiliates.hierarchy.currentRate') }} {{ formatPercent(editingNode.effective_rebate_rate_percent) }}
          </div>
        </div>
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
          <input v-model="clearRate" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          {{ t('admin.affiliates.hierarchy.clearRate') }}
        </label>
        <div>
          <label class="input-label">{{ t('admin.affiliates.hierarchy.newRate') }}</label>
          <input
            v-model.number="rateDraft"
            type="number"
            min="0"
            max="100"
            step="0.01"
            class="input"
            :disabled="clearRate"
          />
          <p class="input-hint">{{ t('admin.affiliates.hierarchy.rateHint') }}</p>
        </div>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" :disabled="savingRate" @click="closeRateDialog">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="savingRate || !canSubmitRate" @click="saveRate">
            <Icon v-if="savingRate" name="refresh" size="sm" class="mr-2 animate-spin" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog
      :show="agentDialogOpen"
      :title="t('admin.affiliates.hierarchy.addAgent')"
      width="normal"
      @close="closeAgentDialog"
    >
      <div class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.affiliates.hierarchy.agentUser') }}</label>
          <div class="relative">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="agentSearch"
              type="text"
              class="input pl-10"
              :placeholder="t('admin.affiliates.hierarchy.agentSearchPlaceholder')"
              @input="debounceAgentSearch"
              @focus="agentPickerOpen = true"
            />
          </div>
          <div
            v-if="agentPickerOpen && agentCandidates.length > 0"
            class="mt-2 max-h-60 overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-900"
          >
            <button
              v-for="user in agentCandidates"
              :key="user.user_id"
              type="button"
              class="flex w-full flex-col px-4 py-3 text-left hover:bg-gray-50 dark:hover:bg-dark-800"
              @click="selectAgentUser(user)"
            >
              <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ user.email || user.username || ('#' + user.user_id) }}</span>
              <span class="truncate text-xs text-gray-500 dark:text-dark-400">#{{ user.user_id }} · {{ user.aff_code || '-' }}</span>
            </button>
          </div>
        </div>

        <div v-if="selectedAgentUser" class="rounded-lg border border-gray-100 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="font-mono text-sm text-gray-900 dark:text-white">#{{ selectedAgentUser.user_id }}</div>
          <div class="mt-1 truncate text-sm font-medium text-gray-900 dark:text-white">{{ selectedAgentUser.email || selectedAgentUser.username || '-' }}</div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.affiliates.hierarchy.agentRate') }}</label>
          <input v-model.number="agentRateDraft" type="number" min="0" max="100" step="0.01" class="input" />
          <p class="input-hint">{{ t('admin.affiliates.hierarchy.rateHint') }}</p>
        </div>

        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
          <input v-model="agentAccessEnabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          {{ t('admin.affiliates.hierarchy.enableAgentAccess') }}
        </label>

        <div>
          <label class="input-label">{{ t('admin.affiliates.hierarchy.agentNotes') }}</label>
          <textarea v-model="agentNotes" class="input min-h-24" :placeholder="t('admin.affiliates.hierarchy.agentNotesPlaceholder')"></textarea>
        </div>

        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" :disabled="savingAgent" @click="closeAgentDialog">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="savingAgent || !canSubmitAgent" @click="saveAgent">
            <Icon v-if="savingAgent" name="refresh" size="sm" class="mr-2 animate-spin" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  affiliateHierarchyAPI,
  type AffiliateHierarchyNode,
  type AffiliateHierarchyRoot,
  type AffiliateHierarchySummary,
} from '@/api/admin/affiliateHierarchy'

const { t } = useI18n()
const appStore = useAppStore()

const roots = ref<AffiliateHierarchyRoot[]>([])
const selectedRoot = ref<AffiliateHierarchyRoot | null>(null)
const nodes = ref<AffiliateHierarchyNode[]>([])
const loading = ref(false)
const rootsLoading = ref(false)
const rootPickerOpen = ref(false)
const rootSearch = ref('')
const filters = reactive({
  search: '',
  start_at: '',
  end_at: '',
  max_depth: 20,
})
const summary = reactive<AffiliateHierarchySummary>({
  root_user_id: 0,
  node_count: 0,
  max_depth: 0,
  direct_invite_count: 0,
  team_size: 0,
  self_recharge_amount: 0,
  team_recharge_amount: 0,
  rebate_amount: 0,
})
const rateDialogOpen = ref(false)
const editingNode = ref<AffiliateHierarchyNode | null>(null)
const rateDraft = ref<number | null>(null)
const clearRate = ref(false)
const savingRate = ref(false)
const agentDialogOpen = ref(false)
const agentSearch = ref('')
const agentCandidates = ref<AffiliateHierarchyRoot[]>([])
const selectedAgentUser = ref<AffiliateHierarchyRoot | null>(null)
const agentPickerOpen = ref(false)
const agentRateDraft = ref<number | null>(20)
const agentAccessEnabled = ref(true)
const agentNotes = ref('')
const savingAgent = ref(false)
const savingAccessUserId = ref<number | null>(null)
let rootSearchTimer: ReturnType<typeof setTimeout> | null = null
let hierarchySearchTimer: ReturnType<typeof setTimeout> | null = null
let agentSearchTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.affiliates.hierarchy.user') },
  { key: 'depth', label: t('admin.affiliates.hierarchy.depth') },
  { key: 'parent', label: t('admin.affiliates.hierarchy.parent') },
  { key: 'aff_code', label: t('admin.affiliates.hierarchy.affCode') },
  { key: 'effective_rebate_rate_percent', label: t('admin.affiliates.hierarchy.effectiveRate') },
  { key: 'direct_invite_count', label: t('admin.affiliates.hierarchy.directInvites') },
  { key: 'team_size', label: t('admin.affiliates.hierarchy.teamSize') },
  { key: 'self_recharge_amount', label: t('admin.affiliates.hierarchy.selfRecharge') },
  { key: 'team_recharge_amount', label: t('admin.affiliates.hierarchy.teamRecharge') },
  { key: 'rebate_amount', label: t('admin.affiliates.hierarchy.rebateAmount') },
  { key: 'actions', label: t('admin.affiliates.hierarchy.actions') },
])

const canSubmitRate = computed(() => {
  if (clearRate.value) return true
  return typeof rateDraft.value === 'number' && Number.isFinite(rateDraft.value) && rateDraft.value >= 0 && rateDraft.value <= 100
})

const canSubmitAgent = computed(() => {
  return !!selectedAgentUser.value
    && typeof agentRateDraft.value === 'number'
    && Number.isFinite(agentRateDraft.value)
    && agentRateDraft.value >= 0
    && agentRateDraft.value <= 100
})

function userTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

function resetSummary(next?: AffiliateHierarchySummary) {
  const value = next ?? {
    root_user_id: selectedRoot.value?.user_id ?? 0,
    node_count: 0,
    max_depth: 0,
    direct_invite_count: 0,
    team_size: 0,
    self_recharge_amount: 0,
    team_recharge_amount: 0,
    rebate_amount: 0,
  }
  Object.assign(summary, value)
}

async function loadRoots() {
  rootsLoading.value = true
  try {
    roots.value = await affiliateHierarchyAPI.listHierarchyRoots({
      search: rootSearch.value.trim(),
      limit: 20,
    })
    if (!selectedRoot.value && roots.value.length > 0) {
      selectRoot(roots.value[0], false)
      await loadHierarchy()
    }
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.affiliates.hierarchy.errors', t('admin.affiliates.hierarchy.errors.loadRootsFailed')))
  } finally {
    rootsLoading.value = false
  }
}

function isAgentEnabled(userId: number): boolean {
  const node = nodes.value.find(item => item.user_id === userId)
  if (node) {
    return node.agent_access_enabled
  }
  if (selectedRoot.value?.user_id === userId) {
    return selectedRoot.value.agent_access_enabled
  }
  return roots.value.some(root => root.user_id === userId && root.agent_access_enabled)
}

async function loadHierarchy() {
  if (!selectedRoot.value) {
    nodes.value = []
    resetSummary()
    return
  }
  loading.value = true
  try {
    const report = await affiliateHierarchyAPI.getHierarchy({
      root_user_id: selectedRoot.value.user_id,
      search: filters.search.trim() || undefined,
      start_at: filters.start_at || undefined,
      end_at: filters.end_at || undefined,
      max_depth: filters.max_depth || 20,
      limit: 2000,
      timezone: userTimezone(),
    })
    nodes.value = report.nodes || []
    resetSummary(report.summary)
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.affiliates.hierarchy.errors', t('admin.affiliates.hierarchy.errors.loadFailed')))
  } finally {
    loading.value = false
  }
}

function selectRoot(root: AffiliateHierarchyRoot, shouldLoad = true) {
  selectedRoot.value = root
  rootSearch.value = `${root.email || root.username || '#' + root.user_id}`
  rootPickerOpen.value = false
  if (shouldLoad) {
    void loadHierarchy()
  }
}

function debounceRootSearch() {
  rootPickerOpen.value = true
  if (rootSearchTimer) clearTimeout(rootSearchTimer)
  rootSearchTimer = setTimeout(() => {
    void loadRoots()
  }, 300)
}

function debounceHierarchySearch() {
  if (hierarchySearchTimer) clearTimeout(hierarchySearchTimer)
  hierarchySearchTimer = setTimeout(() => {
    void loadHierarchy()
  }, 300)
}

function openRateDialog(node: AffiliateHierarchyNode) {
  editingNode.value = node
  rateDraft.value = node.effective_rebate_rate_percent
  clearRate.value = false
  rateDialogOpen.value = true
}

function closeRateDialog() {
  if (savingRate.value) return
  rateDialogOpen.value = false
  editingNode.value = null
  rateDraft.value = null
  clearRate.value = false
}

async function saveRate() {
  if (!editingNode.value || !canSubmitRate.value) return
  savingRate.value = true
  try {
    await affiliateHierarchyAPI.updateHierarchyUserRate(editingNode.value.user_id, {
      clear_rebate_rate: clearRate.value,
      aff_rebate_rate_percent: clearRate.value ? null : rateDraft.value,
    })
    appStore.showSuccess(t('admin.affiliates.hierarchy.rateSaved'))
    rateDialogOpen.value = false
    editingNode.value = null
    rateDraft.value = null
    clearRate.value = false
    await loadHierarchy()
    await loadRoots()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.affiliates.hierarchy.errors', t('admin.affiliates.hierarchy.errors.saveRateFailed')))
  } finally {
    savingRate.value = false
  }
}

function openAgentDialog() {
  agentDialogOpen.value = true
  agentPickerOpen.value = false
  agentSearch.value = ''
  agentCandidates.value = []
  selectedAgentUser.value = null
  agentRateDraft.value = 20
  agentAccessEnabled.value = true
  agentNotes.value = ''
}

function closeAgentDialog() {
  if (savingAgent.value) return
  agentDialogOpen.value = false
}

function debounceAgentSearch() {
  selectedAgentUser.value = null
  agentPickerOpen.value = true
  if (agentSearchTimer) clearTimeout(agentSearchTimer)
  agentSearchTimer = setTimeout(() => {
    void searchAgentUsers()
  }, 300)
}

async function searchAgentUsers() {
  const q = agentSearch.value.trim()
  if (!q) {
    agentCandidates.value = []
    return
  }
  try {
    agentCandidates.value = await affiliateHierarchyAPI.listHierarchyRoots({
      search: q,
      limit: 20,
    })
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.affiliates.hierarchy.errors', t('admin.affiliates.hierarchy.errors.loadUsersFailed')))
  }
}

function selectAgentUser(user: AffiliateHierarchyRoot) {
  selectedAgentUser.value = user
  agentSearch.value = user.email || user.username || `#${user.user_id}`
  agentPickerOpen.value = false
}

async function saveAgent() {
  if (!selectedAgentUser.value || !canSubmitAgent.value) return
  savingAgent.value = true
  try {
    await affiliateHierarchyAPI.updateHierarchyUserRate(selectedAgentUser.value.user_id, {
      aff_rebate_rate_percent: agentRateDraft.value,
      clear_rebate_rate: false,
    })
    await affiliateHierarchyAPI.updateHierarchyUserAccess(selectedAgentUser.value.user_id, {
      enabled: agentAccessEnabled.value,
      notes: agentNotes.value.trim(),
    })
    appStore.showSuccess(t('admin.affiliates.hierarchy.agentSaved'))
    agentDialogOpen.value = false
    await loadRoots()
    if (selectedAgentUser.value) {
      const root = roots.value.find(item => item.user_id === selectedAgentUser.value?.user_id)
      if (root) {
        selectRoot(root)
      } else {
        await loadHierarchy()
      }
    }
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.affiliates.hierarchy.errors', t('admin.affiliates.hierarchy.errors.saveAgentFailed')))
  } finally {
    savingAgent.value = false
  }
}

async function toggleNodeAgentAccess(node: AffiliateHierarchyNode) {
  if (savingAccessUserId.value) return
  const nextEnabled = !isAgentEnabled(node.user_id)
  savingAccessUserId.value = node.user_id
  try {
    await affiliateHierarchyAPI.updateHierarchyUserAccess(node.user_id, {
      enabled: nextEnabled,
      notes: '',
    })
    appStore.showSuccess(nextEnabled ? t('admin.affiliates.hierarchy.agentEnabledSaved') : t('admin.affiliates.hierarchy.agentDisabledSaved'))
    await loadRoots()
    await loadHierarchy()
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'admin.affiliates.hierarchy.errors', t('admin.affiliates.hierarchy.errors.saveAccessFailed')))
  } finally {
    savingAccessUserId.value = null
  }
}

function formatMoney(value?: number | null): string {
  const amount = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return '$' + amount.toFixed(2)
}

function formatPercent(value?: number | null): string {
  const amount = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return `${amount.toFixed(2)}%`
}

const SummaryStat = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
  },
  setup(props) {
    return () => h('div', { class: 'rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900' }, [
      h('div', { class: 'text-xs font-medium text-gray-500 dark:text-dark-400' }, props.label),
      h('div', { class: 'mt-1 truncate text-xl font-semibold tabular-nums text-gray-900 dark:text-white' }, props.value),
    ])
  },
})

onMounted(() => {
  void loadRoots()
})
</script>
