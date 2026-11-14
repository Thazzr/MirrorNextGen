import { lazy, type ComponentType, type LazyExoticComponent } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  IconCalendar,
  IconCloud,
  IconDeviceDesktop,
  IconIcons,
  IconLayoutDashboard,
  IconShoppingCart,
  IconUserShield,
  IconWifi,
} from '@tabler/icons-react'
import { useCalendarTileStatus } from '../modules/calendar/admin/status.ts'
import { useWeatherTileStatus } from '../modules/weather/admin/status.ts'
import { useBringTileStatus } from '../modules/bring/admin/status.ts'
import { useWifiTileStatus } from '../modules/wifi/admin/status.ts'
import { useIconsTileStatus } from './api/iconsStatus.ts'
import { displaySettingsQueryKey, fetchDisplaySettings } from './api/display.ts'
import { fetchLayout, layoutQueryKey } from './api/layout.ts'
import { placedKeys } from './pages/layoutEdit.ts'

/** What a tile shows under its name. */
export type TileStatus = { loading: true } | { loading: false; text: string }

export type AdminModule = {
  key: string
  title: string
  icon: ComponentType<{ size?: number; 'aria-hidden'?: boolean }>
  /** Route below `/admin`. */
  path: string
  load: () => Promise<{ default: ComponentType }>
  Page: LazyExoticComponent<ComponentType>
  useStatus: () => TileStatus
}

function entry(
  base: Omit<AdminModule, 'Page'>,
): AdminModule {
  return { ...base, Page: lazy(base.load) }
}

const UNAVAILABLE: TileStatus = { loading: false, text: 'Status nicht verfügbar' }

function useLayoutStatus(): TileStatus {
  const query = useQuery({ queryKey: layoutQueryKey, queryFn: fetchLayout })
  if (query.isPending) return { loading: true }
  if (query.isError) return UNAVAILABLE
  const n = placedKeys(query.data).length
  return { loading: false, text: n === 1 ? '1 Modul angezeigt' : `${n} Module angezeigt` }
}

function useDisplayStatus(): TileStatus {
  const query = useQuery({ queryKey: displaySettingsQueryKey, queryFn: fetchDisplaySettings })
  if (query.isPending) return { loading: true }
  if (query.isError) return UNAVAILABLE
  const { rotation, testPattern, schedule } = query.data
  const night = schedule.enabled ? ` · Nachtruhe ${schedule.offTime}–${schedule.onTime}` : ''
  return {
    loading: false,
    text: `Drehung ${rotation}° · Testbild ${testPattern ? 'an' : 'aus'}${night}`,
  }
}

const useAccountStatus = (): TileStatus => ({ loading: false, text: 'Passwort ändern' })

/** Tiles and pages of the admin area, in the order of the overview. */
export const adminModules: AdminModule[] = [
  entry({
    key: 'layout',
    title: 'Anordnung',
    icon: IconLayoutDashboard,
    path: 'layout',
    load: () => import('./pages/LayoutPage.tsx').then((m) => ({ default: m.LayoutPage })),
    useStatus: useLayoutStatus,
  }),
  entry({
    key: 'display',
    title: 'Anzeige',
    icon: IconDeviceDesktop,
    path: 'display',
    load: () => import('./pages/DisplayPage.tsx').then((m) => ({ default: m.DisplayPage })),
    useStatus: useDisplayStatus,
  }),
  entry({
    key: 'weather',
    title: 'Wetter',
    icon: IconCloud,
    path: 'weather',
    load: () => import('../modules/weather/admin/WeatherPage.tsx').then((m) => ({ default: m.WeatherPage })),
    useStatus: useWeatherTileStatus,
  }),
  entry({
    key: 'wifi',
    title: 'WLAN-QR-Code',
    icon: IconWifi,
    path: 'wifi',
    load: () => import('../modules/wifi/admin/WifiPage.tsx').then((m) => ({ default: m.WifiPage })),
    useStatus: useWifiTileStatus,
  }),
  entry({
    key: 'calendar',
    title: 'Kalender',
    icon: IconCalendar,
    path: 'calendar',
    load: () => import('../modules/calendar/admin/CalendarPage.tsx').then((m) => ({ default: m.CalendarPage })),
    useStatus: useCalendarTileStatus,
  }),
  entry({
    key: 'icons',
    title: 'Symbole',
    icon: IconIcons,
    path: 'icons',
    load: () => import('./pages/IconsPage.tsx').then((m) => ({ default: m.IconsPage })),
    useStatus: useIconsTileStatus,
  }),
  entry({
    key: 'bring',
    title: 'Bring!-Einkaufsliste',
    icon: IconShoppingCart,
    path: 'bring',
    load: () => import('../modules/bring/admin/BringPage.tsx').then((m) => ({ default: m.BringPage })),
    useStatus: useBringTileStatus,
  }),
  entry({
    key: 'account',
    title: 'Konto',
    icon: IconUserShield,
    path: 'account',
    load: () => import('./pages/AccountPage.tsx').then((m) => ({ default: m.AccountPage })),
    useStatus: useAccountStatus,
  }),
]

/** Loads every page chunk; tests call it up front so no test pays the transform inside its timeout. */
export function preloadAdminPages(): Promise<unknown> {
  return Promise.all(adminModules.map((m) => m.load()))
}
