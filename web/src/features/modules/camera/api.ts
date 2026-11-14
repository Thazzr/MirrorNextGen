import { api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type CameraSettings = components['schemas']['CameraSettings']
export type CameraSettingsUpdate = components['schemas']['CameraSettingsUpdate']
export type DashboardCamera = components['schemas']['DashboardCamera']

export const cameraSettingsQueryKey = ['settings', 'camera'] as const
export const dashboardCameraQueryKey = ['dashboard', 'camera'] as const

export async function fetchCameraSettings(): Promise<CameraSettings> {
  return unwrap(await api.GET('/api/v1/settings/camera'))
}

export async function saveCameraSettings(body: CameraSettingsUpdate): Promise<CameraSettings> {
  return unwrap(await api.PUT('/api/v1/settings/camera', { body }))
}

export async function fetchDashboardCamera(): Promise<DashboardCamera> {
  return unwrap(await api.GET('/api/v1/dashboard/camera'))
}

/** The mirror's WebSocket proxy of the camera; the player turns the path into ws:// or wss:// for the page's origin. */
export const cameraStreamPath = '/api/v1/camera/stream'

/** The CSS filter of the picture; 100 % and no grayscale leave it as it is. */
export function cameraFilter(c: Pick<DashboardCamera, 'brightness' | 'contrast' | 'saturation' | 'grayscale'>): string {
  return `brightness(${c.brightness}%) contrast(${c.contrast}%) saturate(${c.saturation}%)${c.grayscale ? ' grayscale(1)' : ''}`
}
