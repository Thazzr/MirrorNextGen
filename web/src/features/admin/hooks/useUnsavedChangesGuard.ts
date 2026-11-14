import { useEffect } from 'react'
import { useBlocker } from 'react-router-dom'

/**
 * Holds in-app navigation to another page while `dirty` and registers `beforeunload`
 * for closing or reloading the tab. Needs a data router (`useBlocker` throws otherwise).
 */
export function useUnsavedChangesGuard(dirty: boolean) {
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) => dirty && currentLocation.pathname !== nextLocation.pathname,
  )

  useEffect(() => {
    if (!dirty) return
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault()
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [dirty])

  return {
    asking: blocker.state === 'blocked',
    stay: () => blocker.reset?.(),
    leave: () => blocker.proceed?.(),
  }
}
