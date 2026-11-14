import { Alert, Button, PasswordInput, Stack } from '@mantine/core'
import { useForm } from '@mantine/form'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { login } from '../api/auth.ts'
import { useSetSession } from '../api/session.ts'
import { PublicPage } from '../components/PublicPage.tsx'
import { ApiError } from '../../../shared/api/client.ts'

function waitText(seconds: number): string {
  const minutes = Math.max(1, Math.ceil(seconds / 60))
  return minutes === 1 ? 'in einer Minute' : `in ${minutes} Minuten`
}

export function LoginPage() {
  const form = useForm({ initialValues: { password: '' } })
  const setSession = useSetSession()
  const navigate = useNavigate()
  const mutation = useMutation({
    mutationFn: login,
    // The guard around this page redirects to the requested page once the session is cached.
    onSuccess: setSession,
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) {
        form.setFieldError('password', 'Das Passwort stimmt nicht.')
      } else if (error instanceof ApiError && error.status === 409) {
        setSession({ setupRequired: true, authenticated: false })
        void navigate('/admin/setup', { replace: true })
      }
    },
  })

  const error = mutation.error
  const throttled = error instanceof ApiError && error.status === 429
  const other = error instanceof ApiError && ![401, 409, 429].includes(error.status)

  return (
    <PublicPage title="Anmelden">
      <form onSubmit={form.onSubmit((values) => mutation.mutate(values.password))} noValidate>
        <Stack>
          {throttled && (
            <Alert color="yellow" variant="light">
              Zu viele Fehlversuche. Versuch es {waitText(error.retryAfterSeconds ?? 60)} noch einmal.
            </Alert>
          )}
          {(other || (error && !(error instanceof ApiError))) && (
            <Alert color="red" variant="light">
              Die Anmeldung ist fehlgeschlagen: {error.message}
            </Alert>
          )}
          <PasswordInput
            label="Passwort"
            autoComplete="current-password"
            data-autofocus
            {...form.getInputProps('password')}
          />
          <Button type="submit" loading={mutation.isPending}>
            Anmelden
          </Button>
        </Stack>
      </form>
    </PublicPage>
  )
}
