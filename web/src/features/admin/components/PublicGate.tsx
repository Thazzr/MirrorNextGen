import type { ReactNode } from 'react'
import { Center, Loader } from '@mantine/core'
import { Navigate } from 'react-router-dom'
import { useSession } from '../api/session.ts'
import { useReturnPath } from '../hooks/useReturnPath.ts'

/** Guard of login and setup: shows its page only in the matching session state. */
export function PublicGate({ page, children }: { page: 'login' | 'setup'; children: ReactNode }) {
  const session = useSession()
  const returnTo = useReturnPath()

  if (session.isPending) {
    return (
      <Center mih="100vh">
        <Loader aria-label="Wird geladen" />
      </Center>
    )
  }
  // A failed session call still lets the user try the form; the call itself reports the error.
  if (session.isError) return <>{children}</>
  const { setupRequired, authenticated } = session.data
  if (authenticated) return <Navigate to={returnTo} replace />
  if (page === 'login' && setupRequired) return <Navigate to="/admin/setup" replace />
  if (page === 'setup' && !setupRequired) return <Navigate to="/admin/login" replace />
  return <>{children}</>
}
