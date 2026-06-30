<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="checkingAccess" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <div v-else-if="!access?.enabled" class="card p-8 text-center">
        <Icon name="lock" size="xl" class="mx-auto mb-4 h-12 w-12 text-gray-400 dark:text-dark-500" />
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('affiliate.hierarchy.noAccessTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.hierarchy.noAccessDescription') }}</p>
      </div>

      <template v-else>
        <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
          <SummaryStat :label="t('affiliate.hierarchy.nodeCount')" :value="String(summary.node_count)" />
          <SummaryStat :label="t('affiliate.hierarchy.teamSize')" :value="String(summary.team_size)" />
          <SummaryStat :label="t('affiliate.hierarchy.teamRecharge')" :value="formatMoney(summary.team_recharge_amount)" />
          <SummaryStat :label="t('affiliate.hierarchy.selfRecharge')" :value="formatMoney(summary.self_recharge_amount)" />
          <SummaryStat :label="t('affiliate.hierarchy.rebateAmount')" :value="formatMoney(summary.rebate_amount)" />
        </div>

        <TablePageLayout>
          <template #filters>
            <div class="flex flex-wrap items-end gap-3">
              <div class="w-full sm:w-44">
                <label class="input-label">{{ t('affiliate.hierarchy.startAt') }}</label>
                <input v-model="filters.start_at" type="date" class="input" @change="loadHierarchy" />
              </div>
              <div class="w-full sm:w-44">
                <label class="input-label">{{ t('affiliate.hierarchy.endAt') }}</label>
                <input v-model="filters.end_at" type="date" class="input" @change="loadHierarchy" />
              </div>
              <div class="w-full sm:w-36">
                <label class="input-label">{{ t('affiliate.hierarchy.maxDepth') }}</label>
                <input v-model.number="filters.max_depth" type="number" min="1" max="20" class="input" @change="loadHierarchy" />
              </div>
              <div class="relative w-full md:w-72">
                <label class="input-label">{{ t('affiliate.hierarchy.nodeSearch') }}</label>
                <Icon name="search" size="md" class="absolute left-3 top-[2.35rem] text-gray-400" />
                <input
                  v-model="filters.search"
                  type="text"
                  class="input pl-10"
                  :placeholder="t('affiliate.hierarchy.nodeSearchPlaceholder')"
                  @input="debounceHierarchySearch"
                />
              </div>
              <button class="btn btn-secondary px-2 md:px-3" :disabled="loading" :title="t('common.refresh')" @click="loadHierarchy">
                <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
              </button>
            </div>
          </template>

          <template #table>
            <DataTable :columns="columns" :data="nodes" :loading="loading" row-key="user_id">
              <template #cell-user="{ row }">
                <div class="flex items-center gap-2" :style="{ paddingLeft: `${Math.min(row.depth, 12) * 18}px` }">
                  <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                    {{ relativeDepthLabel(row.depth) }}
                  </span>
                  <div class="min-w-0">
                    <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                      {{ row.depth === 0 ? t('affiliate.hierarchy.me') : (row.email || row.username || ('#' + row.user_id)) }}
                    </div>
                    <div class="truncate text-xs text-gray-500 dark:text-dark-400">#{{ row.user_id }}</div>
                  </div>
                </div>
              </template>
              <template #cell-depth="{ row }">
                <span class="text-sm text-gray-700 dark:text-dark-200">{{ relativeDepthName(row.depth) }}</span>
              </template>
              <template #cell-parent="{ row }">
                <span v-if="row.depth === 0" class="text-sm text-gray-400">-</span>
                <span v-else-if="row.inviter_id === summary.root_user_id" class="text-sm text-gray-700 dark:text-dark-200">
                  {{ t('affiliate.hierarchy.me') }}
                </span>
                <span v-else class="text-sm text-gray-700 dark:text-dark-200">
                  {{ row.parent_email || row.parent_username || ('#' + row.inviter_id) }}
                </span>
              </template>
              <template #cell-aff_code="{ row }">
                <span class="font-mono text-sm text-gray-700 dark:text-dark-200">{{ row.aff_code || '-' }}</span>
              </template>
              <template #cell-effective_rebate_rate_percent="{ row }">
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ formatPercent(row.effective_rebate_rate_percent) }}</span>
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
              <template #empty>
                <div class="flex flex-col items-center">
                  <Icon name="users" size="xl" class="mb-4 h-12 w-12 text-gray-400 dark:text-dark-500" />
                  <p class="text-lg font-medium text-gray-900 dark:text-gray-100">{{ t('affiliate.hierarchy.emptyNodes') }}</p>
                </div>
              </template>
            </DataTable>
          </template>
        </TablePageLayout>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  getMyAgentAccess,
  getMyAffiliateHierarchy,
  type AffiliateAgentAccess,
} from '@/api/affiliateHierarchy'
import type {
  AffiliateHierarchyNode,
  AffiliateHierarchySummary,
} from '@/api/admin/affiliateHierarchy'

const { t } = useI18n()
const appStore = useAppStore()

const checkingAccess = ref(true)
const access = ref<AffiliateAgentAccess | null>(null)
const nodes = ref<AffiliateHierarchyNode[]>([])
const loading = ref(false)
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
let hierarchySearchTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('affiliate.hierarchy.user') },
  { key: 'depth', label: t('affiliate.hierarchy.depth') },
  { key: 'parent', label: t('affiliate.hierarchy.parent') },
  { key: 'aff_code', label: t('affiliate.hierarchy.affCode') },
  { key: 'effective_rebate_rate_percent', label: t('affiliate.hierarchy.effectiveRate') },
  { key: 'direct_invite_count', label: t('affiliate.hierarchy.directInvites') },
  { key: 'team_size', label: t('affiliate.hierarchy.teamSize') },
  { key: 'self_recharge_amount', label: t('affiliate.hierarchy.selfRecharge') },
  { key: 'team_recharge_amount', label: t('affiliate.hierarchy.teamRecharge') },
  { key: 'rebate_amount', label: t('affiliate.hierarchy.rebateAmount') },
])

function userTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

function resetSummary(next?: AffiliateHierarchySummary) {
  const value = next ?? {
    root_user_id: access.value?.user_id ?? 0,
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

async function loadAccess() {
  checkingAccess.value = true
  try {
    access.value = await getMyAgentAccess()
    if (access.value.enabled) {
      await loadHierarchy()
    }
  } catch (err) {
    appStore.showError(extractI18nErrorMessage(err, t, 'affiliate.hierarchy.errors', t('affiliate.hierarchy.errors.loadAccessFailed')))
  } finally {
    checkingAccess.value = false
  }
}

async function loadHierarchy() {
  if (!access.value?.enabled) {
    nodes.value = []
    resetSummary()
    return
  }
  loading.value = true
  try {
    const report = await getMyAffiliateHierarchy({
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
    appStore.showError(extractI18nErrorMessage(err, t, 'affiliate.hierarchy.errors', t('affiliate.hierarchy.errors.loadFailed')))
  } finally {
    loading.value = false
  }
}

function debounceHierarchySearch() {
  if (hierarchySearchTimer) clearTimeout(hierarchySearchTimer)
  hierarchySearchTimer = setTimeout(() => {
    void loadHierarchy()
  }, 300)
}

function relativeDepthLabel(depth: number): string {
  return depth === 0 ? t('affiliate.hierarchy.meShort') : String(depth)
}

function relativeDepthName(depth: number): string {
  if (depth === 0) return t('affiliate.hierarchy.me')
  if (depth === 1) return t('affiliate.hierarchy.directDownline')
  return t('affiliate.hierarchy.nthDownline', { depth })
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
  void loadAccess()
})
</script>
