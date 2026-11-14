import type { components } from '../shared/api/schema.gen.ts'

export type Layout = components['schemas']['Layout']
export type WeatherData = components['schemas']['WeatherData']

export function layoutOf(parts: Partial<Layout> = {}): Layout {
  return { topLeft: [], topRight: [], middle: [], bottomLeft: [], bottomRight: [], ...parts }
}

/** Fixed system time of the weather tests is 2026-11-14T08:05:07Z (09:05 in Berlin). */
export function weatherData(parts: Partial<WeatherData> = {}): WeatherData {
  const hourly = [0, 3, 6, 9, 12, 15, 18, 21, 24].map((hour, index) => ({
    time: new Date(Date.UTC(2026, 10, 13, 23 + hour)).toISOString(),
    temperatureC: 5 + (index % 4),
    feelsLikeC: 4,
    condition: 'Leichter Regen',
    iconCode: '10d',
    precipitationProbability: index === 3 ? 40 : 0,
    windKph: 10,
    gustKph: 15,
    precipitationMm: 0,
  }))
  return {
    location: 'Berlin',
    updatedAt: '2026-11-14T08:00:00Z',
    temperatureC: 17,
    feelsLikeC: 5,
    humidity: 80,
    condition: 'Leichter Regen',
    iconCode: '10d',
    showIcons: true,
    windKph: 12,
    precipitationProbability: 40,
    hourly,
    forecast: [
      { date: '2026-11-15', condition: 'Bewölkt', iconCode: '04d', lowC: 2, highC: 8, precipitationProbability: 10, windKph: 14, precipitationMm: 0 },
      { date: '2026-11-16', condition: 'Regen', iconCode: '10d', lowC: 3, highC: 9, precipitationProbability: 80, windKph: 20, precipitationMm: 2.4 },
      { date: '2026-11-17', condition: 'Klar', iconCode: '01d', lowC: 1, highC: 6, precipitationProbability: 0, windKph: 8, precipitationMm: 0 },
    ],
    ...parts,
  }
}
