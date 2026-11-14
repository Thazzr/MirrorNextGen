export type MeteoconVariant = 'animated' | 'monochrome'

const codeToIcon: Record<string, string> = {
  '01d': 'clear-day', '01n': 'clear-night',
  '02d': 'partly-cloudy-day', '02n': 'partly-cloudy-night',
  '03d': 'cloudy', '03n': 'cloudy',
  '04d': 'overcast', '04n': 'overcast',
  '09d': 'rain', '09n': 'rain', '10d': 'rain', '10n': 'rain',
  '11d': 'thunderstorms-day-rain', '11n': 'thunderstorms-night-rain',
  '13d': 'snow', '13n': 'snow', '50d': 'fog-day', '50n': 'fog-night',
}

export function getMeteocon(iconCode: string | undefined, condition: string, variant: MeteoconVariant) {
  const code = iconCode?.toLowerCase()
  const name = code && codeToIcon[code] ? codeToIcon[code] : fallbackIcon(condition, code?.endsWith('n') ?? false)
  return `/assets/weather/meteocons/${variant}/${name}.svg`
}

function fallbackIcon(condition: string, night: boolean) {
  const text = condition.toLowerCase()
  if (text.includes('gewitter')) return night ? 'thunderstorms-night-rain' : 'thunderstorms-day-rain'
  if (text.includes('schnee')) return 'snow'
  if (text.includes('nebel') || text.includes('dunst')) return night ? 'fog-night' : 'fog-day'
  if (text.includes('regen') || text.includes('schauer') || text.includes('niesel')) return 'rain'
  if (text.includes('bedeckt')) return 'overcast'
  if (text.includes('bewölkt') || text.includes('bewolkt') || text.includes('wolke')) return night ? 'partly-cloudy-night' : 'partly-cloudy-day'
  return night ? 'clear-night' : 'clear-day'
}
