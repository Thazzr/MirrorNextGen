import { useId } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'
import { useElementWidth } from '../../../shared/useElementWidth.ts'
import type { WidgetProps } from '../widgets.ts'
import { buildSmoothPath, chartLayout, chartX, chartXPercent, temperatureStops, valueRange, type ChartPoint } from './chart.ts'
import { getMeteocon, type MeteoconVariant } from './meteocon.ts'
import './weather.css'

type WeatherData = components['schemas']['WeatherData']
type Hourly = components['schemas']['HourlyPoint']

const weekday = new Intl.DateTimeFormat('de-DE', { weekday: 'long' })
const clockTime = new Intl.DateTimeFormat('de-DE', { hour: '2-digit', minute: '2-digit' })

// Stamps the answer with the time it arrived: "past" chart points are judged against it, not against a clock read during render.
const stamp = (weather: components['schemas']['DashboardWeather']) => ({ weather, now: Date.now() })

export function WeatherWidget({ region }: WidgetProps) {
  const query = useQuery({
    queryKey: ['dashboard', 'weather'],
    queryFn: async () => unwrap(await api.GET('/api/v1/dashboard/weather')),
    refetchInterval: 60_000,
    select: stamp,
  })
  const weather = query.data?.weather
  const now = query.data?.now ?? 0
  // Left regions align the block to the left, right regions (and the middle) to the right like SmartMirror2.
  const side = region === 'topLeft' || region === 'bottomLeft' ? ' mirror-weather--left' : ''

  if (weather && !weather.configured) return null
  if (!weather) {
    return (
      <section className={`mirror-weather${side}`}>
        {query.isError ? (
          <div className="mirror-error">Wetterdaten sind nicht verfügbar.</div>
        ) : (
          <div className="mirror-muted">Wetterdaten werden geladen …</div>
        )}
      </section>
    )
  }
  return (
    <section className={`mirror-weather${side}`}>
      {weather.data ? (
        <>
          <Weather data={weather.data} now={now} />
          {weather.stale && <div className="mirror-weather__stale mirror-muted">Stand: {clockTime.format(new Date(weather.data.updatedAt))}</div>}
        </>
      ) : (
        <div className="mirror-error">{weather.error ?? 'Wetterdaten sind nicht verfügbar.'}</div>
      )}
    </section>
  )
}

function Weather({ data, now }: { data: WeatherData; now: number }) {
  return (
    <>
      <div className="mirror-weather__overview">
        <div className="mirror-weather__current">
          {data.showIcons && <WeatherIcon condition={data.condition} iconCode={data.iconCode} variant="animated" />}
          <span className="mirror-weather__temperature">{data.temperatureC}°</span>
        </div>
        <div className="mirror-weather__condition">{data.condition}</div>
        {data.alert && (
          <div className="mirror-weather__alert">
            <i className="bi bi-exclamation-triangle" aria-hidden="true" />
            {data.alert}
          </div>
        )}
      </div>
      <HourlySection entries={data.hourly} now={now} />
      <section className="mirror-weather__forecast">
        <h2>3 Tage Vorhersage</h2>
        {data.forecast.length > 0 && (
          <div className="mirror-weather__forecast-rows">
            {data.forecast.map((entry, index) => (
              <article className="mirror-weather__forecast-row" key={entry.date}>
                <span className="mirror-weather__forecast-day">{forecastDayLabel(entry.date, index, now)}</span>
                <div className="mirror-weather__forecast-main">
                  <WeatherIcon condition={entry.condition} iconCode={entry.iconCode} variant="monochrome" />
                  <strong>
                    {entry.highC}°<small>/ {entry.lowC}°</small>
                  </strong>
                </div>
                <span className="mirror-weather__forecast-condition">{entry.condition}</span>
                <footer>
                  <span>
                    <i className="bi bi-droplet" aria-hidden="true" /> {entry.precipitationProbability}%
                    {entry.precipitationMm > 0 && ` · ${formatPrecipitation(entry.precipitationMm)}`}
                  </span>
                  <span>
                    <i className="bi bi-wind" aria-hidden="true" /> {entry.windKph} km/h
                  </span>
                </footer>
              </article>
            ))}
          </div>
        )}
      </section>
    </>
  )
}

function isPast(time: string, now: number) {
  return new Date(time).getTime() < now
}

// Width used until the container has been measured (and where ResizeObserver is missing): about the widget's width at 1440 px.
const fallbackChartWidth = 760
const chartHeight = 290
// Rain columns: 100 % is this tall and they stand on the baseline at the bottom of the chart.
const rainMaxHeight = 75
const rainBaseline = 280
const rainColumnWidth = 28

function HourlySection({ entries, now }: { entries: Hourly[]; now: number }) {
  const [ref, width] = useElementWidth<HTMLElement>(fallbackChartWidth)
  return (
    <section className="mirror-weather__hourly" ref={ref}>
      <WeatherChart entries={entries} now={now} width={width} />
      <div className="mirror-weather__hour-labels">
        {entries.map((entry, index) => (
          <HourLabel key={entry.time} entry={entry} index={index} pointCount={entries.length} now={now} width={width} />
        ))}
      </div>
    </section>
  )
}

function HourLabel({ entry, index, pointCount, now, width }: { entry: Hourly; index: number; pointCount: number; now: number; width: number }) {
  const date = new Date(entry.time)
  // The chart ends at midnight: the last point reads 24:00 rather than 00:00.
  const label = index === pointCount - 1 && date.getHours() === 0 ? '24:00' : clockTime.format(date)
  return (
    <div className={isPast(entry.time, now) ? 'mirror-weather__hour-label--past' : undefined} style={{ left: `${chartXPercent(index, pointCount, width)}%` }}>
      <span>{label}</span>
      <WeatherIcon condition={entry.condition} iconCode={entry.iconCode} variant="monochrome" />
    </div>
  )
}

function forecastDayLabel(date: string, index: number, now: number) {
  const forecastDate = new Date(`${date}T12:00:00`)
  const tomorrow = new Date(now)
  tomorrow.setHours(0, 0, 0, 0)
  tomorrow.setDate(tomorrow.getDate() + 1)
  return index === 0 && forecastDate.toDateString() === tomorrow.toDateString() ? 'Morgen' : weekday.format(forecastDate)
}

function WeatherChart({ entries, now, width }: { entries: Hourly[]; now: number; width: number }) {
  const clipId = useId().replace(/[^a-zA-Z0-9_-]/g, '')
  if (entries.length < 2) return null
  const values = entries.map((entry) => entry.temperatureC)
  const { low, high } = valueRange(values)
  const { left, plotWidth } = chartLayout(width)
  const right = left + plotWidth
  // The curve lives in the upper part; the lower ~90 px belong to the rain columns and their labels.
  const yTop = 45
  const yBottom = 175
  const xAt = (index: number) => chartX(index, entries.length, width)
  const point = (value: number, index: number): ChartPoint => ({ x: xAt(index), y: yBottom - ((value - low) / (high - low || 1)) * (yBottom - yTop) })
  const temperatures = entries.map((entry, index) => point(entry.temperatureC, index))
  const path = buildSmoothPath(temperatures, { minY: yTop, maxY: yBottom })
  const firstTime = new Date(entries[0].time).getTime()
  const lastTime = new Date(entries[entries.length - 1].time).getTime()
  const currentProgress = Math.max(0, Math.min(1, (now - firstTime) / (lastTime - firstTime || 1)))
  const currentX = left + plotWidth * currentProgress
  const first = temperatures[0]
  const last = temperatures[temperatures.length - 1]
  // Closed shape below the curve down to the bottom plot line; the gradient fades it out towards that line.
  const areaPath = `${path} L ${last.x},${yBottom} L ${first.x},${yBottom} Z`
  const stops = temperatureStops(low, high)
  const rainHeight = (entry: Hourly) => (Math.max(0, entry.precipitationProbability) * rainMaxHeight) / 100
  const pct = (x: number) => `${(x / width) * 100}%`
  // Text is HTML on top of the SVG: Chromium draws SVG text without the hinting and weight of HTML text.
  return (
    <div className="mirror-weather__chart-box" style={{ height: chartHeight }}>
      <svg
        className="mirror-weather__chart"
        viewBox={`0 0 ${width} ${chartHeight}`}
        width={width}
        height={chartHeight}
        aria-label="Temperatur- und Niederschlagsverlauf"
      >
        <defs>
          <clipPath id={`${clipId}-past`}>
            <rect x="-20" y="0" width={currentX + 20} height={chartHeight} />
          </clipPath>
          <clipPath id={`${clipId}-future`}>
            <rect x={currentX} y="0" width={width - currentX + 20} height={chartHeight} />
          </clipPath>
          <linearGradient id={`${clipId}-area`} gradientUnits="userSpaceOnUse" x1="0" y1={yTop} x2="0" y2={yBottom}>
            <stop offset="0" stopColor="#fff" stopOpacity="0.16" />
            <stop offset="1" stopColor="#fff" stopOpacity="0" />
          </linearGradient>
          {/* Same value-to-y mapping as the curve: the lowest value sits at yBottom, the highest at yTop. */}
          <linearGradient id={`${clipId}-temp`} gradientUnits="userSpaceOnUse" x1="0" y1={yBottom} x2="0" y2={yTop}>
            {stops.map((stop) => (
              <stop key={stop.offset} offset={stop.offset} stopColor={stop.color} />
            ))}
          </linearGradient>
        </defs>
        <line className="mirror-weather__chart-baseline" x1={left} y1={rainBaseline} x2={right} y2={rainBaseline} />
        <g className="mirror-weather__rain-bars">
          {entries.map((entry, index) => {
            const height = rainHeight(entry)
            const p = entry.precipitationProbability
            return height > 0 ? (
              <g className={isPast(entry.time, now) ? 'mirror-weather__chart-past' : undefined} key={entry.time}>
                <rect
                  className="mirror-weather__rain-col"
                  x={xAt(index) - rainColumnWidth / 2}
                  y={rainBaseline - height}
                  width={rainColumnWidth}
                  height={height}
                  rx="8"
                  fill="#5aa9ff"
                  opacity={Math.min(1, 0.35 + p / 100)}
                >
                  <title>{p}% Regenwahrscheinlichkeit</title>
                </rect>
              </g>
            ) : null
          })}
        </g>
        {/* The same curve twice, clipped at "now": the past part is dimmed and there is no join to kink. */}
        <g className="mirror-weather__chart-past" clipPath={`url(#${clipId}-past)`}>
          <path className="mirror-weather__chart-area" d={areaPath} fill={`url(#${clipId}-area)`} />
          <path className="mirror-weather__chart-temp" d={path} stroke={`url(#${clipId}-temp)`} />
        </g>
        <g clipPath={`url(#${clipId}-future)`}>
          <path className="mirror-weather__chart-area" d={areaPath} fill={`url(#${clipId}-area)`} />
          <path className="mirror-weather__chart-temp" d={path} stroke={`url(#${clipId}-temp)`} />
        </g>
        <line className="mirror-weather__chart-now" x1={currentX} y1="12" x2={currentX} y2={rainBaseline} />
        <circle className="mirror-weather__chart-now-marker" cx={currentX} cy={rainBaseline} r="3" />
        {temperatures.map((value, index) => (
          <g className={isPast(entries[index].time, now) ? 'mirror-weather__chart-past' : undefined} key={entries[index].time}>
            <circle className="mirror-weather__chart-point" cx={value.x} cy={value.y} r="4.5" stroke="#000" strokeWidth="2" />
          </g>
        ))}
      </svg>
      <div className="mirror-weather__chart-labels" aria-hidden="true">
        {entries.map((entry, index) => {
          const height = rainHeight(entry)
          return height > 0 ? (
            <span
              className={`mirror-weather__chart-rain${isPast(entry.time, now) ? ' mirror-weather__chart-past' : ''}`}
              key={`rain-${entry.time}`}
              style={{ left: pct(xAt(index)), top: rainBaseline - height - 4 }}
            >
              {entry.precipitationProbability}%
            </span>
          ) : null
        })}
        {temperatures.map((value, index) => (
          <span
            className={`mirror-weather__chart-value${isPast(entries[index].time, now) ? ' mirror-weather__chart-past' : ''}`}
            key={`value-${entries[index].time}`}
            style={{ left: pct(value.x), top: value.y - 9 }}
          >
            {entries[index].temperatureC}°
          </span>
        ))}
      </div>
    </div>
  )
}

function formatPrecipitation(mm: number) {
  return `${mm.toLocaleString('de-DE', { maximumFractionDigits: 1 })} mm`
}

function WeatherIcon({ condition, iconCode, variant }: { condition: string; iconCode?: string; variant: MeteoconVariant }) {
  return <img className={`mirror-weather__icon meteocon meteocon--${variant}`} src={getMeteocon(iconCode, condition, variant)} alt="" />
}
