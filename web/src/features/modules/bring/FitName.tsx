import { useLayoutEffect, useRef } from 'react'

/** The smallest the name may get, as a share of its normal size. */
const minScale = 0.7
const step = 0.5

/**
 * Shrinks `el` until its widest word fits, in steps of half a pixel. The line height stays that of the normal
 * size (set in px) so a shrunk name keeps the line of its neighbours. When even the smallest size does not fit,
 * the word may break inside itself - text is never clipped.
 */
function fit(el: HTMLElement) {
  el.style.fontSize = ''
  el.style.lineHeight = ''
  el.classList.remove('mirror-bring__name--break')
  if (el.scrollWidth <= el.clientWidth) return
  const computed = getComputedStyle(el)
  const base = parseFloat(computed.fontSize)
  if (!Number.isFinite(base)) return
  const lineHeight = parseFloat(computed.lineHeight)
  const linePx = Number.isFinite(lineHeight) ? lineHeight : base * 1.2
  const floor = base * minScale
  el.style.lineHeight = `${linePx}px`
  let size = base
  while (size - step >= floor - 1e-9) {
    size -= step
    el.style.fontSize = `${size}px`
    if (el.scrollWidth <= el.clientWidth) return
  }
  el.style.fontSize = `${floor}px`
  el.classList.add('mirror-bring__name--break')
}

/** An item name that wraps at spaces and shrinks, instead of being cut off, when a single word is too wide. */
export function FitName({ name }: { name: string }) {
  const ref = useRef<HTMLSpanElement>(null)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    let active = true
    const run = () => {
      if (active) fit(el)
    }
    run()
    let observer: ResizeObserver | undefined
    if (typeof ResizeObserver !== 'undefined') {
      let lastWidth = el.clientWidth
      // Fitting changes the element's own height, not its width; comparing widths avoids a feedback loop.
      observer = new ResizeObserver(() => {
        if (el.clientWidth === lastWidth) return
        lastWidth = el.clientWidth
        run()
      })
      observer.observe(el)
    }
    // The font loads late via @font-face and changes every measurement.
    if (typeof document !== 'undefined' && document.fonts?.ready) void document.fonts.ready.then(run)
    return () => {
      active = false
      observer?.disconnect()
    }
  }, [name])
  return (
    <span ref={ref} className="mirror-bring__name">
      {name}
    </span>
  )
}
