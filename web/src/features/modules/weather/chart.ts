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
