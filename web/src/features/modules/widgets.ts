import type { ComponentType } from 'react'
import { ClockWidget } from './clock/Widget.tsx'
import { WeatherWidget } from './weather/Widget.tsx'
import { WifiWidget } from './wifi/Widget.tsx'

export type Region = 'topLeft' | 'topRight' | 'middle' | 'bottomLeft' | 'bottomRight'

export type WidgetProps = { region: Region }

/** Module key -> dashboard widget. Keys the server does not offer (yet) are simply absent. */
export const widgets: Record<string, ComponentType<WidgetProps>> = {
  clock: ClockWidget,
  weather: WeatherWidget,
  wifi: WifiWidget,
}
