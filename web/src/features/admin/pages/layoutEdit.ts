import type { Layout, Region } from '../api/layout.ts'
import { regions } from '../api/layout.ts'

/** Pure edit operations on the layout; each returns a new layout. */

export function move(layout: Layout, region: Region, index: number, delta: -1 | 1): Layout {
  const list = [...layout[region]]
  const target = index + delta
  if (target < 0 || target >= list.length) return layout
  ;[list[index], list[target]] = [list[target], list[index]]
  return { ...layout, [region]: list }
}

export function remove(layout: Layout, region: Region, index: number): Layout {
  return { ...layout, [region]: layout[region].filter((_, i) => i !== index) }
}

export function moveToRegion(layout: Layout, from: Region, index: number, to: Region): Layout {
  if (from === to) return layout
  const key = layout[from][index]
  return { ...remove(layout, from, index), [to]: [...layout[to], key] }
}

export function add(layout: Layout, region: Region, key: string): Layout {
  return { ...layout, [region]: [...layout[region], key] }
}

export function placedKeys(layout: Layout): string[] {
  return regions.flatMap((r) => layout[r.key])
}

export function sameLayout(a: Layout, b: Layout): boolean {
  return regions.every((r) => a[r.key].length === b[r.key].length && a[r.key].every((k, i) => k === b[r.key][i]))
}

/** Server messages that belong to a region (`topLeft`, or an indexed key such as `topLeft[2]`). */
export function regionErrors(errors: Record<string, string>, region: Region): string[] {
  return Object.entries(errors)
    .filter(([key]) => key === region || key.startsWith(`${region}[`) || key.startsWith(`${region}.`))
    .map(([, message]) => message)
}
