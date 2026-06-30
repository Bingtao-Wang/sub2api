import { apiClient } from '../client'

export interface AffiliateHierarchyRoot {
  user_id: number
  email: string
  username: string
  aff_code: string
  effective_rebate_rate_percent: number
  direct_invite_count: number
  team_size: number
  agent_access_enabled: boolean
}

export interface AffiliateHierarchyNode {
  user_id: number
  email: string
  username: string
  aff_code: string
  inviter_id?: number | null
  parent_email: string
  parent_username: string
  depth: number
  path: number[]
  effective_rebate_rate_percent: number
  rebate_rate_custom: boolean
  direct_invite_count: number
  team_size: number
  self_recharge_amount: number
  team_recharge_amount: number
  rebate_amount: number
  agent_access_enabled: boolean
}

export interface AffiliateHierarchySummary {
  root_user_id: number
  node_count: number
  max_depth: number
  direct_invite_count: number
  team_size: number
  self_recharge_amount: number
  team_recharge_amount: number
  rebate_amount: number
}

export interface AffiliateHierarchyReport {
  summary: AffiliateHierarchySummary
  nodes: AffiliateHierarchyNode[]
}

export interface ListHierarchyRootsParams {
  search?: string
  limit?: number
}

export interface GetHierarchyParams {
  root_user_id: number
  search?: string
  start_at?: string
  end_at?: string
  max_depth?: number
  limit?: number
  timezone?: string
}

export interface UpdateHierarchyUserRateRequest {
  aff_rebate_rate_percent?: number | null
  clear_rebate_rate?: boolean
}

export interface UpdateHierarchyUserAccessRequest {
  enabled: boolean
  notes?: string
}

export async function listHierarchyRoots(
  params: ListHierarchyRootsParams = {},
): Promise<AffiliateHierarchyRoot[]> {
  const { data } = await apiClient.get<AffiliateHierarchyRoot[]>(
    '/admin/affiliates/hierarchy/roots',
    {
      params: {
        search: params.search ?? '',
        limit: params.limit ?? 20,
      },
    },
  )
  return data
}

export async function getHierarchy(
  params: GetHierarchyParams,
): Promise<AffiliateHierarchyReport> {
  const { data } = await apiClient.get<AffiliateHierarchyReport>(
    '/admin/affiliates/hierarchy',
    {
      params: {
        root_user_id: params.root_user_id,
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

export async function updateHierarchyUserRate(
  userId: number,
  payload: UpdateHierarchyUserRateRequest,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.put<{ user_id: number }>(
    `/admin/affiliates/hierarchy/users/${userId}/rate`,
    payload,
  )
  return data
}

export async function updateHierarchyUserAccess(
  userId: number,
  payload: UpdateHierarchyUserAccessRequest,
): Promise<{ user_id: number; enabled: boolean }> {
  const { data } = await apiClient.put<{ user_id: number; enabled: boolean }>(
    `/admin/affiliates/hierarchy/users/${userId}/access`,
    payload,
  )
  return data
}

export const affiliateHierarchyAPI = {
  listHierarchyRoots,
  getHierarchy,
  updateHierarchyUserRate,
  updateHierarchyUserAccess,
}

export default affiliateHierarchyAPI
