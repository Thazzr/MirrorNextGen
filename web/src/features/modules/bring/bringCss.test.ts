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

  it('never clips a tile, its icon or its texts', () => {
    const item = declarations('.mirror-bring__item')
    expect(item).not.toMatch(/aspect-ratio/)
    expect(item).not.toMatch(/overflow:\s*hidden/)
    expect(declarations('.mirror-bring__icon')).not.toMatch(/transform:/)
    expect(declarations('.mirror-bring__emoji')).not.toMatch(/transform:/)
    for (const selector of ['.mirror-bring__name, .mirror-bring__specification', '.mirror-bring__specification']) {
      const rule = declarations(selector)
      expect(rule).not.toMatch(/nowrap|text-overflow/)
    }
    expect(css).not.toMatch(/nowrap|text-overflow|aspect-ratio|(?<![-\w])transform:/)
  })

  it('is 880px wide at most', () => {
    expect(declarations('.mirror-bring')).toMatch(/width:\s*min\(880px,\s*62vw\)/)
  })

  it('breaks inside a word only for the last-resort name class and the specification', () => {
    for (const selector of ['.mirror-bring__item', '.mirror-bring__label', '.mirror-bring__list', '.mirror-bring__name']) {
      expect(declarations(selector), selector).not.toMatch(/break-word|anywhere|break-all/)
    }
    expect(declarations('.mirror-bring__name')).toMatch(/overflow-wrap:\s*normal/)
    expect(declarations('.mirror-bring__name')).toMatch(/word-break:\s*normal/)
    expect(declarations('.mirror-bring__name--break')).toMatch(/overflow-wrap:\s*anywhere/)
  })
})
