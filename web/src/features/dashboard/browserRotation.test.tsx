import { act, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { json, mockApi } from '../../test/mockApi.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

function setWindowSize(width: number, height: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: height })
}

function stubState(rotation: number, testPattern = false) {
  return mockApi({
    'GET /api/v1/kiosk/state': () => json({ rotation, output: '', testPattern }),
    'GET /api/v1/info': () => json({ version: '0.1.0', commit: 'abc1234', buildTime: '' }),
  })
}

const cases = [
  { rotation: 0, window: [1440, 2560], size: '1440px × 2560px', transform: '' },
  { rotation: 90, window: [2560, 1440], size: '1440px × 2560px', transform: 'rotate(90deg) translateY(-100%)' },
  { rotation: 180, window: [1440, 2560], size: '1440px × 2560px', transform: 'rotate(180deg) translate(-100%, -100%)' },
  { rotation: 270, window: [2560, 1440], size: '1440px × 2560px', transform: 'rotate(270deg) translateX(-100%)' },
] as const

describe('browser rotation', () => {
  beforeEach(() => setWindowSize(1440, 2560))
  afterEach(() => vi.unstubAllGlobals())

  it.each(cases)('rotation $rotation sizes and transforms the wrapper', async (c) => {
    setWindowSize(c.window[0], c.window[1])
    stubState(c.rotation)
    renderWithProviders('/?rotation=browser')
    const wrapper = screen.getByTestId('rotated-viewport')
    await waitFor(() => expect(wrapper.style.transform).toBe(c.transform))
    expect(`${wrapper.style.width} × ${wrapper.style.height}`).toBe(c.size)
    expect(wrapper.style.position).toBe('fixed')
    expect(wrapper.style.transformOrigin).toMatch(/^(top left|left top|0(px|%)? 0(px|%)?)$/)
  })

  it('applies no wrapper and no transform without the query parameter', async () => {
    setWindowSize(2560, 1440)
    const api = stubState(90)
    renderWithProviders('/')
    await waitFor(() => expect(api.callsTo('GET /api/v1/kiosk/state')).toHaveLength(1))
    await act(async () => {})
    expect(screen.queryByTestId('rotated-viewport')).not.toBeInTheDocument()
  })

  it('follows window resizes', async () => {
    setWindowSize(2560, 1440)
    stubState(90)
    renderWithProviders('/?rotation=browser')
    const wrapper = screen.getByTestId('rotated-viewport')
    await waitFor(() => expect(wrapper.style.transform).toBe('rotate(90deg) translateY(-100%)'))
    act(() => {
      setWindowSize(1920, 1080)
      window.dispatchEvent(new Event('resize'))
    })
    expect(`${wrapper.style.width} × ${wrapper.style.height}`).toBe('1080px × 1920px')
  })

  it('shows the content size and the browser rotation in the overlay', async () => {
    setWindowSize(2560, 1440)
    stubState(90, true)
    renderWithProviders('/?rotation=browser')
    expect(await screen.findByText('1440 × 2560')).toBeInTheDocument()
    expect(screen.getByText('Hochformat')).toBeInTheDocument()
    expect(screen.getByText('Browser 90°')).toBeInTheDocument()
  })

  it('shows the monitor rotation and the window size when the compositor rotates', async () => {
    setWindowSize(1440, 2560)
    stubState(90, true)
    renderWithProviders('/')
    expect(await screen.findByText('Monitor 90°')).toBeInTheDocument()
    expect(screen.getByText('1440 × 2560')).toBeInTheDocument()
  })

  it('falls back to rotation 0 when the kiosk state fails', async () => {
    setWindowSize(1440, 2560)
    const api = mockApi({
      'GET /api/v1/kiosk/state': () => new Response(JSON.stringify({ title: 'Kaputt', status: 500 }), { status: 500 }),
    })
    renderWithProviders('/?rotation=browser&diagnose')
    await waitFor(() => expect(api.callsTo('GET /api/v1/kiosk/state')).toHaveLength(1))
    await act(async () => {})
    const wrapper = screen.getByTestId('rotated-viewport')
    expect(wrapper.style.transform).toBe('')
    expect(`${wrapper.style.width} × ${wrapper.style.height}`).toBe('1440px × 2560px')
    expect(screen.getByText('Browser 0°')).toBeInTheDocument()
  })
})
