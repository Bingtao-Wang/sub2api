import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const customFeatureSources = [
  'src/components/keys/UseKeyModal.vue',
  'src/components/layout/AppHeader.vue',
  'src/components/layout/AppSidebar.vue',
  'src/components/payment/providerConfig.ts',
  'src/router/index.ts',
  'src/views/admin/affiliates/AdminAffiliateHierarchyView.vue',
  'src/views/user/AffiliateHierarchyView.vue',
  'src/views/user/UsageTutorialView.vue',
]

const staticKeyPatterns = [
  /(?:\bt|\$t)\(\s*['"]([^'"]+)['"]/g,
  /(?:titleKey|descriptionKey|hintKey):\s*['"]([^'"]+)['"]/g,
]

const dynamicCustomKeys = [
  'admin.settings.payment.field_queryUrl',
  'affiliate.hierarchy.errors.AFFILIATE_AGENT_ACCESS_DENIED',
  'affiliate.hierarchy.errors.AFFILIATE_DISABLED',
  'admin.affiliates.hierarchy.errors.AFFILIATE_RATE_EXCEEDS_CAP',
  'admin.affiliates.hierarchy.errors.INVALID_RATE',
]

function collectStaticKeys(file: string): string[] {
  const source = readFileSync(resolve(process.cwd(), file), 'utf8')
  const keys = new Set<string>()

  for (const pattern of staticKeyPatterns) {
    pattern.lastIndex = 0
    for (let match = pattern.exec(source); match; match = pattern.exec(source)) {
      keys.add(match[1])
    }
  }

  return [...keys].sort()
}

function hasLocaleKey(messages: Record<string, unknown>, key: string): boolean {
  let current: unknown = messages
  for (const segment of key.split('.')) {
    if (!current || typeof current !== 'object' || !(segment in current)) return false
    current = (current as Record<string, unknown>)[segment]
  }
  return typeof current === 'string' || (current !== null && typeof current === 'object')
}

describe.each([
  ['zh', zh],
  ['en', en],
] as const)('custom feature locale completeness: %s', (_locale, messages) => {
  it('defines every statically referenced key', () => {
    const missing = customFeatureSources.flatMap(file =>
      collectStaticKeys(file)
        .filter(key => !hasLocaleKey(messages, key))
        .map(key => `${file}: ${key}`),
    )

    missing.push(...dynamicCustomKeys
      .filter(key => !hasLocaleKey(messages, key))
      .map(key => `dynamic custom key: ${key}`))

    expect(missing).toEqual([])
  })
})
