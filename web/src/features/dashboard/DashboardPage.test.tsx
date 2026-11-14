import { act, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

describe('DashboardPage', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    // 09:05:07 in Europe/Berlin (CET, UTC+1)
    vi.setSystemTime(new Date('2026-11-14T08:05:07Z'))
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows date, time, seconds and calendar week', () => {
    renderWithProviders('/')
    expect(screen.getByText('Samstag, 14. November 2026')).toBeInTheDocument()
    expect(screen.getByText('09:05')).toBeInTheDocument()
    expect(screen.getByText('07')).toBeInTheDocument()
    expect(screen.getByText('KW 46')).toBeInTheDocument()
  })

  it('ticks every second', () => {
    renderWithProviders('/')
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByText('08')).toBeInTheDocument()
  })

  it('marks the body as kiosk only while mounted', () => {
    const { unmount } = renderWithProviders('/')
    expect(document.body).toHaveClass('mirror-kiosk')
    unmount()
    expect(document.body).not.toHaveClass('mirror-kiosk')
  })

  it('shows no diagnose overlay without the query parameter', () => {
    renderWithProviders('/')
    expect(screen.queryByTestId('diagnose')).not.toBeInTheDocument()
  })

  it('has exactly one h1', () => {
    renderWithProviders('/')
    expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1)
  })
})
