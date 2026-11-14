import { useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { api, unwrap } from '../../shared/api/client.ts'
import { isoWeek, useClock } from './clock.ts'
import { DiagnoseOverlay } from './DiagnoseOverlay.tsx'
import { RotatedViewport } from './RotatedViewport.tsx'
import { normalizeRotation, useBrowserRotation } from './useBrowserRotation.ts'
import './dashboard.css'

const kioskStateInterval = 5000

const dateFormat = new Intl.DateTimeFormat('de-DE', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })

export function DashboardPage() {
  const now = useClock()
  const calendarWeek = useMemo(() => isoWeek(now), [now])
  const [searchParams] = useSearchParams()
  // A failing request only means "no test pattern": the mirror shows no error text for it.
  const kioskState = useQuery({
    queryKey: ['kiosk', 'state'],
    queryFn: async () => unwrap(await api.GET('/api/v1/kiosk/state')),
    refetchInterval: kioskStateInterval,
  })

  // Not loaded yet or failed: rotation 0.
  const rotation = normalizeRotation(kioskState.data?.rotation)
  const browserMode = searchParams.get('rotation') === 'browser'
  const browser = useBrowserRotation(browserMode ? rotation : 0)

  useEffect(() => {
    document.body.classList.add('mirror-kiosk')
    return () => document.body.classList.remove('mirror-kiosk')
  }, [])

  const page = (
    <main className="mirror-dashboard">
      <h1 className="visually-hidden">Spiegel</h1>
      <header className="mirror-header">
        <section className="mirror-clock">
          <div className="mirror-clock__date">{dateFormat.format(now)}</div>
          <div className="mirror-clock__time">
            <span className="mirror-clock__hour">{now.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })}</span>
            <small className="mirror-clock__seconds">{String(now.getSeconds()).padStart(2, '0')}</small>
          </div>
          <div className="mirror-clock__meta">KW {calendarWeek}</div>
        </section>
      </header>
      {(kioskState.data?.testPattern || searchParams.has('diagnose')) && (
        <DiagnoseOverlay
          size={browserMode ? browser.size : undefined}
          rotation={{ mode: browserMode ? 'browser' : 'monitor', degrees: rotation }}
        />
      )}
    </main>
  )
  return browserMode ? <RotatedViewport style={browser.style}>{page}</RotatedViewport> : page
}
