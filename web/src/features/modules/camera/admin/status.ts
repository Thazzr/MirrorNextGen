import { useQuery } from '@tanstack/react-query'
import { cameraSettingsQueryKey, fetchCameraSettings, type CameraSettings } from '../api.ts'

/** Tile text: "An" (shown on the mirror), "Aus" (set up but switched off) or "Nicht eingerichtet" (no stream address). */
export function cameraLine(settings: Pick<CameraSettings, 'enabled' | 'hasStreamUrl'>): string {
  if (!settings.hasStreamUrl) return 'Nicht eingerichtet'
  return settings.enabled ? 'An' : 'Aus'
}

export function useCameraTileStatus() {
  const query = useQuery({ queryKey: cameraSettingsQueryKey, queryFn: fetchCameraSettings })
  if (query.isPending) return { loading: true as const }
  if (query.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  return { loading: false as const, text: cameraLine(query.data) }
}
