import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export type GalleryItemStatus = 'visible' | 'hidden' | 'deleted'

export interface GalleryItem {
  id: number
  user_id: number
  user_name?: string
  prompt: string
  revised_prompt?: string
  model: string
  size: string
  quality: string
  format: string
  mode: string
  image_url: string
  thumb_url: string
  image_size_bytes: number
  status: GalleryItemStatus
  permanent: boolean
  featured: boolean
  created_at: string
  updated_at: string
  deleted_at?: string | null
}

export interface GalleryUpdateRequest {
  status?: GalleryItemStatus
  permanent?: boolean
  featured?: boolean
}

export interface GalleryCleanupResult {
  deleted_records: number
  deleted_files: number
  freed_bytes: number
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<GalleryItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<GalleryItem>>('/admin/gallery/items', {
    params: { page, page_size: pageSize },
    signal: options?.signal
  })
  return data
}

export async function update(id: number, request: GalleryUpdateRequest): Promise<GalleryItem> {
  const { data } = await apiClient.put<GalleryItem>(`/admin/gallery/items/${id}`, request)
  return data
}

export async function cleanup(): Promise<GalleryCleanupResult> {
  const { data } = await apiClient.post<GalleryCleanupResult>('/admin/gallery/cleanup')
  return data
}

const galleryAPI = {
  list,
  update,
  cleanup
}

export default galleryAPI
