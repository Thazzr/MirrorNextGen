export type ChartPoint = { x: number; y: number }

export const chartLeft = 80
export const chartWidth = 850

export function chartX(index: number, pointCount: number) {
  return chartLeft + (index * chartWidth) / (pointCount - 1 || 1)
}

export function chartXPercent(index: number, pointCount: number) {
  return chartX(index, pointCount) / 10
}

/** Cubic Bezier path through the points that never overshoots between neighbours (monotone cubic interpolation). */
export function buildMonotonePath(points: ChartPoint[]): string {
  if (points.length < 2) return ''
  const slopes = points.slice(0, -1).map((point, index) => (points[index + 1].y - point.y) / (points[index + 1].x - point.x || 1))
  const tangents = points.map((_, index) =>
    index === 0
      ? slopes[0]
      : index === points.length - 1
        ? slopes[slopes.length - 1]
        : slopes[index - 1] * slopes[index] <= 0
          ? 0
          : (slopes[index - 1] + slopes[index]) / 2,
  )
  for (let index = 0; index < slopes.length; index++) {
    const first = tangents[index] / slopes[index]
    const second = tangents[index + 1] / slopes[index]
    const length = first * first + second * second
    if (length > 9) {
      const scale = 3 / Math.sqrt(length)
      tangents[index] = scale * first * slopes[index]
      tangents[index + 1] = scale * second * slopes[index]
    }
  }
  return points.slice(1).reduce((path, current, index) => {
    const previous = points[index]
    const width = current.x - previous.x
    return `${path} C ${previous.x + width / 3},${previous.y + (tangents[index] * width) / 3} ${current.x - width / 3},${current.y - (tangents[index + 1] * width) / 3} ${current.x},${current.y}`
  }, `M ${points[0].x},${points[0].y}`)
}
