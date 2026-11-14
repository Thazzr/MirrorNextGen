import { useEffect, useState } from 'react'
import type { CSSProperties } from 'react'

export type Rotation = 0 | 90 | 180 | 270

export type ContentSize = { width: number; height: number }

type Viewport = { width: number; height: number }

function readViewport(): Viewport {
  return { width: window.innerWidth, height: window.innerHeight }
}

// Each transform moves the rotated box (origin top left) back onto the viewport.
const transforms: Record<Rotation, string | undefined> = {
  0: undefined,
  90: 'rotate(90deg) translateY(-100%)',
  180: 'rotate(180deg) translate(-100%, -100%)',
  270: 'rotate(270deg) translateX(-100%)',
}

export function normalizeRotation(value: number | undefined): Rotation {
  return value === 90 || value === 180 || value === 270 ? value : 0
}

/**
 * Fallback for compositors that cannot rotate the output (KIOSK_ROTATION=browser):
 * the window has the panel's native size, the content is laid out at the swapped
 * size and rotated into place. Returns the wrapper style and the content size.
 */
export function useBrowserRotation(rotation: Rotation): { size: ContentSize; style: CSSProperties } {
  const [viewport, setViewport] = useState(readViewport)
  useEffect(() => {
    const onResize = () => setViewport(readViewport())
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  const swap = rotation === 90 || rotation === 270
  const size = swap ? { width: viewport.height, height: viewport.width } : viewport
  return {
    size,
    style: {
      position: 'fixed',
      top: 0,
      left: 0,
      width: `${size.width}px`,
      height: `${size.height}px`,
      transformOrigin: 'top left',
      transform: transforms[rotation],
    },
  }
}
