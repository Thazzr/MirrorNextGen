import { describe, expect, it } from 'vitest'
import { statusText } from './displayForm.ts'

const base = {
  powerOn: true,
  reason: 'schedule',
  nextSwitchOn: false,
  timeZone: 'Europe/Berlin',
} as const

describe('statusText', () => {
  it('names no day when the switch is the same day in the mirror zone, although UTC differs', () => {
    // 22:30 UTC on 06-09 and 04:00 UTC on 06-10 are two UTC days, but both 06-10 in Berlin.
    const text = statusText({
      ...base,
      now: '2026-06-10T00:30:00+02:00',
      nextSwitchAt: '2026-06-10T06:00:00+02:00',
      nextSwitchOn: true,
    })
    expect(text).toBe('Monitor laut Zeitplan an · nächste Schaltung 06:00 (an)')
  })

  it('says "morgen" when the switch is the next day in the mirror zone, although UTC is the same day', () => {
    // 21:30 UTC and 23:00 UTC on 06-10 share a UTC day, but Berlin is already on 06-11.
    const text = statusText({
      ...base,
      now: '2026-06-10T23:30:00+02:00',
      nextSwitchAt: '2026-06-11T01:00:00+02:00',
    })
    expect(text).toBe('Monitor laut Zeitplan an · nächste Schaltung morgen 01:00 (aus)')
  })
})
