import { describe, expect, it } from 'vitest'
import { buildMonotonePath, chartX } from './chart.ts'

function coords(path: string) {
  return [...path.matchAll(/(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)/g)].map((m) => ({ x: Number(m[1]), y: Number(m[2]) }))
}

describe('buildMonotonePath', () => {
  it('returns an empty path for fewer than two points', () => {
    expect(buildMonotonePath([])).toBe('')
    expect(buildMonotonePath([{ x: 0, y: 0 }])).toBe('')
  })

  it('draws a straight cubic through two points', () => {
    expect(buildMonotonePath([{ x: 0, y: 0 }, { x: 30, y: 60 }])).toBe('M 0,0 C 10,20 20,40 30,60')
  })

  it('keeps a flat segment flat', () => {
    const path = buildMonotonePath([{ x: 0, y: 10 }, { x: 10, y: 10 }, { x: 20, y: 40 }])
    // First segment: both control points on y = 10.
    expect(path).toContain('C 3.3333333333333335,10 6.666666666666666,10 10,10')
  })

  it('does not overshoot at a local extremum', () => {
    const points = [{ x: 0, y: 0 }, { x: 10, y: 50 }, { x: 20, y: 0 }]
    const ys = coords(buildMonotonePath(points)).map((p) => p.y)
    expect(Math.min(...ys)).toBeGreaterThanOrEqual(0)
    expect(Math.max(...ys)).toBeLessThanOrEqual(50)
  })

  it('clamps steep tangents so a control point stays between its neighbours', () => {
    const points = [{ x: 0, y: 0 }, { x: 10, y: 1 }, { x: 20, y: 100 }]
    const control = coords(buildMonotonePath(points)).slice(1, 3)
    for (const c of control) {
      expect(c.y).toBeGreaterThanOrEqual(-1e-9)
      expect(c.y).toBeLessThanOrEqual(1 + 1e-9)
    }
  })
})

describe('chartX', () => {
  it('spreads points over the chart width', () => {
    expect(chartX(0, 9)).toBe(80)
    expect(chartX(8, 9)).toBe(930)
  })
})
