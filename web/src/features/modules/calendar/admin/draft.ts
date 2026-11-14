import type {
  CalendarRule,
  CalendarSection,
  CalendarSettings,
  CalendarSettingsUpdate,
  CalendarShift,
  CalendarSourceUpdate,
} from './api.ts'

export type TabKey = 'calendars' | 'sections' | 'rules' | 'preview'

export const TAB_LABEL: Record<Exclude<TabKey, 'preview'>, string> = {
  calendars: 'Kalender',
  sections: 'Bereiche',
  rules: 'Regeln',
}

/** Eight lowercase hex characters; matches the server's id pattern `^[a-z0-9-]{1,40}$`. */
export function newId(): string {
  const bytes = new Uint8Array(4)
  crypto.getRandomValues(bytes)
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
}

/** The write model of stored settings: no addresses, the server keeps them. */
export function toDraft(saved: CalendarSettings): CalendarSettingsUpdate {
  return {
    refreshMinutes: saved.refreshMinutes,
    calendars: saved.calendars.map((c) => ({ id: c.id, name: c.name, icon: c.icon })),
    sections: saved.sections,
    rules: saved.rules,
  }
}

/** Drops empty addresses: an absent `url` keeps the stored one. */
export function toBody(draft: CalendarSettingsUpdate): CalendarSettingsUpdate {
  return {
    ...draft,
    calendars: draft.calendars.map((c): CalendarSourceUpdate => {
      const { url, ...rest } = c
      return url ? { ...rest, url } : rest
    }),
  }
}

export function changedTabs(draft: CalendarSettingsUpdate, saved: CalendarSettings): Array<Exclude<TabKey, 'preview'>> {
  const base = toDraft(saved)
  const body = toBody(draft)
  const tabs: Array<Exclude<TabKey, 'preview'>> = []
  if (JSON.stringify(body.calendars) !== JSON.stringify(base.calendars)) tabs.push('calendars')
  if (JSON.stringify(body.sections) !== JSON.stringify(base.sections)) tabs.push('sections')
  if (JSON.stringify(body.rules) !== JSON.stringify(base.rules)) tabs.push('rules')
  return tabs
}

/** Which tab a server field path belongs to. */
export function tabOfError(path: string): Exclude<TabKey, 'preview'> | undefined {
  if (path.startsWith('calendars')) return 'calendars'
  if (path.startsWith('sections')) return 'sections'
  if (path.startsWith('rules')) return 'rules'
  return undefined
}

export const newCalendar = (): CalendarSourceUpdate => ({ id: newId(), name: '', icon: { kind: 'none' }, url: '' })

export const newSection = (): CalendarSection => ({
  id: newId(),
  title: '',
  type: 'list',
  calendarIds: [],
  maxEvents: 8,
  lookAheadDays: 14,
  shiftDays: 5,
  showFreeDays: false,
  shifts: [],
})

export const newShift = (): CalendarShift => ({
  id: newId(),
  match: '',
  matchMode: 'equals',
  label: '',
  start: '',
  end: '',
  icon: { kind: 'none' },
})

export const newRule = (): CalendarRule => ({
  id: newId(),
  calendarIds: [],
  matchMode: 'contains',
  text: '',
  action: 'hide',
  icon: { kind: 'none' },
  renameTo: '',
})

export function move<T>(items: T[], index: number, delta: -1 | 1): T[] {
  const target = index + delta
  if (target < 0 || target >= items.length) return items
  const next = [...items]
  ;[next[index], next[target]] = [next[target], next[index]]
  return next
}

export function replaceAt<T>(items: T[], index: number, value: T): T[] {
  return items.map((item, i) => (i === index ? value : item))
}

export const removeAt = <T,>(items: T[], index: number): T[] => items.filter((_, i) => i !== index)
