import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../../test/mockApi.ts'
import { layoutOf } from '../../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../../test/renderWithProviders.tsx'

type Item = { name: string; specification?: string; iconUrl: string }

const item = (name: string, extra: Partial<Item> = {}): Item => ({ name, iconUrl: `https://web.getbring.com/assets/images/items/${name.toLowerCase()}.png`, ...extra })
const names = (n: number) => Array.from({ length: n }, (_, i) => item(`Artikel${i + 1}`))

function setup(bring: unknown | Response) {
  return mockApi({
    'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
    'GET /api/v1/dashboard/layout': () => json(layoutOf({ bottomLeft: ['bring'] })),
    'GET /api/v1/dashboard/bring': () => (bring instanceof Response ? bring.clone() : json(bring)),
  })
}

const state = (items: Item[], extra: object = {}) => ({ configured: true, stale: false, items, ...extra })

async function settled(api: ReturnType<typeof setup>) {
  await vi.waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/bring')).toHaveLength(1))
  await new Promise((r) => setTimeout(r, 20))
}

describe('bring widget', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('puts 13 items into rows of 6, 6 and 1, with the first row at the bottom (column-reverse)', async () => {
    setup(state(names(13)))
    const { container } = renderWithProviders('/')
    await screen.findByText('Artikel1')
    const rows = [...container.querySelectorAll('.mirror-bring__list')]
    expect(rows.map((r) => within(r as HTMLElement).getAllByRole('listitem').length)).toEqual([6, 6, 1])
    // DOM order is item order; the bottom-up stacking is the CSS of the rows wrapper.
    expect(within(rows[0] as HTMLElement).getAllByRole('listitem')[0]).toHaveTextContent('Artikel1')
    expect(within(rows[2] as HTMLElement).getByRole('listitem')).toHaveTextContent('Artikel13')
    expect(container.querySelector('.mirror-bring__rows')).toBeInTheDocument()
  })

  it('shows at most 18 items, even when the server delivers more', async () => {
    setup(state(names(25)))
    const { container } = renderWithProviders('/')
    await screen.findByText('Artikel1')
    expect(container.querySelectorAll('.mirror-bring__item')).toHaveLength(18)
    expect(container.querySelectorAll('.mirror-bring__list')).toHaveLength(3)
    expect(screen.getByText('Artikel18')).toBeInTheDocument()
    expect(screen.queryByText('Artikel19')).not.toBeInTheDocument()
  })

  it('shows name, specification and the picture of the item', async () => {
    setup(state([item('Milch', { specification: '2 Liter' }), item('Brot')]))
    const { container } = renderWithProviders('/')
    const milk = (await screen.findByText('Milch')).closest('li') as HTMLElement
    expect(within(milk).getByText('2 Liter')).toBeInTheDocument()
    expect(milk.querySelector('img')).toHaveAttribute('src', 'https://web.getbring.com/assets/images/items/milch.png')
    const bread = screen.getByText('Brot').closest('li') as HTMLElement
    expect(bread.querySelector('.mirror-bring__specification')).toBeNull()
    expect(container.querySelectorAll('img')).toHaveLength(2)
  })

  it('falls back to the first letter, upper-cased, when the picture fails', async () => {
    setup(state([item('ätherisches Öl')]))
    const { container } = renderWithProviders('/')
    const img = await waitFor(() => {
      const el = container.querySelector('img')
      expect(el).not.toBeNull()
      return el as HTMLImageElement
    })
    fireEvent.error(img)
    await waitFor(() => expect(container.querySelector('img')).toBeNull())
    expect(container.querySelector('.mirror-bring__fallback')).toHaveTextContent('Ä')
  })

  it('shows an emoji-only name as the emoji, also without a picture address', async () => {
    setup(state([{ name: '🥛', iconUrl: '' }, { name: '🥛 Milch', iconUrl: '' }]))
    const { container } = renderWithProviders('/')
    await screen.findByText('🥛 Milch')
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('.mirror-bring__emoji')).toHaveTextContent('🥛')
    // A name with more than the emoji is a normal name: its first character is the fallback.
    expect(container.querySelectorAll('.mirror-bring__emoji')).toHaveLength(1)
    expect(container.querySelectorAll('.mirror-bring__fallback')).toHaveLength(1)
  })

  it('renders nothing when not configured', async () => {
    const api = setup({ configured: false, stale: false, items: [] })
    renderWithProviders('/')
    await settled(api)
    expect(document.querySelector('.mirror-bring')).toBeNull()
    expect(screen.queryByRole('complementary', { name: 'Einkaufsliste' })).not.toBeInTheDocument()
  })

  it('renders nothing for an empty list', async () => {
    const api = setup(state([]))
    renderWithProviders('/')
    await settled(api)
    expect(document.querySelector('.mirror-bring')).toBeNull()
  })

  it('shows the error text of the body in place of the list', async () => {
    setup(state([], { error: 'Anmeldung bei Bring! fehlgeschlagen.' }))
    const { container } = renderWithProviders('/')
    const message = await screen.findByText('Anmeldung bei Bring! fehlgeschlagen.')
    expect(message).toHaveClass('mirror-error')
    expect(container.querySelector('.mirror-bring__item')).toBeNull()
  })

  it('shows an error text when the request itself fails', async () => {
    setup(problem(500, 'Kaputt'))
    renderWithProviders('/')
    expect(await screen.findByText('Die Bring!-Liste konnte nicht geladen werden.')).toHaveClass('mirror-error')
  })

  it('keeps showing stale items', async () => {
    setup(state([item('Milch')], { stale: true }))
    renderWithProviders('/')
    expect(await screen.findByText('Milch')).toBeInTheDocument()
  })
})
