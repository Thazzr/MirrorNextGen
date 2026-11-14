import { useLayoutEffect, useRef, useState } from 'react'

/** Width in CSS pixels of the referenced element, kept current by a ResizeObserver; `fallback` until measured (and where no observer exists). */
export function useElementWidth<T extends HTMLElement>(fallback: number) {
  const ref = useRef<T>(null)
  const [width, setWidth] = useState(fallback)
  useLayoutEffect(() => {
    const element = ref.current
    if (!element) return
    const measured = element.getBoundingClientRect().width
    if (measured > 0) setWidth(measured)
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver((entries) => {
      const next = entries[entries.length - 1]?.contentRect.width
      if (next && next > 0) setWidth(next)
    })
    observer.observe(element)
    return () => observer.disconnect()
  }, [])
  return [ref, width] as const
}
