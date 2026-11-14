import type { DisplaySettings, DisplayStatus } from '../api/display.ts'

/** Names of the areas of the display form whose values differ from the stored ones. */
export function changedAreas(values: DisplaySettings, saved: DisplaySettings): string[] {
  const areas: string[] = []
  if (values.rotation !== saved.rotation) areas.push('Drehung')
  if (values.output.trim() !== saved.output) areas.push('Monitor-Ausgang')
  if (values.testPattern !== saved.testPattern) areas.push('Testbild')
  const a = values.schedule
  const b = saved.schedule
  if (a.enabled !== b.enabled || a.offTime !== b.offTime || a.onTime !== b.onTime) areas.push('Nachtruhe')
  return areas
}

function clock(iso: string, timeZone: string): string {
  return new Intl.DateTimeFormat('de-DE', { hour: '2-digit', minute: '2-digit', timeZone }).format(new Date(iso))
}

function day(iso: string, timeZone: string): string {
  return new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone }).format(new Date(iso))
}

/** One line about the monitor, in the mirror's time zone: why it is on or off and the next switch. */
export function statusText(status: DisplayStatus): string {
  if (status.reason === 'test') return 'Monitor aus (Test)'
  if (status.reason === 'always' || status.nextSwitchAt === undefined) return 'Monitor an · keine Nachtruhe'
  const { timeZone } = status
  const when = clock(status.nextSwitchAt, timeZone)
  const tomorrow = day(status.nextSwitchAt, timeZone) !== day(status.now, timeZone)
  const next = `${tomorrow ? 'morgen ' : ''}${when} (${status.nextSwitchOn ? 'an' : 'aus'})`
  return `Monitor laut Zeitplan ${status.powerOn ? 'an' : 'aus'} · nächste Schaltung ${next}`
}
