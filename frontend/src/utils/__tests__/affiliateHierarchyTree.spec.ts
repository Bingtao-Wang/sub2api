import { describe, expect, it } from 'vitest'

import type { AffiliateHierarchyNode } from '@/api/admin/affiliateHierarchy'
import {
  getAffiliateHierarchyDescendantCountMap,
  getVisibleAffiliateHierarchyNodes,
} from '../affiliateHierarchyTree'

function node(
  userId: number,
  inviterId: number | null,
  depth: number,
  path: number[],
  overrides: Partial<AffiliateHierarchyNode> = {},
): AffiliateHierarchyNode {
  return {
    user_id: userId,
    email: `user-${userId}@example.com`,
    username: '',
    aff_code: `AFF${userId}`,
    inviter_id: inviterId,
    parent_email: inviterId ? `user-${inviterId}@example.com` : '',
    parent_username: '',
    depth,
    path,
    effective_rebate_rate_percent: 0,
    rebate_rate_custom: false,
    direct_invite_count: 0,
    team_size: 0,
    self_recharge_amount: 0,
    team_recharge_amount: 0,
    rebate_amount: 0,
    agent_access_enabled: false,
    ...overrides,
  }
}

describe('affiliateHierarchyTree', () => {
  it('collapses all descendants for the selected node', () => {
    const nodes = [
      node(1, null, 0, [1]),
      node(2, 1, 1, [1, 2]),
      node(3, 2, 2, [1, 2, 3]),
      node(4, 1, 1, [1, 4]),
    ]

    const visible = getVisibleAffiliateHierarchyNodes(nodes, new Set([2]), { key: '', order: 'asc' })

    expect(visible.map((item) => item.user_id)).toEqual([1, 2, 4])
  })

  it('sorts siblings without moving children outside their parent branch', () => {
    const nodes = [
      node(1, null, 0, [1]),
      node(2, 1, 1, [1, 2], { team_recharge_amount: 20 }),
      node(3, 2, 2, [1, 2, 3], { team_recharge_amount: 1000 }),
      node(4, 1, 1, [1, 4], { team_recharge_amount: 80 }),
    ]

    const visible = getVisibleAffiliateHierarchyNodes(nodes, new Set(), {
      key: 'team_recharge_amount',
      order: 'desc',
    })

    expect(visible.map((item) => item.user_id)).toEqual([1, 4, 2, 3])
  })

  it('counts descendants from node paths', () => {
    const counts = getAffiliateHierarchyDescendantCountMap([
      node(1, null, 0, [1]),
      node(2, 1, 1, [1, 2]),
      node(3, 2, 2, [1, 2, 3]),
    ])

    expect(counts.get(1)).toBe(2)
    expect(counts.get(2)).toBe(1)
    expect(counts.get(3)).toBeUndefined()
  })
})
