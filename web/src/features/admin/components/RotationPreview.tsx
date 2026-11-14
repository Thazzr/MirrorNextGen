/** Small monitor outline turned by `degrees`; the arrow points to "up" of the picture. */
export function RotationPreview({ degrees }: { degrees: number }) {
  return (
    <svg width="48" height="48" viewBox="0 0 48 48" aria-hidden="true" focusable="false">
      <g transform={`rotate(${degrees} 24 24)`} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="6" y="11" width="36" height="22" rx="2" />
        <path d="M18 38h12" />
        <path d="M24 28V16M20 20l4-4 4 4" />
      </g>
    </svg>
  )
}
