import { useId } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'
import { useElementWidth } from '../../../shared/useElementWidth.ts'
import type { WidgetProps } from '../widgets.ts'
import { buildSmoothPath, chartLayout, chartX, chartXPercent, type ChartPoint } from './chart.ts'
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
const chartHeight = 265

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
  const low = Math.floor(Math.min(...values) / 5) * 5
  const high = Math.ceil(Math.max(...values) / 5) * 5 + 5
  const mid = Math.round((low + high) / 2)
  const { left, plotWidth } = chartLayout(width)
  const right = left + plotWidth
  const yTop = 50
  const yBottom = 210
  const xAt = (index: number) => chartX(index, entries.length, width)
  const point = (value: number, index: number): ChartPoint => ({ x: xAt(index), y: yBottom - ((value - low) / (high - low || 1)) * (yBottom - yTop) })
  const temperatures = entries.map((entry, index) => point(entry.temperatureC, index))
  const path = buildSmoothPath(temperatures, { minY: yTop, maxY: yBottom })
  const firstTime = new Date(entries[0].time).getTime()
  const lastTime = new Date(entries[entries.length - 1].time).getTime()
  const currentProgress = Math.max(0, Math.min(1, (now - firstTime) / (lastTime - firstTime || 1)))
  const currentX = left + plotWidth * currentProgress
  return (
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
      </defs>
      <line x1={left} y1={yBottom} x2={right} y2={yBottom} />
      <line x1={left} y1="130" x2={right} y2="130" />
      <line x1={left} y1={yTop} x2={right} y2={yTop} />
      <text className="mirror-weather__chart-scale" x="0" y="217">{low}°</text>
      <text className="mirror-weather__chart-scale" x="0" y="137">{mid}°</text>
      <text className="mirror-weather__chart-scale" x="0" y="57">{high}°</text>
      <g className="mirror-weather__rain-bars">
        {entries.map((entry, index) => {
          const height = Math.max(0, entry.precipitationProbability * 0.28)
          return height > 0 ? (
            <g className={isPast(entry.time, now) ? 'mirror-weather__chart-past' : undefined} key={entry.time}>
              <rect x={xAt(index) - 5} y={244 - height} width="10" height={height} rx="2">
                <title>{entry.precipitationProbability}% Regenwahrscheinlichkeit</title>
              </rect>
              <text x={xAt(index)} y={240 - height}>{entry.precipitationProbability}%</text>
            </g>
          ) : null
        })}
      </g>
      {/* The same curve twice, clipped at "now": the past part is dimmed and there is no join to kink. */}
      <g className="mirror-weather__chart-past" clipPath={`url(#${clipId}-past)`}>
        <path className="mirror-weather__chart-temp" d={path} />
      </g>
      <g clipPath={`url(#${clipId}-future)`}>
        <path className="mirror-weather__chart-temp" d={path} />
      </g>
      <line className="mirror-weather__chart-now" x1={currentX} y1="32" x2={currentX} y2="244" />
      <circle className="mirror-weather__chart-now-marker" cx={currentX} cy="244" r="3" />
      {temperatures.map((value, index) => (
        <g className={isPast(entries[index].time, now) ? 'mirror-weather__chart-past' : undefined} key={entries[index].time}>
          <circle cx={value.x} cy={value.y} r="4" />
          <text className="mirror-weather__chart-value" x={value.x} y={value.y - 11}>{entries[index].temperatureC}°</text>
        </g>
      ))}
    </svg>
  )
}

function formatPrecipitation(mm: number) {
  return `${mm.toLocaleString('de-DE', { maximumFractionDigits: 1 })} mm`
}

function WeatherIcon({ condition, iconCode, variant }: { condition: string; iconCode?: string; variant: MeteoconVariant }) {
  return <img className={`mirror-weather__icon meteocon meteocon--${variant}`} src={getMeteocon(iconCode, condition, variant)} alt="" />
}
