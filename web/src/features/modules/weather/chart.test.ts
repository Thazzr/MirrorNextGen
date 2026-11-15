import { describe, expect, it } from 'vitest'
import { buildSmoothPath, chartX, chartXPercent, temperatureColor, temperatureStops, valueRange } from './chart.ts'

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

describe('temperatureColor', () => {
  it('clamps below 0 °C and above 28 °C', () => {
    expect(temperatureColor(-5)).toBe('#7fc8ff')
    expect(temperatureColor(0)).toBe('#7fc8ff')
    expect(temperatureColor(28)).toBe('#ff6b5c')
    expect(temperatureColor(35)).toBe('#ff6b5c')
  })

  it('hits the middle stops exactly', () => {
    expect(temperatureColor(10)).toBe('#ffffff')
    expect(temperatureColor(20)).toBe('#ffb35c')
  })

  it('interpolates linearly in RGB between stops', () => {
    // Midpoint of #7fc8ff and #ffffff: (127+255)/2 = 191, (200+255)/2 = 227.5 -> 228.
    expect(temperatureColor(5)).toBe('#bfe4ff')
  })
})

describe('temperatureStops', () => {
  it('has both ends plus the 0 °C threshold inside -3..5', () => {
    expect(temperatureStops(-3, 5)).toEqual([
      { offset: 0, color: temperatureColor(-3) },
      { offset: 3 / 8, color: temperatureColor(0) },
      { offset: 1, color: temperatureColor(5) },
    ])
  })

  it('has only the 20 °C threshold inside 12..25', () => {
    const stops = temperatureStops(12, 25)
    expect(stops).toHaveLength(3)
    expect(stops[1]).toEqual({ offset: 8 / 13, color: '#ffb35c' })
    expect(stops[0].color).toBe(temperatureColor(12))
    expect(stops[2].color).toBe(temperatureColor(25))
  })

  it('has just the two end stops when no threshold lies inside', () => {
    expect(temperatureStops(2, 8)).toEqual([
      { offset: 0, color: temperatureColor(2) },
      { offset: 1, color: temperatureColor(8) },
    ])
  })

  it('stays valid when low equals high', () => {
    const stops = temperatureStops(10, 10)
    expect(stops).toEqual([
      { offset: 0, color: '#ffffff' },
      { offset: 1, color: '#ffffff' },
    ])
  })
})

describe('valueRange', () => {
  it('pads a flat range symmetrically around its midpoint to the minimum span', () => {
    expect(valueRange([1, 2, 1], 10)).toEqual({ low: -3.5, high: 6.5 })
  })

  it('keeps a range that already spans the minimum', () => {
    expect(valueRange([-3, 7])).toEqual({ low: -3, high: 7 })
    expect(valueRange([-5, 20])).toEqual({ low: -5, high: 20 })
  })

  it('handles a single value without NaN', () => {
    const { low, high } = valueRange([5])
    expect(high - low).toBe(10)
    expect(low).toBe(0)
  })
})
