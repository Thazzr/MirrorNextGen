import { useEffect, useRef } from 'react'
import { Alert, Center, Container, Loader, Stack, Anchor } from '@mantine/core'
import { useMutation } from '@tanstack/react-query'
import { Link, Navigate } from 'react-router-dom'
import { logout } from '../api/auth.ts'
import { useSession, useSetSession } from '../api/session.ts'
import { ApiError } from '../../../shared/api/client.ts'

/**
 * Logging out is a route of its own, so that the unsaved-changes guard of the page
 * left behind can hold it like any other navigation.
 */
export function LogoutPage() {
  const session = useSession()
  const setSession = useSetSession()
  const started = useRef(false)
  const mutation = useMutation({
    mutationFn: logout,
    onSettled: (_data, error) => {
      // A 401 means the session was already over, which is what we wanted.
      if (!error || (error instanceof ApiError && error.status === 401)) {
        setSession({ setupRequired: false, authenticated: false })
      }
    },
  })

  useEffect(() => {
    if (started.current) return
    started.current = true
    mutation.mutate()
  }, [mutation])

  if (session.data && !session.data.authenticated) return <Navigate to="/admin/login" replace />
  if (mutation.isError) {
    return (
      <Container size={420} py="xl">
        <Stack>
          <Alert color="red" title="Abmelden fehlgeschlagen.">
            {mutation.error.message}
          </Alert>
          <Anchor component={Link} to="/admin">
            Zurück zur Übersicht
          </Anchor>
        </Stack>
      </Container>
    )
  }
  return (
    <Center mih="100vh">
      <Loader aria-label="Du wirst abgemeldet" />
    </Center>
  )
}
