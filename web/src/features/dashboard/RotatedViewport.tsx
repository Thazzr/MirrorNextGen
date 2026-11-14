import type { CSSProperties, ReactNode } from 'react'

/** Fixed wrapper that carries the browser rotation; see useBrowserRotation. */
export function RotatedViewport({ style, children }: { style: CSSProperties; children: ReactNode }) {
  return (
    <div className="rotated-viewport" data-testid="rotated-viewport" style={style}>
      {children}
    </div>
  )
}
