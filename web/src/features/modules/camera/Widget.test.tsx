import { QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi, problem } from '../../../test/mockApi.ts'
import { layoutOf } from '../../../test/dashboardFixtures.ts'
import { renderWithProviders } from '../../../test/renderWithProviders.tsx'
import { createQueryClient } from '../../../shared/api/queryClient.ts'
import { cameraFilter } from './api.ts'
import { CameraWidget } from './Widget.tsx'

const camera = { configured: true, title: 'Haustür', brightness: 110, contrast: 100, saturation: 90, grayscale: true }

/** Stands in for the browser's WebSocket: records the addresses the player opens and never connects. */
class FakeWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3
  static opened: string[] = []
  binaryType = ''
  constructor(url: string) {
    FakeWebSocket.opened.push(url)
  }
  addEventListener() {}
  send() {}
  close() {}
}

function setup(dashboardCamera: unknown | Response, region: 'topRight' | 'topLeft' | 'middle' | 'bottomLeft' | 'bottomRight' = 'topRight') {
  return mockApi({
    'GET /api/v1/kiosk/state': () => json({ rotation: 0, output: '', testPattern: false }),
    'GET /api/v1/dashboard/layout': () => json(layoutOf({ [region]: ['camera'] })),
    'GET /api/v1/dashboard/camera': () => (dashboardCamera instanceof Response ? dashboardCamera.clone() : json(dashboardCamera)),
  })
}

const settled = () => new Promise((r) => setTimeout(r, 20))

describe('camera widget', () => {
  beforeEach(() => {
    FakeWebSocket.opened = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('shows the title and plays the mirror stream in MSE mode with the filters applied to the picture', async () => {
    setup(camera)
    renderWithProviders('/')
    const aside = await screen.findByRole('complementary', { name: 'Haustür' })
    expect(aside.querySelector('.mirror-camera__title')).toHaveTextContent('Haustür')

    const host = aside.querySelector<HTMLElement>('.mirror-camera__video')!
    expect(host.style.filter).toBe('brightness(110%) contrast(100%) saturate(90%) grayscale(1)')
    // The title must not be filtered with the picture.
    expect(aside.style.filter).toBe('')

    const player = host.querySelector('video-rtc') as (HTMLElement & { mode: string; media: string }) | null
    expect(player).not.toBeNull()
    expect(player!.mode).toBe('mse')
    expect(player!.media).toBe('video')
    const video = player!.querySelector('video')!
    expect(video.muted).toBe(true)
    expect(video.controls).toBe(false)
    expect(video.autoplay).toBe(true)

    // The stream comes from the mirror's own proxy route, as ws:// or wss:// of the page's origin.
    await waitFor(() => expect(FakeWebSocket.opened).toEqual([`ws${location.origin.substring(4)}/api/v1/camera/stream`]))
  })

  it('leaves out grayscale unless switched on', async () => {
    setup({ ...camera, grayscale: false, brightness: 100, saturation: 100 })
    renderWithProviders('/')
    const aside = await screen.findByRole('complementary', { name: 'Haustür' })
    expect(aside.querySelector<HTMLElement>('.mirror-camera__video')!.style.filter).toBe('brightness(100%) contrast(100%) saturate(100%)')
  })

  it('renders nothing and opens no stream when not configured', async () => {
    const api = setup({ ...camera, configured: false })
    renderWithProviders('/')
    await vi.waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(1))
    await settled()
    expect(document.querySelector('.mirror-camera')).toBeNull()
    expect(FakeWebSocket.opened).toEqual([])
  })

  it('renders nothing, not even an error text, when the request fails', async () => {
    const api = setup(problem(500, 'Kaputt'))
    renderWithProviders('/')
    await vi.waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(1))
    await settled()
    expect(document.querySelector('.mirror-camera')).toBeNull()
    expect(screen.queryByText(/Kaputt/)).not.toBeInTheDocument()
  })

  it('removes the player element when the widget unmounts', async () => {
    setup(camera)
    const view = renderWithProviders('/')
    await screen.findByRole('complementary', { name: 'Haustür' })
    expect(document.querySelector('video-rtc')).not.toBeNull()
    view.unmount()
    expect(document.querySelector('video-rtc')).toBeNull()
  })

  it('keeps exactly one player when effects run twice (cleanup removes the first)', async () => {
    setup(camera)
    const queryClient = createQueryClient({ retry: false })
    const { container } = render(
      <StrictMode>
        <QueryClientProvider client={queryClient}>
          <CameraWidget region="topRight" />
        </QueryClientProvider>
      </StrictMode>,
    )
    await screen.findByRole('complementary', { name: 'Haustür' })
    expect(container.querySelectorAll('video-rtc')).toHaveLength(1)
  })

  it('leaves its own container empty when the request fails', async () => {
    const api = setup(problem(500, 'Kaputt'))
    const queryClient = createQueryClient({ retry: false })
    const { container } = render(
      <QueryClientProvider client={queryClient}>
        <CameraWidget region="topRight" />
      </QueryClientProvider>,
    )
    await vi.waitFor(() => expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(1))
    await settled()
    expect(container.innerHTML).toBe('')
  })

  it('has no title line for an empty title', async () => {
    setup({ ...camera, title: '' })
    renderWithProviders('/')
    const aside = await screen.findByRole('complementary', { name: 'Kamera' })
    expect(aside.querySelector('.mirror-camera__title')).toBeNull()
  })

  it('aligns left in left regions and right everywhere else, like the weather', async () => {
    for (const [region, left] of [
      ['topLeft', true],
      ['bottomLeft', true],
      ['bottomRight', false],
      ['topRight', false],
      ['middle', false],
    ] as const) {
      setup(camera, region)
      const view = renderWithProviders('/')
      // The default layout (camera top right) shows before the stored one arrives: wait for the region itself.
      await waitFor(() => expect(document.querySelector(`[data-region="${region}"] .mirror-camera`)).not.toBeNull())
      const aside = document.querySelector(`[data-region="${region}"] .mirror-camera`)!
      expect(aside.classList.contains('mirror-camera--left'), region).toBe(left)
      view.unmount()
    }
  })
})

describe('camera widget refresh', () => {
  beforeEach(() => vi.stubGlobal('WebSocket', FakeWebSocket))
  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('loads the dashboard camera again after 60 seconds, not before', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'setTimeout', 'clearTimeout'] })
    const api = setup(camera)
    renderWithProviders('/')
    await vi.advanceTimersByTimeAsync(100)
    expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(59_000)
    expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(1_500)
    expect(api.callsTo('GET /api/v1/dashboard/camera')).toHaveLength(2)
  })
})

describe('cameraFilter', () => {
  it('writes the three percentages and grayscale only when asked', () => {
    expect(cameraFilter({ brightness: 0, contrast: 200, saturation: 50, grayscale: false })).toBe('brightness(0%) contrast(200%) saturate(50%)')
    expect(cameraFilter({ brightness: 100, contrast: 100, saturation: 100, grayscale: true })).toBe(
      'brightness(100%) contrast(100%) saturate(100%) grayscale(1)',
    )
  })
})
