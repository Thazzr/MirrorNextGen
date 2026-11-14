/// <reference types="node" />
import { File as NodeFile } from 'node:buffer'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, noContent, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

// jsdom replaces FormData and File with its own classes, but the generated client sends a Node Request. Newer
// Node versions only accept Node's own FormData there (Node 22 tolerated the mix), so the upload tests use
// Node's FormData together with Node's File. Response.formData() hands out an instance of Node's class.
async function useNodeFormData() {
  const NodeFormData = (await new Response(new URLSearchParams()).formData()).constructor
  vi.stubGlobal('FormData', NodeFormData)
}

const icons = [
  { id: 'ic1', name: 'Logo', contentType: 'image/png', size: 100 },
  { id: 'ic2', name: 'Stern', contentType: 'image/svg+xml', size: 50 },
]

function arrange(over: Record<string, Handler> = {}) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/icons': () => json(icons),
    ...over,
  })
}

const pngFile = () => new NodeFile([new Uint8Array([137, 80, 78, 71])], 'logo.png', { type: 'image/png' }) as unknown as File

async function openIcons() {
  const view = renderWithProviders('/admin/icons')
  await screen.findByRole('group', { name: 'Logo' })
  return view
}

describe('test environment', () => {
  it('lets FormData take the test file and keeps its name', async () => {
    await useNodeFormData()
    const form = new FormData()
    expect(() => form.append('file', pngFile())).not.toThrow()
    expect((form.get('file') as File).name).toBe('logo.png')
  })
})

describe('icons page', () => {
  it('lists the uploaded icons with previews and has one h1', async () => {
    arrange()
    const { container } = await openIcons()
    expectSinglePageTitle('Symbole')
    expect(screen.getByRole('group', { name: 'Stern' })).toBeInTheDocument()
    // A PNG stays an image; an SVG is a mask in the text colour, so it shows on the dark page.
    const imgs = [...container.querySelectorAll('img')].map((i) => i.getAttribute('src'))
    expect(imgs).toEqual(['/api/v1/icons/ic1/file'])
    const masks = [...container.querySelectorAll<HTMLElement>('span[aria-hidden="true"]')].filter((m) => m.style.maskImage)
    expect(masks).toHaveLength(1)
    expect(masks[0].style.maskImage).toContain('/api/v1/icons/ic2/file')
    expect(within(screen.getByRole('group', { name: 'Stern' })).queryByRole('img', { hidden: true })).toBeNull()
  })

  it('says so when there are no icons', async () => {
    arrange({ 'GET /api/v1/icons': () => json([]) })
    renderWithProviders('/admin/icons')
    expect(await screen.findByText('Noch keine eigenen Symbole hochgeladen.')).toBeInTheDocument()
  })

  it('uploads the file and the name as multipart with the CSRF header', async () => {
    await useNodeFormData()
    const api = arrange({ 'POST /api/v1/icons': () => json({ id: 'ic3', name: 'Mein Logo', contentType: 'image/png', size: 4 }, { status: 201 }) })
    const { container } = await openIcons()
    await userEvent.upload(container.querySelector('input[type="file"]') as HTMLInputElement, pngFile())
    await userEvent.type(screen.getByLabelText('Name (optional)'), 'Mein Logo')
    await userEvent.click(screen.getByRole('button', { name: 'Hochladen' }))
    await waitFor(() => expect(api.callsTo('POST /api/v1/icons')).toHaveLength(1))
    const call = api.callsTo('POST /api/v1/icons')[0]
    expect(call.csrf).toBe('tok-123')
    const form = call.body as FormData
    const part = form.get('file') as File
    expect(part.name).toBe('logo.png')
    expect(part.size).toBe(4)
    expect(form.get('name')).toBe('Mein Logo')
    // The list is fetched again after the upload.
    await waitFor(() => expect(api.callsTo('GET /api/v1/icons').length).toBeGreaterThan(1))
  })

  it('asks for a file before uploading', async () => {
    const api = arrange()
    await openIcons()
    await userEvent.click(screen.getByRole('button', { name: 'Hochladen' }))
    expect(await screen.findByText('Wähle zuerst eine Datei aus.')).toBeInTheDocument()
    expect(api.callsTo('POST /api/v1/icons')).toHaveLength(0)
  })

  it('shows the server error of the file', async () => {
    await useNodeFormData()
    arrange({ 'POST /api/v1/icons': () => problem(400, 'Ungültig', { errors: { file: 'Die SVG-Datei enthält nicht erlaubte Inhalte.' } }) })
    const { container } = await openIcons()
    await userEvent.upload(container.querySelector('input[type="file"]') as HTMLInputElement, pngFile())
    await userEvent.click(screen.getByRole('button', { name: 'Hochladen' }))
    expect(await screen.findByText('Die SVG-Datei enthält nicht erlaubte Inhalte.')).toBeInTheDocument()
  })

  it('deletes after a confirmation', async () => {
    const api = arrange({ 'DELETE /api/v1/icons/ic1': () => noContent() })
    await openIcons()
    await userEvent.click(screen.getByRole('button', { name: 'Logo löschen' }))
    expect(api.callsTo('DELETE /api/v1/icons/ic1')).toHaveLength(0)
    await userEvent.click(screen.getByRole('button', { name: 'Wirklich löschen' }))
    await waitFor(() => expect(api.callsTo('DELETE /api/v1/icons/ic1')).toHaveLength(1))
    expect(api.callsTo('DELETE /api/v1/icons/ic1')[0].csrf).toBe('tok-123')
  })

  it('shows "Das Symbol wird noch verwendet." on 409', async () => {
    arrange({ 'DELETE /api/v1/icons/ic1': () => problem(409, 'Konflikt') })
    await openIcons()
    await userEvent.click(screen.getByRole('button', { name: 'Logo löschen' }))
    await userEvent.click(screen.getByRole('button', { name: 'Wirklich löschen' }))
    const card = within(screen.getByRole('group', { name: 'Logo' }))
    expect(await card.findByText('Das Symbol wird noch verwendet.')).toBeInTheDocument()
    expect(screen.getByRole('group', { name: 'Stern' })).not.toHaveTextContent('wird noch verwendet')
  })
})

describe('icons page delete errors', () => {
  it('shows another failure as it is, not the "in use" text', async () => {
    arrange({ 'DELETE /api/v1/icons/ic1': () => problem(500, 'Serverfehler', { detail: 'Platte voll' }) })
    await openIcons()
    await userEvent.click(screen.getByRole('button', { name: 'Logo löschen' }))
    await userEvent.click(screen.getByRole('button', { name: 'Wirklich löschen' }))
    const card = within(screen.getByRole('group', { name: 'Logo' }))
    expect(await card.findByText(/Platte voll/)).toBeInTheDocument()
    expect(screen.queryByText('Das Symbol wird noch verwendet.')).not.toBeInTheDocument()
  })
})

describe('IconPicker (in the calendar page)', () => {
  const calendarSettings = {
    refreshMinutes: 15,
    calendars: [{ id: 'cal-a', name: 'Familie', icon: { kind: 'none' }, hasUrl: true, urlHint: 'https://x/.../a.ics' }],
    sections: [],
    rules: [],
  }

  async function openCalendar(over: Record<string, Handler> = {}) {
    const api = arrange({ 'GET /api/v1/settings/calendar': () => json(calendarSettings), ...over })
    renderWithProviders('/admin/calendar')
    const card = within(await screen.findByRole('group', { name: 'Kalender 1' }))
    await waitFor(() => expect(api.callsTo('GET /api/v1/icons')).toHaveLength(1))
    return { api, card }
  }

  it('offers none, own icons and the standard set in groups, searchable', async () => {
    const { card } = await openCalendar()
    const picker = card.getByRole('combobox', { name: 'Symbol' })
    await userEvent.click(picker)
    const list = within(document.getElementById(picker.getAttribute('aria-controls') ?? '') as HTMLElement)
    expect(await list.findByText('Eigene Symbole')).toBeInTheDocument()
    expect(list.getByText('Standard')).toBeInTheDocument()
    expect(list.getByRole('option', { name: 'Kein Symbol', hidden: true })).toBeInTheDocument()
    expect(list.getByRole('option', { name: 'Logo', hidden: true })).toBeInTheDocument()
    expect(list.getByRole('option', { name: 'Geburtstag', hidden: true })).toBeInTheDocument()
    await userEvent.clear(picker)
    await userEvent.type(picker, 'Geburt')
    expect(list.queryByRole('option', { name: 'Logo', hidden: true })).not.toBeInTheDocument()
    expect(list.getByRole('option', { name: 'Geburtstag', hidden: true })).toBeInTheDocument()
  })

  it('previews a selected SVG upload as a mask and a PNG upload as an image', async () => {
    const withIcon = (file: string) => ({ ...calendarSettings, calendars: [{ ...calendarSettings.calendars[0], icon: { kind: 'upload', file } }] })
    const svg = await openCalendar({ 'GET /api/v1/settings/calendar': () => json(withIcon('ic2')) })
    await waitFor(() => expect(svg.card.getByRole('combobox', { name: 'Symbol' })).toHaveValue('Stern'))
    const field = svg.card.getByRole('combobox', { name: 'Symbol' }).closest('.mantine-InputWrapper-root') as HTMLElement
    const maskOf = () => [...field.querySelectorAll<HTMLElement>('span[aria-hidden="true"]')].filter((m) => m.style.maskImage)
    expect(maskOf()[0].style.maskImage).toContain('/api/v1/icons/ic2/file')
    expect(field.querySelector('img')).toBeNull()
  })

  it('previews a selected PNG upload as an image', async () => {
    const png = await openCalendar({
      'GET /api/v1/settings/calendar': () => json({ ...calendarSettings, calendars: [{ ...calendarSettings.calendars[0], icon: { kind: 'upload', file: 'ic1' } }] }),
    })
    await waitFor(() => expect(png.card.getByRole('combobox', { name: 'Symbol' })).toHaveValue('Logo'))
    const field = png.card.getByRole('combobox', { name: 'Symbol' }).closest('.mantine-InputWrapper-root') as HTMLElement
    expect(field.querySelector('img')).toHaveAttribute('src', '/api/v1/icons/ic1/file')
    expect([...field.querySelectorAll<HTMLElement>('span')].some((m) => m.style.maskImage)).toBe(false)
  })

  it('picking an uploaded icon and a standard icon sends the matching references', async () => {
    const { api, card } = await openCalendar({
      'PUT /api/v1/settings/calendar': (c) => json({ ...calendarSettings, calendars: [{ ...(c.body as { calendars: object[] }).calendars[0], hasUrl: true, urlHint: 'h' }] }),
    })
    const pick = async (name: string) => {
      const picker = card.getByRole('combobox', { name: 'Symbol' })
      await userEvent.click(picker)
      const list = document.getElementById(picker.getAttribute('aria-controls') ?? '') as HTMLElement
      await userEvent.click(await within(list).findByRole('option', { name, hidden: true }))
    }
    await pick('Logo')
    const bar = screen.getByRole('region', { name: 'Ungespeicherte Änderungen' })
    await userEvent.click(within(bar).getByRole('button', { name: 'Speichern' }))
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/calendar')).toHaveLength(1))
    expect((api.callsTo('PUT /api/v1/settings/calendar')[0].body as { calendars: Array<{ icon: unknown }> }).calendars[0].icon).toEqual({
      kind: 'upload',
      file: 'ic1',
    })
    await pick('Geburtstag')
    await userEvent.click(within(screen.getByRole('region', { name: 'Ungespeicherte Änderungen' })).getByRole('button', { name: 'Speichern' }))
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/calendar')).toHaveLength(2))
    expect((api.callsTo('PUT /api/v1/settings/calendar')[1].body as { calendars: Array<{ icon: unknown }> }).calendars[0].icon).toEqual({
      kind: 'bootstrap',
      name: 'cake2',
    })
  })
})
