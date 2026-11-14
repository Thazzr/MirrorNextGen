import { describe, expect, it } from 'vitest'
import { isoWeek } from './clock.ts'

describe('isoWeek', () => {
  it.each([
    [2026, 12, 31, 53],
    [2027, 1, 1, 53],
    [2027, 1, 3, 53],
    [2027, 1, 4, 1],
    [2025, 12, 29, 1],
    [2024, 12, 30, 1],
    [2026, 11, 14, 46],
  ])('%i-%i-%i is week %i', (year, month, day, week) => {
    expect(isoWeek(new Date(year, month - 1, day, 12))).toBe(week)
  })
})
