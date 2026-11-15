import { cleanup, render } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { FitName } from './FitName.tsx'

const clientWidth = 100
const charWidthPerPx = 0.5 // widest word of n chars at s px is n * s * 0.5 wide

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(clientWidth)
  vi.spyOn(HTMLElement.prototype, 'scrollWidth', 'get').mockImplementation(function (this: HTMLElement) {
    const size = parseFloat(this.style.fontSize) || 16
    const longest = Math.max(...(this.textContent ?? '').split(' ').map((w) => w.length))
    return longest * size * charWidthPerPx
  })
  vi.spyOn(window, 'getComputedStyle').mockReturnValue({ fontSize: '16px', lineHeight: '19.2px' } as CSSStyleDeclaration)
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('FitName', () => {
  it('sets no inline style when the name fits', () => {
    const { container } = render(<FitName name="Milch" />)
    const el = container.querySelector('.mirror-bring__name') as HTMLElement
    expect(el.style.fontSize).toBe('')
    expect(el.style.lineHeight).toBe('')
    expect(el.classList.contains('mirror-bring__name--break')).toBe(false)
  })

  it('shrinks a too wide word until it fits and keeps the base line height', () => {
    // 15 chars: 120px at 16px, needs <= 13.33px
    const { container } = render(<FitName name="Fünfzehnzeichen" />)
    const el = container.querySelector('.mirror-bring__name') as HTMLElement
    const size = parseFloat(el.style.fontSize)
    expect(size).toBeLessThan(16)
    expect(size).toBeGreaterThanOrEqual(16 * 0.7)
    expect(15 * size * charWidthPerPx).toBeLessThanOrEqual(clientWidth)
    expect(15 * (size + 0.5) * charWidthPerPx).toBeGreaterThan(clientWidth)
    expect(el.style.lineHeight).toBe('19.2px')
    expect(el.classList.contains('mirror-bring__name--break')).toBe(false)
  })

  it('stops at 70 percent and allows breaking inside the word', () => {
    const { container } = render(<FitName name="Donaudampfschifffahrtsgesellschaft" />)
    const el = container.querySelector('.mirror-bring__name') as HTMLElement
    expect(parseFloat(el.style.fontSize)).toBeCloseTo(16 * 0.7, 5)
    expect(el.classList.contains('mirror-bring__name--break')).toBe(true)
  })

  it('removes the inline style again when the name changes to a short one', () => {
    const { container, rerender } = render(<FitName name="Fünfzehnzeichen" />)
    const el = container.querySelector('.mirror-bring__name') as HTMLElement
    expect(el.style.fontSize).not.toBe('')
    rerender(<FitName name="Ei" />)
    expect(el.style.fontSize).toBe('')
    expect(el.style.lineHeight).toBe('')
  })
})
