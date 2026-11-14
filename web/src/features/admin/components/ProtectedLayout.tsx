import { Alert, Button, Center, Container, Loader, Stack } from '@mantine/core'
import { Navigate, useLocation } from 'react-router-dom'
import { useSession } from '../api/session.ts'
import { AdminLayout } from './AdminLayout.tsx'

/** Route guard: only a signed-in session reaches the pages inside; everything else is redirected. */
export function ProtectedLayout() {
  const session = useSession()
  const location = useLocation()

  if (session.isPending) {
    return (
      <Center mih="100vh">
        <Loader aria-label="Wird geladen" />
      </Center>
    )
  }
  if (session.isError) {
    return (
      <Container size={420} py="xl">
        <Stack>
          <Alert color="red" title="Der Server antwortet nicht.">
            {session.error.message}
          </Alert>
          <Button onClick={() => void session.refetch()}>Erneut versuchen</Button>
        </Stack>
      </Container>
    )
  }
  if (session.data.setupRequired) return <Navigate to="/admin/setup" replace />
  if (!session.data.authenticated) {
    return <Navigate to="/admin/login" replace state={{ from: location.pathname + location.search }} />
  }
  return <AdminLayout />
}
