import { useLocation } from 'react-router-dom'

/** Where a signed-in user who opens login or setup goes: the page they asked for before, else the overview. */
export function useReturnPath(): string {
  const state = useLocation().state as { from?: unknown } | null
  return typeof state?.from === 'string' && state.from.startsWith('/admin') ? state.from : '/admin'
}
