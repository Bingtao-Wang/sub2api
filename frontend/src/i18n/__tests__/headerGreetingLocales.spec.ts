import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

const greetingKeys = ['earlyMorning', 'morning', 'noon', 'afternoon', 'evening', 'lateNight'] as const

describe('header greeting locale keys', () => {
  it.each(greetingKeys)('contains the %s greeting in both locales', (key) => {
    expect(zh.common.headerGreeting[key]).toContain('{name}')
    expect(en.common.headerGreeting[key]).toContain('{name}')
  })

  it('keeps the requested Chinese late-night care message', () => {
    expect(zh.common.headerGreeting.lateNight).toBe('{name} 夜深了，辛苦了。喝口水，早点休息！加油！')
  })
})
