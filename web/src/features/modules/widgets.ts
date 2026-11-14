import type { ComponentType } from 'react'
import { BringWidget } from './bring/Widget.tsx'
import { CalendarWidget } from './calendar/Widget.tsx'
import { ClockWidget } from './clock/Widget.tsx'
import { WeatherWidget } from './weather/Widget.tsx'
import { WifiWidget } from './wifi/Widget.tsx'

export type Region = 'topLeft' | 'topRight' | 'middle' | 'bottomLeft' | 'bottomRight'

export type WidgetProps = { region: Region }

export type InstanceWidgetProps = WidgetProps & { instanceId: string }

/** Module key -> dashboard widget. Keys the server does not offer (yet) are simply absent. */
export const widgets: Record<string, ComponentType<WidgetProps>> = {
  clock: ClockWidget,
  weather: WeatherWidget,
  wifi: WifiWidget,
  bring: BringWidget,
}

/** Module key prefix -> widget of a module with several instances ("calendar:<id>"); the part after the prefix is the instance id. */
export const prefixWidgets: Record<string, ComponentType<InstanceWidgetProps>> = {
  'calendar:': CalendarWidget,
}

export type ResolvedWidget =
  | { Widget: ComponentType<WidgetProps>; instanceId?: undefined }
  | { Widget: ComponentType<InstanceWidgetProps>; instanceId: string }

/** The widget behind a key: an exact module, or a prefix module with a non-empty instance id. Unknown keys give undefined. */
export function resolveWidget(key: string): ResolvedWidget | undefined {
  // hasOwn: "toString" is no module.
  if (Object.hasOwn(widgets, key)) return { Widget: widgets[key] }
  const prefix = Object.keys(prefixWidgets).find((p) => key.startsWith(p) && key.length > p.length)
  return prefix ? { Widget: prefixWidgets[prefix], instanceId: key.slice(prefix.length) } : undefined
}
