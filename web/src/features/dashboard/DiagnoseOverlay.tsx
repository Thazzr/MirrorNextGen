import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { dashboardInfoQueryKey } from './useReloadOnNewBuild.ts'
import { api, unwrap } from '../../shared/api/client.ts'
import './diagnose.css'

type Viewport = { width: number; height: number }

function readViewport(): Viewport {
  return { width: window.innerWidth, height: window.innerHeight }
}

function orientation({ width, height }: Viewport): string {
  if (height > width) return 'Hochformat'
  if (width > height) return 'Querformat'
  return 'Quadratisch'
}

function useViewport(): Viewport {
  const [viewport, setViewport] = useState(readViewport)
  useEffect(() => {
    const onResize = () => setViewport(readViewport())
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])
  return viewport
}

type Props = {
  /** Content size when the browser rotates the dashboard; defaults to the window size. */
  size?: Viewport
  rotation?: { mode: 'browser' | 'monitor'; degrees: number }
}

/** Test pattern for aligning and checking the kiosk on the real display. */
export function DiagnoseOverlay({ size, rotation }: Props) {
  const windowViewport = useViewport()
  const viewport = size ?? windowViewport
  const info = useQuery({ queryKey: dashboardInfoQueryKey, queryFn: async () => unwrap(await api.GET('/api/v1/info')) })

  let server: string
  if (info.isPending) server = 'wird geladen …'
  else if (info.isError) server = `Server nicht erreichbar: ${info.error.message}`
  else server = `${info.data.version} (${info.data.commit})`

  return (
    <div className="diagnose" data-testid="diagnose">
      <span className="diagnose__corner diagnose__corner--tl" />
      <span className="diagnose__corner diagnose__corner--tr" />
      <span className="diagnose__corner diagnose__corner--bl" />
      <span className="diagnose__corner diagnose__corner--br" />
      <div className="diagnose__top">
        <span className="diagnose__arrow" aria-hidden="true">▲</span>
        <span>oben</span>
      </div>
      <section className="diagnose__panel">
        <h2>Diagnose</h2>
        <dl>
          <dt>Fenster</dt>
          <dd>{viewport.width} × {viewport.height}</dd>
          <dt>Bildschirm</dt>
          <dd>{window.screen.width} × {window.screen.height}</dd>
          <dt>Pixelverhältnis</dt>
          <dd>{window.devicePixelRatio}</dd>
          <dt>Ausrichtung</dt>
          <dd>{orientation(viewport)}</dd>
          {rotation && (
            <>
              <dt>Drehung</dt>
              <dd>{rotation.mode === 'browser' ? 'Browser' : 'Monitor'} {rotation.degrees}°</dd>
            </>
          )}
          <dt>Server-Version</dt>
          <dd>{server}</dd>
        </dl>
      </section>
    </div>
  )
}
