import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const none = { kind: 'none' }
const stored = {
  refreshMinutes: 15,
  calendars: [
    { id: 'cal-a', name: 'Familie', icon: none, hasUrl: true, urlHint: 'https://calendar.example/.../basic.ics' },
    { id: 'cal-b', name: 'Arbeit', icon: { kind: 'bootstrap', name: 'briefcase' }, hasUrl: true, urlHint: 'https://work.example/.../x.ics' },
  ],
  sections: [
    { id: 's-list', title: 'Familie', type: 'list', calendarIds: ['cal-a'], maxEvents: 8, lookAheadDays: 14, shiftDays: 2, shifts: [] },
    {
      id: 's-shift',
      title: 'Dienst',
      type: 'shifts',
      calendarIds: ['cal-b'],
      maxEvents: 0,
      lookAheadDays: 0,
      shiftDays: 2,
      shifts: [{ id: 'sh1', match: 'F', matchMode: 'equals', label: 'Frühdienst', start: '06:00', end: '14:00', icon: none }],
    },
  ],
  rules: [
    { id: 'r1', calendarIds: [], matchMode: 'contains', text: 'Müll', action: 'hide', icon: none, renameTo: '' },
    { id: 'r2', calendarIds: ['cal-a'], matchMode: 'equals', text: 'Schule', action: 'rename', icon: none, renameTo: 'Unterricht' },
  ],
}

const emptyPreview = { sections: [], calendarErrors: [] }

function arrange(over: Record<string, Handler> = {}, settings: object = stored) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/settings/calendar': () => json(settings),
    'GET /api/v1/icons': () => json([{ id: 'ic1', name: 'Logo', contentType: 'image/png', size: 10 }]),
    ...over,
  })
}

async function open() {
  const view = renderWithProviders('/admin/calendar')
  await screen.findByRole('group', { name: 'Kalender 1' })
  return view
}

const bar = () => screen.queryByRole('region', { name: 'Ungespeicherte Änderungen' })
const saveButton = () => userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
const tab = (name: string) => userEvent.click(screen.getByRole('tab', { name }))
const group = (name: string) => within(screen.getByRole('group', { name }))
const puts = (api: ReturnType<typeof arrange>) => api.callsTo('PUT /api/v1/settings/calendar')

/** Opens a Mantine select and picks an option; closed lists stay mounted but hidden. */
async function pick(el: HTMLElement, option: string) {
  await userEvent.click(el)
  const list = document.getElementById(el.getAttribute('aria-controls') ?? '')
  await userEvent.click(await within(list as HTMLElement).findByRole('option', { name: option, hidden: true }))
}

describe('calendar page', () => {
  it('loads calendars with their address hint, has one h1 and the four tabs', async () => {
    arrange()
    await open()
    expectSinglePageTitle('Kalender')
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Kalender', 'Bereiche', 'Regeln', 'Vorschau'])
    expect(group('Kalender 1').getByLabelText('Name')).toHaveValue('Familie')
    expect(group('Kalender 1').getByText('https://calendar.example/.../basic.ics')).toBeInTheDocument()
    expect(group('Kalender 1').queryByLabelText(/Adresse/)).not.toBeInTheDocument()
    expect(bar()).not.toBeInTheDocument()
  })

  it('a new calendar needs an address and shows the server error on the url field', async () => {
    const api = arrange({
      'PUT /api/v1/settings/calendar': () => problem(400, 'Ungültig', { errors: { 'calendars.2.url': 'Die Adresse fehlt.' } }),
    })
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Kalender hinzufügen' }))
    const fresh = group('Kalender 3')
    expect(fresh.getByLabelText(/Adresse/)).toBeRequired()
    expect(fresh.getByPlaceholderText('https://… oder webcal://…')).toBeInTheDocument()
    expect(fresh.getByText(/Privatadresse im iCal-Format/)).toBeInTheDocument()
    await userEvent.type(fresh.getByLabelText('Name'), 'Schule')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Kalender')
    await saveButton()
    expect(await fresh.findByText('Die Adresse fehlt.')).toBeInTheDocument()
    expect(puts(api)[0].body).toMatchObject({ calendars: [{ id: 'cal-a' }, { id: 'cal-b' }, { name: 'Schule' }] })
    expect((puts(api)[0].body as { calendars: Array<{ url?: string }> }).calendars[2]).not.toHaveProperty('url')
    expect(bar()).toHaveTextContent('Bitte prüfe die Eingaben in: Kalender')
    expect(fresh.getByLabelText('Name')).toHaveValue('Schule')
  })

  it('sends the url only after "Adresse ändern" and typing', async () => {
    const api = arrange({ 'PUT /api/v1/settings/calendar': (c) => json({ ...stored, ...(c.body as object) }) })
    await open()
    await userEvent.click(group('Kalender 1').getByRole('button', { name: 'Adresse ändern' }))
    // Revealed but empty: nothing changed, the stored address stays.
    expect(bar()).not.toBeInTheDocument()
    await userEvent.type(group('Kalender 1').getByLabelText('Adresse (iCal-Link)'), 'webcal://neu.example/a.ics')
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Kalender')
    await saveButton()
    await waitFor(() => expect(puts(api)).toHaveLength(1))
    const sent = puts(api)[0].body as { calendars: Array<{ id: string; url?: string }> }
    expect(sent.calendars[0].url).toBe('webcal://neu.example/a.ics')
    expect(sent.calendars[1]).not.toHaveProperty('url')
    expect(puts(api)[0].csrf).toBe('tok-123')
  })

  it('sections: switching the type shows the matching fields', async () => {
    arrange()
    await open()
    await tab('Bereiche')
    const list = group('Bereich 1')
    expect(list.getByLabelText('Höchstens N Termine')).toHaveValue('8')
    expect(list.getByLabelText('Vorschau-Tage')).toHaveValue('14')
    expect(list.queryByLabelText('Tage')).not.toBeInTheDocument()
    const shifts = group('Bereich 2')
    expect(shifts.getByLabelText('Tage')).toHaveValue('2')
    expect(shifts.queryByLabelText('Höchstens N Termine')).not.toBeInTheDocument()
    await userEvent.click(list.getByRole('radio', { name: 'Schichten' }))
    expect(list.getByLabelText('Tage')).toBeInTheDocument()
    expect(list.queryByLabelText('Höchstens N Termine')).not.toBeInTheDocument()
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Bereiche')
    expect(screen.getByText(/Neue Bereiche erscheinen oben links am Spiegel/)).toBeInTheDocument()
  })

  it('shift rows can be added and removed and warn when only one time is set', async () => {
    arrange()
    await open()
    await tab('Bereiche')
    const shifts = group('Bereich 2')
    expect(shifts.getByLabelText('Beginn')).toHaveValue('06:00')
    await userEvent.click(shifts.getByRole('button', { name: 'Schicht hinzufügen' }))
    const added = group('Schicht 2')
    await userEvent.type(added.getByLabelText('Kürzel/Stichwort'), 'S')
    // fireEvent-free: time inputs accept typed values in jsdom.
    await userEvent.type(added.getByLabelText('Beginn'), '14:00')
    expect(added.getByText('Beginn und Ende gehören zusammen.')).toBeInTheDocument()
    await userEvent.type(added.getByLabelText('Ende'), '22:00')
    expect(added.queryByText('Beginn und Ende gehören zusammen.')).not.toBeInTheDocument()
    await userEvent.click(group('Schicht 1').getByRole('button', { name: 'Schicht 1 entfernen' }))
    expect(screen.getAllByRole('group', { name: /^Schicht \d$/ })).toHaveLength(1)
    expect(group('Schicht 1').getByLabelText('Kürzel/Stichwort')).toHaveValue('S')
  })

  it('removing a section and reordering sections change the sent order', async () => {
    const api = arrange({ 'PUT /api/v1/settings/calendar': (c) => json({ ...stored, ...(c.body as object) }) })
    await open()
    await tab('Bereiche')
    await userEvent.click(screen.getByRole('button', { name: 'Bereich 2 nach oben' }))
    await saveButton()
    await waitFor(() => expect(puts(api)).toHaveLength(1))
    expect((puts(api)[0].body as { sections: Array<{ id: string }> }).sections.map((s) => s.id)).toEqual(['s-shift', 's-list'])
    await userEvent.click(screen.getByRole('button', { name: 'Bereich 1 entfernen' }))
    await saveButton()
    await waitFor(() => expect(puts(api)).toHaveLength(2))
    expect((puts(api)[1].body as { sections: Array<{ id: string }> }).sections.map((s) => s.id)).toEqual(['s-list'])
  })

  it('rules: reorder, action-dependent fields and "Alle Kalender" as an empty list', async () => {
    const api = arrange({ 'PUT /api/v1/settings/calendar': (c) => json({ ...stored, ...(c.body as object) }) })
    await open()
    await tab('Regeln')
    expect(screen.getByText(/„Ausblenden“ beendet die Prüfung/)).toBeInTheDocument()
    const r1 = group('Regel 1')
    expect(r1.getByPlaceholderText('Alle Kalender')).toBeInTheDocument()
    expect(r1.queryByLabelText('Neuer Titel')).not.toBeInTheDocument()
    expect(group('Regel 2').getByLabelText('Neuer Titel')).toHaveValue('Unterricht')
    await pick(r1.getByRole('combobox', { name: 'Aktion' }), 'Symbol setzen')
    expect(r1.getByLabelText('Symbol')).toBeInTheDocument()
    expect(r1.queryByLabelText('Neuer Titel')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Regel 2 nach oben' }))
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Regeln')
    await saveButton()
    await waitFor(() => expect(puts(api)).toHaveLength(1))
    const rules = (puts(api)[0].body as { rules: Array<{ id: string; calendarIds: string[]; action: string }> }).rules
    expect(rules.map((r) => r.id)).toEqual(['r2', 'r1'])
    expect(rules[1]).toMatchObject({ calendarIds: [], action: 'icon' })
  })

  it('a successful save refreshes the dashboard calendar and layout caches', async () => {
    arrange({ 'PUT /api/v1/settings/calendar': (c) => json({ ...stored, ...(c.body as object) }) })
    const { queryClient } = await open()
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    await userEvent.type(group('Kalender 1').getByLabelText('Name'), 'x')
    await saveButton()
    await waitFor(() => expect(bar()).not.toBeInTheDocument())
    const keys = spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey))
    expect(keys).toContain('["dashboard","calendar"]')
    expect(keys).toContain('["dashboard","layout"]')
  })

  it('a new rule applies to all calendars (empty calendarIds) and new ids match the server pattern', async () => {
    const api = arrange({ 'PUT /api/v1/settings/calendar': (c) => json({ ...stored, ...(c.body as object) }) })
    await open()
    await userEvent.click(screen.getByRole('button', { name: 'Kalender hinzufügen' }))
    await tab('Bereiche')
    await userEvent.click(screen.getByRole('button', { name: 'Bereich hinzufügen' }))
    await userEvent.click(within(screen.getByRole('group', { name: 'Bereich 3' })).getByRole('radio', { name: 'Schichten' }))
    await userEvent.click(within(screen.getByRole('group', { name: 'Bereich 3' })).getByRole('button', { name: 'Schicht hinzufügen' }))
    await tab('Regeln')
    await userEvent.click(screen.getByRole('button', { name: 'Regel hinzufügen' }))
    await saveButton()
    await waitFor(() => expect(puts(api)).toHaveLength(1))
    const body = puts(api)[0].body as {
      calendars: Array<{ id: string }>
      sections: Array<{ id: string; shifts: Array<{ id: string }> }>
      rules: Array<{ id: string; calendarIds: string[]; action: string }>
    }
    expect(body.rules).toHaveLength(3)
    expect(body.rules[2].calendarIds).toEqual([])
    const ids = [body.calendars[2].id, body.sections[2].id, body.sections[2].shifts[0].id, body.rules[2].id]
    for (const id of ids) expect(id).toMatch(/^[a-z0-9-]{1,40}$/)
    expect(new Set(ids).size).toBe(4)
  })

  it('names exactly the changed tabs in the bar', async () => {
    arrange()
    await open()
    await userEvent.type(group('Kalender 1').getByLabelText('Name'), 'x')
    await tab('Regeln')
    await userEvent.click(screen.getByRole('button', { name: 'Regel hinzufügen' }))
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Kalender, Regeln')
    expect(bar()).not.toHaveTextContent('Bereiche')
  })

  it('a field error on a section switches to that tab and marks it', async () => {
    arrange({ 'PUT /api/v1/settings/calendar': () => problem(400, 'Ungültig', { errors: { 'sections.1.shifts.0.start': 'Ungültige Uhrzeit.' } }) })
    await open()
    await userEvent.type(group('Kalender 1').getByLabelText('Name'), 'x')
    await saveButton()
    expect(await screen.findByText('Ungültige Uhrzeit.')).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Bereiche (Fehler)' })).toHaveAttribute('aria-selected', 'true')
    expect(bar()).toHaveTextContent('Bitte prüfe die Eingaben in: Bereiche')
    await tab('Kalender')
    expect(group('Kalender 1').getByLabelText('Name')).toHaveValue('Familiex')
  })

  it('discard returns to the stored values and hides the bar', async () => {
    arrange()
    await open()
    await userEvent.type(group('Kalender 1').getByLabelText('Name'), 'x')
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Verwerfen' }))
    expect(group('Kalender 1').getByLabelText('Name')).toHaveValue('Familie')
    expect(bar()).not.toBeInTheDocument()
  })
})

describe('calendar preview', () => {
  const preview = {
    sections: [
      {
        id: 's-list',
        title: 'Familie',
        type: 'list',
        items: [
          item({ title: 'Zahnarzt', originalTitle: 'Zahnarzt', timeLabel: 'Morgen 14:30' }),
          item({ title: 'Müllabfuhr', originalTitle: 'Müllabfuhr', hidden: true, shown: false, matchedRules: ['r1'], timeLabel: 'Heute' }),
          item({ title: 'Unterricht', originalTitle: 'Schule', matchedRules: ['r2'], timeLabel: 'in 2 Std.' }),
          item({ title: 'Spätes', originalTitle: 'Spätes', shown: false, timeLabel: 'in 9 Tagen' }),
        ],
      },
      {
        id: 's-shift',
        title: 'Dienst',
        type: 'shifts',
        days: [
          { date: '2026-11-14', dayLabel: 'Heute', text: 'Frühdienst 06–14 Uhr', icon: none, free: false },
          { date: '2026-11-15', dayLabel: 'Morgen', text: 'frei', icon: none, free: true },
        ],
        summary: 'Heute: Frühdienst 06–14 Uhr · Morgen: frei',
      },
    ],
    calendarErrors: [{ calendarId: 'cal-b', message: 'Kalender nicht erreichbar.' }],
  }

  function item(over: Record<string, unknown>) {
    return {
      title: '',
      originalTitle: '',
      timeLabel: '',
      relative: '',
      icon: none,
      allDay: false,
      ongoing: false,
      hidden: false,
      shown: true,
      matchedRules: [],
      calendarId: 'cal-a',
      start: '2026-11-15T14:30:00+01:00',
      ...over,
    }
  }

  it('runs when the tab opens, sends the unsaved draft and renders hidden, cut, renamed and matched items', async () => {
    const api = arrange({ 'POST /api/v1/settings/calendar/preview': () => json(preview) })
    await open()
    await userEvent.type(group('Kalender 1').getByLabelText('Name'), 'x')
    await tab('Vorschau')
    const section = await screen.findByRole('group', { name: 'Vorschau Familie' })
    const calls = api.callsTo('POST /api/v1/settings/calendar/preview')
    expect(calls).toHaveLength(1)
    expect((calls[0].body as { calendars: Array<{ name: string }> }).calendars[0].name).toBe('Familiex')
    expect(calls[0].csrf).toBe('tok-123')

    const s = within(section)
    expect(s.getByText('Zahnarzt')).not.toHaveStyle({ textDecoration: 'line-through' })
    expect(s.getByText('Müllabfuhr')).toHaveStyle({ textDecoration: 'line-through' })
    expect(s.getByText('ausgeblendet')).toBeInTheDocument()
    expect(s.getByText('Regel 1')).toBeInTheDocument()
    expect(s.getByText('Unterricht (vorher: Schule)')).toBeInTheDocument()
    expect(s.getByText('Regel 2')).toBeInTheDocument()
    expect(s.getByText('über dem Limit')).toBeInTheDocument()

    const shifts = within(screen.getByRole('group', { name: 'Vorschau Dienst' }))
    expect(shifts.getByText('Heute: Frühdienst 06–14 Uhr · Morgen: frei')).toBeInTheDocument()
    expect(shifts.getByText('Morgen')).toBeInTheDocument()
    expect(screen.getByText('Kalender nicht erreichbar.')).toBeInTheDocument()
    expect(screen.getByText('Kalender „Arbeit“')).toBeInTheDocument()
  })

  it('sends the whole current draft, and the refresh carries the newer state', async () => {
    const api = arrange({ 'POST /api/v1/settings/calendar/preview': () => json(emptyPreview) })
    await open()
    await tab('Regeln')
    await userEvent.clear(group('Regel 1').getByLabelText('Text'))
    await userEvent.type(group('Regel 1').getByLabelText('Text'), 'Abfall')
    await tab('Bereiche')
    await userEvent.type(group('Bereich 1').getByLabelText('Titel'), ' neu')
    await tab('Vorschau')
    await waitFor(() => expect(api.callsTo('POST /api/v1/settings/calendar/preview')).toHaveLength(1))
    type Body = { rules: Array<{ id: string; text: string }>; sections: Array<{ title: string }>; calendars: unknown[] }
    const first = api.callsTo('POST /api/v1/settings/calendar/preview')[0].body as Body
    expect(first.rules.map((r) => r.text)).toEqual(['Abfall', 'Schule'])
    expect(first.sections.map((s) => s.title)).toEqual(['Familie neu', 'Dienst'])
    expect(first.calendars).toHaveLength(2)
    await tab('Regeln')
    await userEvent.type(group('Regel 2').getByLabelText('Text'), 'x')
    await tab('Vorschau')
    await userEvent.click(screen.getByRole('button', { name: 'Vorschau aktualisieren' }))
    await waitFor(() => expect(api.callsTo('POST /api/v1/settings/calendar/preview').length).toBeGreaterThanOrEqual(2))
    const calls = api.callsTo('POST /api/v1/settings/calendar/preview')
    const last = calls[calls.length - 1].body as Body
    expect(last.rules.map((r) => r.text)).toEqual(['Abfall', 'Schulex'])
    expect(last.sections[0].title).toBe('Familie neu')
  })

  it('"Vorschau aktualisieren" runs it again', async () => {
    const api = arrange({ 'POST /api/v1/settings/calendar/preview': () => json(emptyPreview) })
    await open()
    await tab('Vorschau')
    await waitFor(() => expect(api.callsTo('POST /api/v1/settings/calendar/preview')).toHaveLength(1))
    await userEvent.click(screen.getByRole('button', { name: 'Vorschau aktualisieren' }))
    await waitFor(() => expect(api.callsTo('POST /api/v1/settings/calendar/preview')).toHaveLength(2))
  })

  it('shows an error when the preview fails', async () => {
    arrange({ 'POST /api/v1/settings/calendar/preview': () => problem(500, 'Kaputt') })
    await open()
    await tab('Vorschau')
    expect(await screen.findByText('Die Vorschau konnte nicht erstellt werden.')).toBeInTheDocument()
  })
})
