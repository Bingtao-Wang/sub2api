import type { AffiliateHierarchyNode } from '@/api/admin/affiliateHierarchy'

export type AffiliateHierarchySortOrder = 'asc' | 'desc'

export interface AffiliateHierarchySortState {
  key: string
  order: AffiliateHierarchySortOrder
}

const collator = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: 'base',
})

function displayUser(node: AffiliateHierarchyNode): string {
  return node.email || node.username || `#${node.user_id}`
}

function displayParent(node: AffiliateHierarchyNode): string {
  if (!node.inviter_id) return ''
  return node.parent_email || node.parent_username || `#${node.inviter_id}`
}

function isEmpty(value: unknown): boolean {
  return value === null || value === undefined || value === ''
}

function toNumber(value: unknown): number | null {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null
  if (typeof value === 'boolean') return value ? 1 : 0
  if (typeof value === 'string') {
    const parsed = Number(value.trim())
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

function toSortString(value: unknown): string {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  try {
    return JSON.stringify(value) ?? String(value)
  } catch {
    return String(value)
  }
}

function nodeSortValue(node: AffiliateHierarchyNode, key: string): unknown {
  switch (key) {
    case 'user':
      return displayUser(node)
    case 'parent':
      return displayParent(node)
    case 'actions':
      return node.agent_access_enabled ? 1 : 0
    default:
      return node[key as keyof AffiliateHierarchyNode]
  }
}

function compareValues(a: unknown, b: unknown): number {
  const aEmpty = isEmpty(a)
  const bEmpty = isEmpty(b)
  if (aEmpty && bEmpty) return 0
  if (aEmpty) return 1
  if (bEmpty) return -1

  const aNum = toNumber(a)
  const bNum = toNumber(b)
  if (aNum !== null && bNum !== null) {
    if (aNum === bNum) return 0
    return aNum < bNum ? -1 : 1
  }

  const compared = collator.compare(toSortString(a), toSortString(b))
  if (compared === 0) return 0
  return compared < 0 ? -1 : 1
}

function isHiddenByCollapsedAncestor(
  node: AffiliateHierarchyNode,
  collapsedIds: ReadonlySet<number>,
): boolean {
  const ancestors = Array.isArray(node.path) ? node.path.slice(0, -1) : []
  return ancestors.some((id) => collapsedIds.has(id))
}

export function getAffiliateHierarchyDescendantCountMap(
  nodes: AffiliateHierarchyNode[],
): Map<number, number> {
  const counts = new Map<number, number>()
  for (const node of nodes) {
    if (!Array.isArray(node.path) || node.path.length < 2) continue
    for (const ancestorId of node.path.slice(0, -1)) {
      counts.set(ancestorId, (counts.get(ancestorId) ?? 0) + 1)
    }
  }
  return counts
}

export function getVisibleAffiliateHierarchyNodes(
  nodes: AffiliateHierarchyNode[],
  collapsedIds: ReadonlySet<number>,
  sortState: AffiliateHierarchySortState,
): AffiliateHierarchyNode[] {
  if (nodes.length === 0) return []

  const sourceIndex = new Map<number, number>()
  const byId = new Map<number, AffiliateHierarchyNode>()
  nodes.forEach((node, index) => {
    sourceIndex.set(node.user_id, index)
    byId.set(node.user_id, node)
  })

  const childrenByParent = new Map<number, AffiliateHierarchyNode[]>()
  const roots: AffiliateHierarchyNode[] = []

  for (const node of nodes) {
    const parentId = node.inviter_id ?? null
    if (parentId !== null && byId.has(parentId)) {
      const children = childrenByParent.get(parentId) ?? []
      children.push(node)
      childrenByParent.set(parentId, children)
    } else {
      roots.push(node)
    }
  }

  const compareNodes = (a: AffiliateHierarchyNode, b: AffiliateHierarchyNode): number => {
    if (sortState.key) {
      const compared = compareValues(nodeSortValue(a, sortState.key), nodeSortValue(b, sortState.key))
      if (compared !== 0) return sortState.order === 'asc' ? compared : -compared
    }
    return (sourceIndex.get(a.user_id) ?? 0) - (sourceIndex.get(b.user_id) ?? 0)
  }

  const sortSiblings = (items: AffiliateHierarchyNode[]): AffiliateHierarchyNode[] => {
    if (!sortState.key) return items
    return [...items].sort(compareNodes)
  }

  const flattened: AffiliateHierarchyNode[] = []
  const walk = (node: AffiliateHierarchyNode) => {
    flattened.push(node)
    if (collapsedIds.has(node.user_id)) return
    for (const child of sortSiblings(childrenByParent.get(node.user_id) ?? [])) {
      walk(child)
    }
  }

  for (const root of sortSiblings(roots)) {
    walk(root)
  }

  return flattened.filter((node) => !isHiddenByCollapsedAncestor(node, collapsedIds))
}
