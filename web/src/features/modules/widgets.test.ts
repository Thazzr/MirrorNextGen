import { describe, expect, it } from 'vitest'
import { resolveWidget, widgets } from './widgets.ts'

describe('resolveWidget', () => {
  it('resolves exact keys', () => {
    expect(resolveWidget('clock')?.Widget).toBe(widgets.clock)
    expect(resolveWidget('clock')?.instanceId).toBeUndefined()
  })

  it('resolves prefix keys with the instance id after the prefix', () => {
    expect(resolveWidget('calendar:abc')?.instanceId).toBe('abc')
    expect(resolveWidget('calendar:a:b')?.instanceId).toBe('a:b')
  })

  it('rejects look-alikes, empty ids and inherited names', () => {
    for (const key of ['calendarx', 'calendar', 'calendar:', 'toString', 'constructor', 'xcalendar:abc']) {
      expect(resolveWidget(key)).toBeUndefined()
    }
  })
})
