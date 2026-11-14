import { useQuery } from '@tanstack/react-query'
import { ApiError, api, unwrap } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'
import type { InstanceWidgetProps } from '../widgets.ts'
import { CalendarIcon } from './Icon.tsx'
import './calendar.css'

type Section = components['schemas']['DashboardCalendarSection']
type Item = components['schemas']['DashboardCalendarItem']
type Day = components['schemas']['ShiftDay']

const isNotFound = (error: unknown) => error instanceof ApiError && error.status === 404

export function CalendarWidget({ region, instanceId }: InstanceWidgetProps) {
  const query = useQuery({
    queryKey: ['dashboard', 'calendar', instanceId],
    queryFn: async () => unwrap(await api.GET('/api/v1/dashboard/calendar/{sectionId}', { params: { path: { sectionId: instanceId } } })),
    refetchInterval: 60_000,
  })
  // Right regions align the block to the right, left and middle regions to the left.
  const side = region === 'topRight' || region === 'bottomRight' ? ' mirror-calendar--right' : ''

  if (isNotFound(query.error)) return null
  const section = query.data
  return (
    <div className={`mirror-calendar${side}`}>
      {section ? (
        <section className="mirror-calendar__agenda">
          <h2 className="mirror-calendar__agenda-title">{section.title}</h2>
          {section.type === 'shifts' ? <Shifts section={section} /> : <Agenda section={section} />}
        </section>
      ) : query.isError ? (
        <div className="mirror-error">Termine sind nicht verfügbar.</div>
      ) : (
        <div className="mirror-muted">Termine werden geladen …</div>
      )}
    </div>
  )
}

function Agenda({ section }: { section: Section }) {
  const items = section.items ?? []
  return (
    <>
      {section.error && <div className="mirror-error">{section.error}</div>}
      <ul className="mirror-agenda">
        {items.length > 0 ? (
          items.map((item, index) => <AgendaItem key={`${item.title}-${index}`} item={item} />)
        ) : (
          !section.error && (
            <li className="mirror-agenda__item mirror-agenda__item--empty">
              <span className="mirror-muted">Keine Termine im ausgewählten Zeitraum.</span>
            </li>
          )
        )}
      </ul>
    </>
  )
}

function AgendaItem({ item }: { item: Item }) {
  return (
    <li className="mirror-agenda__item">
      <span className="mirror-agenda__symbol">
        <CalendarIcon icon={item.icon} />
      </span>
      <span className="mirror-agenda__content">
        <span className="mirror-agenda__title">{item.title}</span>
        {item.location && <span className="mirror-agenda__location">{item.location}</span>}
      </span>
      <span className="mirror-agenda__time">
        <span className="mirror-agenda__moment">{item.timeLabel}</span>
        {item.relative && (
          <span className={`mirror-agenda__relative${item.ongoing ? ' mirror-agenda__relative--ongoing' : ''}`}>{item.relative}</span>
        )}
      </span>
    </li>
  )
}

function Shifts({ section }: { section: Section }) {
  const days = section.days ?? []
  return (
    <>
      {section.error && <div className="mirror-error">{section.error}</div>}
      <ul className="mirror-shifts">
        {days.map((day) => (
          <ShiftRow key={day.date} day={day} />
        ))}
      </ul>
    </>
  )
}

function ShiftRow({ day }: { day: Day }) {
  return (
    <li className={`mirror-shifts__day${day.free ? ' mirror-shifts__day--free' : ''}`}>
      <span className="mirror-shifts__label">{day.dayLabel}</span>
      <span className="mirror-shifts__text">
        {!day.free && (
          <span className="mirror-agenda__symbol">
            <CalendarIcon icon={day.icon} />
          </span>
        )}
        <span>{day.free ? 'frei' : day.text}</span>
      </span>
    </li>
  )
}
