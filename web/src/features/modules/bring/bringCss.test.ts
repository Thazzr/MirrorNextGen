/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { columns } from './Widget.tsx'

// jsdom applies no layout, so the two rules the widget relies on are read from the stylesheet itself (vitest
// drops CSS imports, so it is read with fs).
const css = readFileSync(resolve(import.meta.dirname, 'bring.css'), 'utf-8')

function declarations(selector: string) {
  const escaped = selector.replace(/[.\\]/g, '\\$&')
  const match = new RegExp(`(?:^|\\n)${escaped}\\s*\\{([^}]*)\\}`).exec(css)
  expect(match, `rule ${selector} exists`).not.toBeNull()
  return match![1]
}

describe('bring.css', () => {
  it('stacks the rows upwards so the first row sits at the bottom edge', () => {
    expect(declarations('.mirror-bring__rows')).toMatch(/flex-direction:\s*column-reverse\s*;/)
  })

  it('lays the list out in as many columns as the widget slices rows of', () => {
    const match = /grid-template-columns:\s*repeat\((\d+),/.exec(declarations('.mirror-bring__list'))
    expect(match).not.toBeNull()
    expect(Number(match![1])).toBe(columns)
  })
})
