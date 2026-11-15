export type ChartPoint = { x: number; y: number }

/** Horizontal geometry of the chart in CSS pixels: scale labels on the left, a small margin on the right. */
export function chartLayout(width: number) {
  const left = width * 0.08
  return { left, plotWidth: width * 0.85 }
}

export function chartX(index: number, pointCount: number, width: number) {
  const { left, plotWidth } = chartLayout(width)
  return left + (index * plotWidth) / (pointCount - 1 || 1)
}

export function chartXPercent(index: number, pointCount: number, width: number) {
  return (chartX(index, pointCount, width) / width) * 100
}

/**
 * One continuous path through all points: uniform Catmull-Rom (tension 0.5) converted to cubic Beziers, so the
 * tangent is shared by both neighbouring segments. Control-point y values are clamped to `bounds` so an overshoot
 * can never leave the plot area.
 */
export function buildSmoothPath(points: ChartPoint[], bounds?: { minY: number; maxY: number }): string {
  if (points.length < 2) return ''
  const clampY = (y: number) => (bounds ? Math.min(bounds.maxY, Math.max(bounds.minY, y)) : y)
  return points.slice(1).reduce((path, current, index) => {
    const previous = points[index]
    const before = points[index - 1] ?? previous
    const after = points[index + 2] ?? current
    const first = { x: previous.x + (current.x - before.x) / 6, y: clampY(previous.y + (current.y - before.y) / 6) }
    const second = { x: current.x - (after.x - previous.x) / 6, y: clampY(current.y - (after.y - previous.y) / 6) }
    return `${path} C ${first.x},${first.y} ${second.x},${second.y} ${current.x},${current.y}`
  }, `M ${points[0].x},${points[0].y}`)
}

/**
 * Value range the chart maps onto its plot height: the data's min/max, widened symmetrically around the midpoint
 * to at least `minSpan` degrees, so a flat day does not turn a 1 degree swing into a mountain.
 */
export function valueRange(values: number[], minSpan = 10): { low: number; high: number } {
  const min = Math.min(...values)
  const max = Math.max(...values)
  if (max - min >= minSpan) return { low: min, high: max }
  const mid = (min + max) / 2
  return { low: mid - minSpan / 2, high: mid + minSpan / 2 }
}

// Fixed thresholds (not the day's range), so 10 °C always looks the same on every day.
const temperatureScale: { c: number; rgb: [number, number, number] }[] = [
  { c: 0, rgb: [0x7f, 0xc8, 0xff] },
  { c: 10, rgb: [0xff, 0xff, 0xff] },
  { c: 20, rgb: [0xff, 0xb3, 0x5c] },
  { c: 28, rgb: [0xff, 0x6b, 0x5c] },
]

const hex = (n: number) => Math.round(n).toString(16).padStart(2, '0')

/** Colour of a temperature: piecewise linear in RGB between the fixed stops, clamped outside. */
export function temperatureColor(c: number): string {
  const first = temperatureScale[0]
  const last = temperatureScale[temperatureScale.length - 1]
  if (!(c > first.c)) return `#${first.rgb.map(hex).join('')}`
  if (c >= last.c) return `#${last.rgb.map(hex).join('')}`
  const upper = temperatureScale.findIndex((stop) => stop.c >= c)
  const a = temperatureScale[upper - 1]
  const b = temperatureScale[upper]
  const t = (c - a.c) / (b.c - a.c)
  return `#${a.rgb.map((v, i) => hex(v + (b.rgb[i] - v) * t)).join('')}`
}

/**
 * Gradient stops for a line running from `low` (offset 0) to `high` (offset 1): both ends plus one stop at every
 * threshold strictly between them.
 */
export function temperatureStops(low: number, high: number): { offset: number; color: string }[] {
  const span = high - low
  const inner = temperatureScale
    .filter((stop) => stop.c > low && stop.c < high)
    .map((stop) => ({ offset: (stop.c - low) / span, color: temperatureColor(stop.c) }))
  return [{ offset: 0, color: temperatureColor(low) }, ...inner, { offset: 1, color: temperatureColor(high) }]
}
