import { screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../../test/mockApi.ts'
import { layoutOf } from '../../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../../test/renderWithProviders.tsx'
import type { components } from '../../../shared/api/schema.gen.ts'

type Section = components['schemas']['DashboardCalendarSection']
type Item = components['schemas']['DashboardCalendarItem']

const none = { kind: 'none' } as const
const item = (parts: Partial<Item> = {}): Item => ({
  title: 'Zahnarzt',
  timeLabel: 'Morgen 14:30',
  relative: 'in 1 Tag',
  icon: none,
  allDay: false,
  ongoing: false,
  ...parts,
})
const listSection = (parts: Partial<Section> = {}): Section => ({ id: 'familie', title: 'Familie', type: 'list', items: [item()], ...parts })

function setup(response: unknown | Response, layout = layoutOf({ topLeft: ['calendar:familie'] })) {
  return mockApi({
    'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
    'GET /api/v1/dashboard/layout': () => json(layout),
    'GET /api/v1/dashboard/calendar/familie': () => (response instanceof Response ? response.clone() : json(response)),
  })
}

const region = (container: HTMLElement, name: string) => container.querySelector(`[data-region="${name}"]`)

describe('calendar widget (list)', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('loads the section by the instance id after the prefix and shows title, time label and relative text', async () => {
    const mock = setup(listSection({ items: [item({ location: 'Praxis Dr. Meier' })] }))
    renderWithProviders('/')
    expect(await screen.findByRole('heading', { name: 'Familie' })).toBeInTheDocument()
    expect(screen.getByText('Zahnarzt')).toBeInTheDocument()
    expect(screen.getByText('Morgen 14:30')).toBeInTheDocument()
    expect(screen.getByText('in 1 Tag')).toBeInTheDocument()
    expect(screen.getByText('Praxis Dr. Meier')).toBeInTheDocument()
    expect(mock.callsTo('GET /api/v1/dashboard/calendar/familie')).toHaveLength(1)
  })

  it('shows a loading text first', async () => {
    setup(listSection())
    renderWithProviders('/')
    expect(await screen.findByText('Termine werden geladen …')).toBeInTheDocument()
    await screen.findByText('Zahnarzt')
  })

  it('renders the three icon kinds', async () => {
    setup(
      listSection({
        items: [
          item({ title: 'A', icon: { kind: 'bootstrap', name: 'cake' } }),
          item({ title: 'B', icon: { kind: 'upload', file: 'abc123' } }),
          item({ title: 'C', icon: none }),
        ],
      }),
    )
    const { container } = renderWithProviders('/')
    await screen.findByText('A')
    const symbols = [...container.querySelectorAll('.mirror-agenda__symbol')]
    expect(symbols[0].querySelector('i')).toHaveClass('bi', 'bi-cake')
    expect(symbols[1].querySelector('img')).toHaveAttribute('src', '/api/v1/icons/abc123/file')
    expect(symbols[1].querySelector('i')).toBeNull()
    expect(symbols[2].querySelector('i')).toHaveClass('bi-calendar-event')
  })

  it('shows the empty text when there are no events and no error', async () => {
    setup(listSection({ items: [] }))
    renderWithProviders('/')
    expect(await screen.findByText('Keine Termine im ausgewählten Zeitraum.')).toBeInTheDocument()
  })

  it('shows items together with the error line', async () => {
    setup(listSection({ error: 'Kalender „Arbeit“ ist nicht erreichbar.' }))
    renderWithProviders('/')
    expect(await screen.findByText('Zahnarzt')).toBeInTheDocument()
    expect(screen.getByText('Kalender „Arbeit“ ist nicht erreichbar.')).toBeInTheDocument()
  })

  it('shows only the error line (no empty text) when there are no items', async () => {
    setup(listSection({ items: [], error: 'Kalender nicht erreichbar.' }))
    renderWithProviders('/')
    expect(await screen.findByText('Kalender nicht erreichbar.')).toBeInTheDocument()
    expect(screen.queryByText('Keine Termine im ausgewählten Zeitraum.')).not.toBeInTheDocument()
  })

  it('shows an error line when the request fails', async () => {
    setup(problem(500, 'kaputt'))
    renderWithProviders('/')
    expect(await screen.findByText('Termine sind nicht verfügbar.')).toBeInTheDocument()
  })

  it('renders nothing for a deleted section (404)', async () => {
    const mock = setup(problem(404, 'Not Found'))
    const { container } = renderWithProviders('/')
    await waitFor(() => expect(mock.callsTo('GET /api/v1/dashboard/calendar/familie')).toHaveLength(1))
    await waitFor(() => expect(screen.queryByText('Termine werden geladen …')).not.toBeInTheDocument())
    expect(container.querySelector('.mirror-calendar')).toBeNull()
    expect(screen.queryByText('Termine sind nicht verfügbar.')).not.toBeInTheDocument()
  })

  it('marks an ongoing event with the accent class and leaves others plain', async () => {
    setup(listSection({ items: [item({ title: 'Läuft', relative: 'noch 1 Std.', ongoing: true }), item({ title: 'Später', relative: 'in 3 Std.' })] }))
    renderWithProviders('/')
    expect(await screen.findByText('noch 1 Std.')).toHaveClass('mirror-agenda__relative--ongoing')
    expect(screen.getByText('in 3 Std.')).not.toHaveClass('mirror-agenda__relative--ongoing')
  })

  it('aligns right in right regions and left in left and middle regions', async () => {
    setup(listSection(), layoutOf({ topRight: ['calendar:familie'] }))
    const right = renderWithProviders('/')
    await screen.findByText('Zahnarzt')
    expect(region(right.container, 'topRight')?.querySelector('.mirror-calendar')).toHaveClass('mirror-calendar--right')
    right.unmount()

    setup(listSection(), layoutOf({ topLeft: ['calendar:familie'] }))
    const left = renderWithProviders('/')
    await screen.findByText('Zahnarzt')
    expect(region(left.container, 'topLeft')?.querySelector('.mirror-calendar')).not.toHaveClass('mirror-calendar--right')
    left.unmount()

    setup(listSection(), layoutOf({ middle: ['calendar:familie'] }))
    const middle = renderWithProviders('/')
    await screen.findByText('Zahnarzt')
    expect(region(middle.container, 'middle')?.querySelector('.mirror-calendar')).not.toHaveClass('mirror-calendar--right')
  })

  it('aligns right in the bottom right region and left in the bottom left region', async () => {
    setup(listSection(), layoutOf({ bottomRight: ['calendar:familie'] }))
    const right = renderWithProviders('/')
    await screen.findByText('Zahnarzt')
    expect(region(right.container, 'bottomRight')?.querySelector('.mirror-calendar')).toHaveClass('mirror-calendar--right')
    right.unmount()

    setup(listSection(), layoutOf({ bottomLeft: ['calendar:familie'] }))
    const left = renderWithProviders('/')
    await screen.findByText('Zahnarzt')
    expect(region(left.container, 'bottomLeft')?.querySelector('.mirror-calendar')).not.toHaveClass('mirror-calendar--right')
  })

  it('refetches the section every 60 seconds', async () => {
    setup(listSection())
    const { queryClient } = renderWithProviders('/')
    await screen.findByText('Zahnarzt')
    const query = queryClient.getQueryCache().find({ queryKey: ['dashboard', 'calendar', 'familie'] })
    expect(query?.observers).toHaveLength(1)
    expect(query?.observers[0].options.refetchInterval).toBe(60_000)
  })
})

describe('calendar widget (shifts)', () => {
  afterEach(() => vi.unstubAllGlobals())

  const shifts = (parts: Partial<Section> = {}): Section => ({
    id: 'familie',
    title: 'Dienst',
    type: 'shifts',
    days: [
      { date: '2026-11-14', dayLabel: 'Heute', text: 'Frühdienst 06–14 Uhr', icon: { kind: 'bootstrap', name: 'sunrise' }, free: false },
      { date: '2026-11-15', dayLabel: 'Morgen', text: '', icon: none, free: true },
    ],
    ...parts,
  })

  it('renders one row per day with label and text, free days muted as "frei"', async () => {
    setup(shifts())
    const { container } = renderWithProviders('/')
    expect(await screen.findByRole('heading', { name: 'Dienst' })).toBeInTheDocument()
    const rows = [...container.querySelectorAll('.mirror-shifts__day')]
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('HeuteFrühdienst 06–14 Uhr')
    expect(rows[0]).not.toHaveClass('mirror-shifts__day--free')
    expect(rows[0].querySelector('i')).toHaveClass('bi-sunrise')
    expect(rows[1]).toHaveTextContent('Morgenfrei')
    expect(rows[1]).toHaveClass('mirror-shifts__day--free')
    expect(rows[1].querySelector('i')).toBeNull()
  })

  it('shows an uploaded shift icon', async () => {
    setup(shifts({ days: [{ date: '2026-11-14', dayLabel: 'Heute', text: 'Spät', icon: { kind: 'upload', file: 'zz' }, free: false }] }))
    const { container } = renderWithProviders('/')
    await screen.findByText('Spät')
    expect(container.querySelector('.mirror-shifts__text img')).toHaveAttribute('src', '/api/v1/icons/zz/file')
  })

  it('shows the error line and renders nothing on 404', async () => {
    setup(shifts({ days: [], error: 'Dienstplan nicht erreichbar.' }))
    renderWithProviders('/')
    expect(await screen.findByText('Dienstplan nicht erreichbar.')).toBeInTheDocument()
  })

  it('renders nothing for a deleted shifts section', async () => {
    const mock = setup(problem(404, 'Not Found'))
    const { container } = renderWithProviders('/')
    await waitFor(() => expect(mock.callsTo('GET /api/v1/dashboard/calendar/familie')).toHaveLength(1))
    await waitFor(() => expect(container.querySelector('.mirror-calendar')).toBeNull())
  })
})
