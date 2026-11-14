import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type Logbook = components['schemas']['Logbook']
export type LogbookEntry = components['schemas']['LogbookEntry']
export type LogLevel = LogbookEntry['level']

export const logbookQueryKey = (minLevel: LogLevel) => ['logbook', minLevel] as const

export async function fetchLogbook(minLevel: LogLevel): Promise<Logbook> {
  return unwrap(await api.GET('/api/v1/logbook', { params: { query: { minLevel } } }))
}

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`

/** Tile text from the entries at warn level or above that the logbook holds. */
export function logbookLine(entries: Pick<LogbookEntry, 'level'>[]): string {
  const warnings = entries.filter((e) => e.level === 'warn').length
  const errors = entries.filter((e) => e.level === 'error').length
  const parts = []
  if (warnings > 0) parts.push(plural(warnings, 'Warnung', 'Warnungen'))
  if (errors > 0) parts.push(plural(errors, 'Fehler', 'Fehler'))
  return parts.length > 0 ? parts.join(' · ') : 'Keine Warnungen'
}
