import { apiClient } from './client'
import type { AffiliateHierarchyReport } from './admin/affiliateHierarchy'

export interface AffiliateAgentAccess {
  user_id: number
  email: string
  username: string
  aff_code: string
  effective_rebate_rate_percent: number
  enabled: boolean
  notes: string
  created_by_admin_id?: number | null
  created_at: string
  updated_at: string
}

export interface GetMyAffiliateHierarchyParams {
  search?: string
  start_at?: string
  end_at?: string
  max_depth?: number
  limit?: number
  timezone?: string
}

export async function getMyAgentAccess(): Promise<AffiliateAgentAccess> {
  const { data } = await apiClient.get<AffiliateAgentAccess>('/user/aff/hierarchy/access')
  return data
}

export async function getMyAffiliateHierarchy(
  params: GetMyAffiliateHierarchyParams = {},
): Promise<AffiliateHierarchyReport> {
  const { data } = await apiClient.get<AffiliateHierarchyReport>(
    '/user/aff/hierarchy',
    {
      params: {
        search: params.search || undefined,
        start_at: params.start_at || undefined,
        end_at: params.end_at || undefined,
        max_depth: params.max_depth ?? 20,
        limit: params.limit ?? 2000,
        timezone: params.timezone || undefined,
      },
    },
  )
  return data
}

export const affiliateHierarchyAPI = {
  getMyAgentAccess,
  getMyAffiliateHierarchy,
}

export default affiliateHierarchyAPI
