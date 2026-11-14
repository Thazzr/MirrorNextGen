/// <reference types="node" />
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const read = (rel: string) => readFileSync(new URL(rel, import.meta.url), 'utf8')

describe('no zoom on phones', () => {
  it('locks the viewport scale', () => {
    const html = read('../../index.html')
    const content = /<meta name="viewport" content="([^"]*)"/.exec(html)?.[1] ?? ''
    expect(content).toContain('maximum-scale=1')
    expect(content).toContain('user-scalable=no')
  })

  it('disables double-tap zoom in the shared base stylesheet', () => {
    expect(read('./base.css')).toContain('touch-action: manipulation')
  })

  it('raises admin inputs to 16px on narrow screens, and the admin chunk loads that file', () => {
    const css = read('../features/admin/admin.css')
    expect(css).toMatch(/@media \(max-width: 48em\)\s*\{[\s\S]*font-size: 16px/)
    expect(read('../features/admin/AdminApp.tsx')).toContain("import './admin.css'")
  })
})
