import { useMemo } from 'react'
import { isoWeek, useClock } from '../../dashboard/clock.ts'
import './clock.css'

const dateFormat = new Intl.DateTimeFormat('de-DE', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })

export function ClockWidget() {
  const now = useClock()
  const calendarWeek = useMemo(() => isoWeek(now), [now])
  return (
    <section className="mirror-clock">
      <div className="mirror-clock__date">{dateFormat.format(now)}</div>
      <div className="mirror-clock__time">
        <span className="mirror-clock__hour">{now.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' })}</span>
        <small className="mirror-clock__seconds">{String(now.getSeconds()).padStart(2, '0')}</small>
      </div>
      <div className="mirror-clock__meta">KW {calendarWeek}</div>
    </section>
  )
}
