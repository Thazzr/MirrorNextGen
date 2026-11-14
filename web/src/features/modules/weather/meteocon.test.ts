import { describe, expect, it } from 'vitest'
import { getMeteocon } from './meteocon.ts'

describe('getMeteocon', () => {
  it.each([
    ['10d', 'Regen', 'animated', '/assets/weather/meteocons/animated/rain.svg'],
    ['01n', 'Klar', 'monochrome', '/assets/weather/meteocons/monochrome/clear-night.svg'],
    ['11d', 'Gewitter', 'animated', '/assets/weather/meteocons/animated/thunderstorms-day-rain.svg'],
    [undefined, 'Nebel', 'animated', '/assets/weather/meteocons/animated/fog-day.svg'],
    ['xx', 'Leichter Schneefall', 'monochrome', '/assets/weather/meteocons/monochrome/snow.svg'],
  ] as const)('maps %s / %s', (code, condition, variant, expected) => {
    expect(getMeteocon(code, condition, variant)).toBe(expected)
  })
})
