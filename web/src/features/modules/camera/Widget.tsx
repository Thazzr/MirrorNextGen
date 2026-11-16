import { useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { VideoRTC } from '../../../vendor/go2rtc/video-rtc.js'
import type { WidgetProps } from '../widgets.ts'
import { cameraFilter, cameraStreamPath, dashboardCameraQueryKey, fetchDashboardCamera, type DashboardCamera } from './api.ts'
import { checkStall, startStall, type StallSample } from './stall.ts'
import './camera.css'

const elementName = 'video-rtc'
if (!customElements.get(elementName)) customElements.define(elementName, VideoRTC)

/**
 * Hosts go2rtc's player element in MSE mode. It is created imperatively: the
 * element builds its own <video> when it is attached and owns the WebSocket,
 * so React must not re-render it. It reconnects by itself when the stream drops.
 * The filter sits on the host, so it covers the picture and not the title.
 */
function createPlayer(parent: HTMLElement): VideoRTC {
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
  return player
}

/** Closes the socket right away; a bare remove() would keep it for the player's 5 s disconnect delay. */
function destroyPlayer(player: VideoRTC) {
  player.ondisconnect()
  player.remove()
}

const checkEveryMs = 2_000

function Player({ filter }: { filter: string }) {
  const host = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const parent = host.current
    if (!parent) return
    let player = createPlayer(parent)
    let stall = startStall(Date.now())
    // The player only reconnects on socket close or <video> error, so a frozen picture would stay
    // frozen (go2rtc#1102): watch the shown frames and rebuild the player when they stand still.
    const timer = setInterval(() => {
      const now = Date.now()
      // The player disconnects in the background by itself; do not count that as a stall.
      if (document.hidden) {
        stall = startStall(now)
        return
      }
      const video = player.video
      const quality = video?.getVideoPlaybackQuality?.()
      const sample: StallSample = {
        presentedFrames: quality ? quality.totalVideoFrames - quality.droppedVideoFrames : undefined,
        currentTime: video?.currentTime ?? 0,
      }
      const result = checkStall(stall, sample, now)
      stall = result.state
      if (!result.restart) return
      console.warn('camera picture stalled, restarting the player')
      destroyPlayer(player)
      player = createPlayer(parent)
      stall = startStall(now)
    }, checkEveryMs)
    return () => {
      clearInterval(timer)
      destroyPlayer(player)
    }
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
