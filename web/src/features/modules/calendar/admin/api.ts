import { api, unwrap } from '../../../../shared/api/client.ts'
import type { components } from '../../../../shared/api/schema.gen.ts'

export type CalendarSettings = components['schemas']['CalendarSettings']
export type CalendarSettingsUpdate = components['schemas']['CalendarSettingsUpdate']
export type CalendarSourceUpdate = components['schemas']['CalendarSourceUpdate']
export type CalendarSection = components['schemas']['CalendarSection']
export type CalendarHighlight = components['schemas']['CalendarHighlight']
export type CalendarShift = components['schemas']['CalendarShift']
export type CalendarRule = components['schemas']['CalendarRule']
export type CalendarPreview = components['schemas']['CalendarPreview']

export const calendarSettingsQueryKey = ['settings', 'calendar'] as const
export const dashboardCalendarQueryKey = ['dashboard', 'calendar'] as const
export const dashboardLayoutQueryKey = ['dashboard', 'layout'] as const

export async function fetchCalendarSettings(): Promise<CalendarSettings> {
  return unwrap(await api.GET('/api/v1/settings/calendar'))
}

export async function saveCalendarSettings(body: CalendarSettingsUpdate): Promise<CalendarSettings> {
  return unwrap(await api.PUT('/api/v1/settings/calendar', { body }))
}

export async function previewCalendar(body: CalendarSettingsUpdate): Promise<CalendarPreview> {
  return unwrap(await api.POST('/api/v1/settings/calendar/preview', { body }))
}
