import { Button, PasswordInput, Stack, Text } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useForm } from '@mantine/form'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { setupAdmin } from '../api/auth.ts'
import { useSetSession } from '../api/session.ts'
import { PublicPage } from '../components/PublicPage.tsx'
import { ApiError } from '../../../shared/api/client.ts'
import { passwordLengthError } from './passwordRules.ts'

export function SetupPage() {
  const form = useForm({
    initialValues: { password: '', repeat: '' },
    validate: {
      password: passwordLengthError,
      repeat: (value, values) => (value === values.password ? null : 'Die Passwörter stimmen nicht überein.'),
    },
  })
  const setSession = useSetSession()
  const navigate = useNavigate()
  const mutation = useMutation({
    mutationFn: setupAdmin,
    // The guard around this page moves on to the overview once the session is cached.
    onSuccess: setSession,
    onError: (error) => {
      if (!(error instanceof ApiError)) return
      if (error.status === 409) {
        setSession({ setupRequired: false, authenticated: false })
        notifications.show({ color: 'yellow', message: 'Der Zugang ist bereits eingerichtet.' })
        void navigate('/admin/login', { replace: true })
      } else if (error.fieldErrors?.password) {
        form.setFieldError('password', error.fieldErrors.password)
      } else {
        form.setFieldError('password', error.detail ?? error.message)
      }
    },
  })

  return (
    <PublicPage title="Spiegel einrichten">
      <Text>Lege das Passwort für die Verwaltung fest. Es schützt alle Einstellungen des Spiegels.</Text>
      <form onSubmit={form.onSubmit((values) => mutation.mutate(values.password))} noValidate>
        <Stack>
          <PasswordInput
            label="Passwort"
            description="Mindestens 10 Zeichen"
            autoComplete="new-password"
            data-autofocus
            {...form.getInputProps('password')}
          />
          <PasswordInput label="Passwort wiederholen" autoComplete="new-password" {...form.getInputProps('repeat')} />
          <Button type="submit" loading={mutation.isPending}>
            Zugang anlegen
          </Button>
        </Stack>
      </form>
    </PublicPage>
  )
}
