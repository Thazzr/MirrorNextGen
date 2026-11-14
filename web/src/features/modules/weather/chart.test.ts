import { describe, expect, it } from 'vitest'
import { buildSmoothPath, chartX, chartXPercent } from './chart.ts'

type P = { x: number; y: number }

/** Parses "M x,y C c1 c2 end C …" into the start point and the segments. */
function parse(path: string) {
  const nums = [...path.matchAll(/(-?\d+(?:\.\d+)?(?:e-?\d+)?),(-?\d+(?:\.\d+)?(?:e-?\d+)?)/g)].map((m): P => ({ x: Number(m[1]), y: Number(m[2]) }))
  const start = nums[0]
  const segments: { c1: P; c2: P; end: P }[] = []
  for (let i = 1; i + 2 < nums.length; i += 3) segments.push({ c1: nums[i], c2: nums[i + 1], end: nums[i + 2] })
  return { start, segments }
}

const wave: P[] = [0, 1, 2, 3, 4].map((i) => ({ x: i * 10, y: [100, 40, 90, 30, 80][i] }))

describe('buildSmoothPath', () => {
  it('returns an empty path for fewer than two points', () => {
    expect(buildSmoothPath([])).toBe('')
    expect(buildSmoothPath([{ x: 0, y: 0 }])).toBe('')
  })

  it('starts at the first point, ends at the last and passes through every point', () => {
    const { start, segments } = parse(buildSmoothPath(wave))
    expect(start).toEqual(wave[0])
    expect(segments.map((s) => s.end)).toEqual(wave.slice(1))
  })

  it('is C1-continuous at interior points: equal, collinear control vectors', () => {
    const { segments } = parse(buildSmoothPath(wave))
    for (let i = 0; i < segments.length - 1; i++) {
      const knot = segments[i].end
      const incoming = { x: knot.x - segments[i].c2.x, y: knot.y - segments[i].c2.y }
      const outgoing = { x: segments[i + 1].c1.x - knot.x, y: segments[i + 1].c1.y - knot.y }
      expect(outgoing.x).toBeCloseTo(incoming.x, 9)
      expect(outgoing.y).toBeCloseTo(incoming.y, 9)
    }
  })

  it('never produces NaN, also for coincident x values', () => {
    expect(buildSmoothPath(wave)).not.toContain('NaN')
    expect(buildSmoothPath([{ x: 5, y: 1 }, { x: 5, y: 2 }, { x: 5, y: 3 }])).not.toContain('NaN')
  })

  it('clamps control-point y values to the bounds', () => {
    const spiky: P[] = [{ x: 0, y: 0 }, { x: 10, y: 100 }, { x: 20, y: 100 }, { x: 30, y: 0 }]
    const unclamped = parse(buildSmoothPath(spiky)).segments.flatMap((s) => [s.c1.y, s.c2.y])
    expect(Math.max(...unclamped)).toBeGreaterThan(100)
    const bounded = parse(buildSmoothPath(spiky, { minY: 0, maxY: 100 })).segments.flatMap((s) => [s.c1.y, s.c2.y])
    expect(Math.min(...bounded)).toBeGreaterThanOrEqual(0)
    expect(Math.max(...bounded)).toBeLessThanOrEqual(100)
  })

  it('draws a valid single cubic through two points', () => {
    const { start, segments } = parse(buildSmoothPath([{ x: 0, y: 0 }, { x: 30, y: 60 }]))
    expect(start).toEqual({ x: 0, y: 0 })
    expect(segments).toEqual([{ c1: { x: 5, y: 10 }, c2: { x: 25, y: 50 }, end: { x: 30, y: 60 } }])
  })
})

describe('chartX', () => {
  it('spreads points over the measured width', () => {
    expect(chartX(0, 9, 1000)).toBeCloseTo(80)
    expect(chartX(8, 9, 1000)).toBeCloseTo(930)
    expect(chartX(8, 9, 500)).toBeCloseTo(465)
  })

  it('gives the same position in percent of the width', () => {
    expect(chartXPercent(8, 9, 760)).toBeCloseTo(93)
    expect(chartXPercent(0, 9, 760)).toBeCloseTo(8)
  })
})
