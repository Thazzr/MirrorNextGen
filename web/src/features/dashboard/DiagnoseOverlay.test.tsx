import { act, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

function setWindowSize(width: number, height: number) {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
  Object.defineProperty(window, 'innerHeight', { configurable: true, value: height })
}

function stubInfo(response: Response) {
  vi.stubGlobal('fetch', vi.fn(async () => response))
}

describe('DiagnoseOverlay', () => {
  beforeEach(() => {
    setWindowSize(1440, 2560)
    stubInfo(Response.json({ version: '0.1.0', commit: 'abc1234', buildTime: '' }))
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows window size and orientation and follows resizes', async () => {
    renderWithProviders('/?diagnose')
    expect(await screen.findByText('1440 × 2560')).toBeInTheDocument()
    expect(screen.getByText('Hochformat')).toBeInTheDocument()

    act(() => {
      setWindowSize(2560, 1440)
      window.dispatchEvent(new Event('resize'))
    })
    expect(screen.getByText('2560 × 1440')).toBeInTheDocument()
    expect(screen.getByText('Querformat')).toBeInTheDocument()
  })

  it('shows the server version and commit', async () => {
    renderWithProviders('/?diagnose')
    expect(screen.getByText('wird geladen …')).toBeInTheDocument()
    expect(await screen.findByText('0.1.0 (abc1234)')).toBeInTheDocument()
  })

  it('shows an error when the server answers with 500', async () => {
    stubInfo(
      new Response(JSON.stringify({ title: 'Kaputt', status: 500 }), {
        status: 500,
        headers: { 'Content-Type': 'application/problem+json' },
      }),
    )
    renderWithProviders('/?diagnose')
    expect(await screen.findByText('Server nicht erreichbar: Kaputt')).toBeInTheDocument()
  })
})
