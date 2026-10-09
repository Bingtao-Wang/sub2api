<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
            <span>{{ t('admin.gallery.summary.total', { count: total }) }}</span>
            <span>{{ t('admin.gallery.summary.pageSize', { count: pageSize }) }}</span>
          </div>
          <div class="ml-auto flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading"
              :title="t('admin.gallery.actions.refresh')"
              @click="loadItems"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="cleanupLoading"
              @click="runCleanup"
            >
              <Icon name="trash" size="md" class="mr-1" />
              {{ t('admin.gallery.actions.cleanup') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="items" :loading="loading">
          <template #cell-image="{ row }">
            <a :href="row.image_url" target="_blank" rel="noopener" class="block h-20 w-20 overflow-hidden rounded border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800">
              <img
                :src="row.thumb_url || row.image_url"
                :alt="t('admin.gallery.imageAlt')"
                loading="lazy"
                class="h-full w-full object-cover"
              />
            </a>
          </template>

          <template #cell-prompt="{ row }">
            <div class="max-w-md">
              <p class="line-clamp-2 whitespace-normal text-sm text-gray-900 dark:text-gray-100">{{ row.prompt }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ row.model || '-' }} · {{ row.size || '-' }} · {{ row.quality || '-' }} · {{ row.format || '-' }}
              </p>
            </div>
          </template>

          <template #cell-status="{ row }">
            <div class="flex flex-wrap gap-1">
              <span :class="statusBadgeClass(row.status)">{{ statusLabel(row.status) }}</span>
              <span v-if="row.permanent" class="badge badge-success">{{ t('admin.gallery.badges.permanent') }}</span>
              <span v-if="row.featured" class="badge badge-warning">{{ t('admin.gallery.badges.featured') }}</span>
            </div>
          </template>

          <template #cell-user_name="{ row }">
            <span>{{ row.user_name || t('admin.gallery.userFallback', { id: row.user_id }) }}</span>
          </template>

          <template #cell-image_size_bytes="{ value }">
            <span>{{ formatBytes(value || 0, 1) }}</span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-1">
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
                :title="row.status === 'visible' ? t('admin.gallery.actions.hide') : t('admin.gallery.actions.show')"
                :disabled="isUpdating(row.id)"
                @click="toggleVisible(row)"
              >
                <Icon :name="row.status === 'visible' ? 'eyeOff' : 'eye'" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400"
                :title="row.permanent ? t('admin.gallery.actions.unsetPermanent') : t('admin.gallery.actions.setPermanent')"
                :disabled="isUpdating(row.id)"
                @click="updateItem(row.id, { permanent: !row.permanent })"
              >
                <Icon name="lock" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-amber-50 hover:text-amber-600 dark:hover:bg-amber-900/20 dark:hover:text-amber-400"
                :title="row.featured ? t('admin.gallery.actions.unsetFeatured') : t('admin.gallery.actions.setFeatured')"
                :disabled="isUpdating(row.id)"
                @click="updateItem(row.id, { featured: !row.featured })"
              >
                <Icon name="badge" size="sm" />
              </button>
              <button
                type="button"
                class="rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('admin.gallery.actions.delete')"
                :disabled="isUpdating(row.id)"
                @click="deleteItem(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>

          <template #empty>
            <div class="flex flex-col items-center">
              <Icon name="grid" size="xl" class="mb-4 h-12 w-12 text-gray-400 dark:text-dark-500" />
              <p class="text-lg font-medium text-gray-900 dark:text-gray-100">{{ t('admin.gallery.empty') }}</p>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-dark-700">
          <button type="button" class="btn btn-secondary" :disabled="page <= 1 || loading" @click="changePage(page - 1)">
            {{ t('admin.gallery.pagination.previous') }}
          </button>
          <span class="text-sm text-gray-600 dark:text-dark-300">
            {{ t('admin.gallery.pagination.indicator', { page, pages }) }}
          </span>
          <button type="button" class="btn btn-secondary" :disabled="page >= pages || loading" @click="changePage(page + 1)">
            {{ t('admin.gallery.pagination.next') }}
          </button>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI, type GalleryItem, type GalleryUpdateRequest } from '@/api/admin'
import type { Column } from '@/components/common/types'
import { formatBytes, formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores'

const appStore = useAppStore()
const { t } = useI18n()
const pageSize = 20
const page = ref(1)
const pages = ref(1)
const total = ref(0)
const loading = ref(false)
const cleanupLoading = ref(false)
const items = ref<GalleryItem[]>([])
const updatingIds = ref<Set<number>>(new Set())
let controller: AbortController | null = null

const columns = computed<Column[]>(() => [
  { key: 'image', label: t('admin.gallery.columns.image') },
  { key: 'prompt', label: t('admin.gallery.columns.prompt') },
  { key: 'status', label: t('admin.gallery.columns.status') },
  { key: 'user_name', label: t('admin.gallery.columns.user') },
  { key: 'image_size_bytes', label: t('admin.gallery.columns.size') },
  { key: 'created_at', label: t('admin.gallery.columns.publishedAt') },
  { key: 'actions', label: t('admin.gallery.columns.actions') }
])

function statusLabel(status: string): string {
  if (status === 'visible') return t('admin.gallery.status.visible')
  if (status === 'hidden') return t('admin.gallery.status.hidden')
  if (status === 'deleted') return t('admin.gallery.status.deleted')
  return status || '-'
}

function statusBadgeClass(status: string): string {
  if (status === 'visible') return 'badge badge-success'
  if (status === 'hidden') return 'badge badge-warning'
  return 'badge badge-secondary'
}

function isUpdating(id: number): boolean {
  return updatingIds.value.has(id)
}

function setUpdating(id: number, updating: boolean) {
  const next = new Set(updatingIds.value)
  if (updating) next.add(id)
  else next.delete(id)
  updatingIds.value = next
}

async function loadItems() {
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  try {
    const response = await adminAPI.gallery.list(page.value, pageSize, { signal: controller.signal })
    items.value = response.items
    total.value = response.total
    pages.value = Math.max(response.pages || 1, 1)
  } catch (error: any) {
    if (error?.code !== 'ERR_CANCELED') {
      appStore.showError(error?.response?.data?.message || t('admin.gallery.messages.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function changePage(nextPage: number) {
  page.value = Math.min(Math.max(nextPage, 1), pages.value)
  await loadItems()
}

async function updateItem(id: number, request: GalleryUpdateRequest) {
  setUpdating(id, true)
  try {
    const updated = await adminAPI.gallery.update(id, request)
    items.value = items.value.map(item => item.id === id ? updated : item)
    appStore.showSuccess(t('admin.gallery.messages.updated'))
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || t('admin.gallery.messages.updateFailed'))
  } finally {
    setUpdating(id, false)
  }
}

async function toggleVisible(item: GalleryItem) {
  await updateItem(item.id, { status: item.status === 'visible' ? 'hidden' : 'visible' })
}

async function deleteItem(item: GalleryItem) {
  if (!window.confirm(t('admin.gallery.messages.confirmDelete', { id: item.id }))) return
  await updateItem(item.id, { status: 'deleted' })
  items.value = items.value.filter(current => current.id !== item.id)
  total.value = Math.max(total.value - 1, 0)
}

async function runCleanup() {
  cleanupLoading.value = true
  try {
    const result = await adminAPI.gallery.cleanup()
    appStore.showSuccess(t('admin.gallery.messages.cleanupComplete', {
      count: result.deleted_records,
      size: formatBytes(result.freed_bytes, 1)
    }))
    await loadItems()
  } catch (error: any) {
    appStore.showError(error?.response?.data?.message || t('admin.gallery.messages.cleanupFailed'))
  } finally {
    cleanupLoading.value = false
  }
}

onMounted(loadItems)

onUnmounted(() => {
  controller?.abort()
})
</script>
