import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type Layout = components['schemas']['Layout']
export type ModuleInfo = components['schemas']['ModuleInfo']
export type Region = keyof Layout

export const layoutQueryKey = ['settings', 'layout'] as const
export const modulesQueryKey = ['modules'] as const

/** Regions in the order of the sketch: top row, middle, bottom row. */
export const regions: { key: Region; label: string }[] = [
  { key: 'topLeft', label: 'Oben links' },
  { key: 'topRight', label: 'Oben rechts' },
  { key: 'middle', label: 'Mitte' },
  { key: 'bottomLeft', label: 'Unten links' },
  { key: 'bottomRight', label: 'Unten rechts' },
]

export async function fetchLayout(): Promise<Layout> {
  return unwrap(await api.GET('/api/v1/settings/layout'))
}

export async function saveLayout(body: Layout): Promise<Layout> {
  return unwrap(await api.PUT('/api/v1/settings/layout', { body }))
}

export async function fetchModules(): Promise<ModuleInfo[]> {
  return unwrap(await api.GET('/api/v1/modules'))
}
