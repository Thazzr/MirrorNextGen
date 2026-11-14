import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { VideoRTC } from '../../../vendor/go2rtc/video-rtc.js'
import type { WidgetProps } from '../widgets.ts'
import { cameraFilter, cameraStreamPath, dashboardCameraQueryKey, fetchDashboardCamera, type DashboardCamera } from './api.ts'
import './camera.css'

const elementName = 'video-rtc'
if (!customElements.get(elementName)) customElements.define(elementName, VideoRTC)

/**
 * Hosts go2rtc's player element in MSE mode. It is created imperatively: the
 * element builds its own <video> when it is attached and owns the WebSocket,
 * so React must not re-render it. It reconnects by itself when the stream drops.
 * The filter sits on the host, so it covers the picture and not the title.
 */
function Player({ filter }: { filter: string }) {
  const host = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const parent = host.current
    if (!parent) return
    const player = document.createElement(elementName) as VideoRTC
    player.mode = 'mse'
    player.media = 'video'
    parent.append(player)
    if (player.video) {
      player.video.controls = false
      player.video.muted = true
      player.video.autoplay = true
      player.video.disablePictureInPicture = true
    }
    player.src = cameraStreamPath
    return () => player.remove()
  }, [])
  return <div className="mirror-camera__video" ref={host} style={{ filter }} />
}

function CameraView({ camera, region }: { camera: DashboardCamera; region: WidgetProps['region'] }) {
  // Left regions align the block to the left, right regions (and the middle) to the right like the weather.
  const side = region === 'topLeft' || region === 'bottomLeft' ? ' mirror-camera--left' : ''
  return (
    <aside className={`mirror-camera${side}`} aria-label={camera.title || 'Kamera'}>
      {camera.title && <div className="mirror-camera__title">{camera.title}</div>}
      <Player filter={cameraFilter(camera)} />
    </aside>
  )
}

export function CameraWidget({ region }: WidgetProps) {
  // Nothing while loading, on failure or while not configured: an error text would only cover the mirror.
  const query = useQuery({ queryKey: dashboardCameraQueryKey, queryFn: fetchDashboardCamera, refetchInterval: 60_000 })
  if (!query.data?.configured) return null
  return <CameraView camera={query.data} region={region} />
}
