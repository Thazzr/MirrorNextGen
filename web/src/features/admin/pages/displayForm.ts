import type { DisplaySettings } from '../api/display.ts'

/** Names of the areas of the display form whose values differ from the stored ones. */
export function changedAreas(values: DisplaySettings, saved: DisplaySettings): string[] {
  const areas: string[] = []
  if (values.rotation !== saved.rotation) areas.push('Drehung')
  if (values.output.trim() !== saved.output) areas.push('Monitor-Ausgang')
  if (values.testPattern !== saved.testPattern) areas.push('Testbild')
  return areas
}
