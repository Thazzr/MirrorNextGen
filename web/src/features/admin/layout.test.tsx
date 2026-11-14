import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, problem, type Handler } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { preloadAdmin } from '../../test/preloadAdmin.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

preloadAdmin()
afterEach(() => vi.unstubAllGlobals())

const stored = { topLeft: ['clock', 'weather'], topRight: [], middle: [], bottomLeft: [], bottomRight: ['wifi'] }
const modules = [
  { key: 'clock', name: 'Uhr' },
  { key: 'weather', name: 'Wetter' },
  { key: 'wifi', name: 'WLAN-QR-Code' },
  { key: 'camera', name: 'Kamera' },
]

function arrange(put?: Handler) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'GET /api/v1/settings/layout': () => json(stored),
    'GET /api/v1/modules': () => json(modules),
    ...(put ? { 'PUT /api/v1/settings/layout': put } : {}),
  })
}

async function open() {
  const view = renderWithProviders('/admin/layout')
  await screen.findByRole('region', { name: 'Oben links' })
  return view
}

const region = (name: string) => within(screen.getByRole('region', { name }))
const bar = () => screen.queryByRole('region', { name: 'Ungespeicherte Änderungen' })
const itemNames = (name: string) =>
  region(name)
    .queryAllByRole('button', { name: /^Nach oben: / })
    .map((b) => b.getAttribute('aria-label')!.replace('Nach oben: ', ''))

describe('layout page', () => {
  it('renders the five regions with module names, one h1 and the unplaced modules', async () => {
    arrange()
    await open()
    expectSinglePageTitle('Anordnung')
    expect(screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent)).toEqual([
      'Oben links',
      'Oben rechts',
      'Mitte',
      'Unten links',
      'Unten rechts',
    ])
    expect(itemNames('Oben links')).toEqual(['Uhr', 'Wetter'])
    expect(itemNames('Unten rechts')).toEqual(['WLAN-QR-Code'])
    expect(region('Mitte').getByText('Leer')).toBeInTheDocument()
    expect(screen.getByText('Nicht angezeigt: Kamera')).toBeInTheDocument()
    expect(bar()).not.toBeInTheDocument()
  })

  it('moves a module down and up, disabling the ends', async () => {
    arrange()
    await open()
    expect(region('Oben links').getByRole('button', { name: 'Nach oben: Uhr' })).toBeDisabled()
    expect(region('Oben links').getByRole('button', { name: 'Nach unten: Wetter' })).toBeDisabled()
    await userEvent.click(region('Oben links').getByRole('button', { name: 'Nach unten: Uhr' }))
    expect(itemNames('Oben links')).toEqual(['Wetter', 'Uhr'])
    expect(bar()).toHaveTextContent('Ungespeicherte Änderungen: Anordnung')
    await userEvent.click(region('Oben links').getByRole('button', { name: 'Nach oben: Uhr' }))
    expect(itemNames('Oben links')).toEqual(['Uhr', 'Wetter'])
    expect(bar()).not.toBeInTheDocument()
  })

  it('moves a module to another region through the menu', async () => {
    arrange()
    await open()
    await userEvent.click(region('Oben links').getByRole('button', { name: 'Verschieben nach …: Uhr' }))
    // hidden: true skips the visibility check, which walks computed styles of the whole page in jsdom.
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Mitte', hidden: true }))
    expect(itemNames('Oben links')).toEqual(['Wetter'])
    expect(itemNames('Mitte')).toEqual(['Uhr'])
  })

  it('removes a module, which then appears under "Nicht angezeigt"', async () => {
    arrange()
    await open()
    await userEvent.click(region('Unten rechts').getByRole('button', { name: 'Entfernen: WLAN-QR-Code' }))
    expect(itemNames('Unten rechts')).toEqual([])
    expect(screen.getByText('Nicht angezeigt: WLAN-QR-Code, Kamera')).toBeInTheDocument()
  })

  it('adds an unplaced module via the select and hides the select when none is left', async () => {
    arrange()
    await open()
    await userEvent.click(region('Mitte').getByRole('combobox', { name: 'Modul hinzufügen' }))
    // Every card keeps its closed option list mounted, so pick the open one's option by keyboard.
    await userEvent.keyboard('{ArrowDown}{Enter}')
    expect(itemNames('Mitte')).toEqual(['Kamera'])
    expect(screen.getByText('Nicht angezeigt: nichts')).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Modul hinzufügen' })).not.toBeInTheDocument()
  })

  it('saves the full layout with the CSRF token', async () => {
    const api = arrange((call) => json(call.body))
    await open()
    await userEvent.click(region('Oben links').getByRole('button', { name: 'Nach unten: Uhr' }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    await waitFor(() => expect(api.callsTo('PUT /api/v1/settings/layout')).toHaveLength(1))
    const put = api.callsTo('PUT /api/v1/settings/layout')[0]
    expect(put.body).toEqual({ ...stored, topLeft: ['weather', 'clock'] })
    expect(put.csrf).toBe('tok-123')
    await waitFor(() => expect(bar()).not.toBeInTheDocument())
    expect(itemNames('Oben links')).toEqual(['Wetter', 'Uhr'])
  })

  it('invalidates the dashboard layout after saving so the mirror picks it up', async () => {
    arrange((call) => json(call.body))
    const { queryClient } = await open()
    // No dashboard is mounted here; seeded data stands in for the mirror's cached layout.
    queryClient.setQueryData(['dashboard', 'layout'], stored)
    expect(queryClient.getQueryState(['dashboard', 'layout'])?.isInvalidated).toBe(false)
    await userEvent.click(region('Oben links').getByRole('button', { name: 'Nach unten: Uhr' }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    await waitFor(() => expect(queryClient.getQueryState(['dashboard', 'layout'])?.isInvalidated).toBe(true))
  })

  it('shows a server error in the matching region card and keeps the draft', async () => {
    arrange(() => problem(400, 'Ungültig', { errors: { middle: 'Höchstens 12 Module.' } }))
    await open()
    await userEvent.click(region('Oben links').getByRole('button', { name: 'Nach unten: Uhr' }))
    await userEvent.click(within(bar()!).getByRole('button', { name: 'Speichern' }))
    expect(await region('Mitte').findByText('Höchstens 12 Module.')).toBeInTheDocument()
    expect(region('Oben links').queryByText('Höchstens 12 Module.')).not.toBeInTheDocument()
    expect(itemNames('Oben links')).toEqual(['Wetter', 'Uhr'])
    expect(bar()).toBeInTheDocument()
  })

  it('asks before leaving with unsaved changes', async () => {
    arrange()
    const { router } = await open()
    await userEvent.click(region('Oben links').getByRole('button', { name: 'Nach unten: Uhr' }))
    await userEvent.click(screen.getByRole('link', { name: '← Übersicht' }))
    expect(await screen.findByText('Änderungen verwerfen?')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/admin/layout')
  })
})
