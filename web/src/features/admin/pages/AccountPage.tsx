import { Button, PasswordInput, Stack, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useForm } from '@mantine/form'
import { useMutation } from '@tanstack/react-query'
import { changePassword } from '../api/auth.ts'
import { DetailHeader } from '../components/DetailHeader.tsx'
import { ApiError } from '../../../shared/api/client.ts'
import { passwordLengthError } from './passwordRules.ts'

const empty = { currentPassword: '', newPassword: '', repeat: '' }

// A credential flow: it is submitted as a whole, so there is no unsaved-changes bar here.
export function AccountPage() {
  const form = useForm({
    initialValues: empty,
    validate: {
      currentPassword: (value) => (value ? null : 'Gib dein aktuelles Passwort ein.'),
      newPassword: passwordLengthError,
      repeat: (value, values) => (value === values.newPassword ? null : 'Die Passwörter stimmen nicht überein.'),
    },
  })
  const mutation = useMutation({
    mutationFn: (values: typeof empty) => changePassword(values.currentPassword, values.newPassword),
    onSuccess: () => {
      form.setValues(empty)
      form.resetDirty(empty)
      notifications.show({ color: 'green', message: 'Passwort geändert. Andere Geräte wurden abgemeldet.' })
    },
    onError: (error) => {
      if (!(error instanceof ApiError)) return
      if (error.status === 401) {
        form.setFieldError('currentPassword', 'Das aktuelle Passwort stimmt nicht.')
      } else if (error.fieldErrors?.newPassword) {
        form.setFieldError('newPassword', error.fieldErrors.newPassword)
      } else {
        form.setFieldError('newPassword', error.detail ?? error.message)
      }
    },
  })

  return (
    <>
      <DetailHeader title="Konto" />
      <Title order={2} size="h3" mb="md">
        Passwort ändern
      </Title>
      <form onSubmit={form.onSubmit((values) => mutation.mutate(values))} noValidate>
        <Stack>
          <PasswordInput
            label="Aktuelles Passwort"
            autoComplete="current-password"
            {...form.getInputProps('currentPassword')}
          />
          <PasswordInput
            label="Neues Passwort"
            description="Mindestens 10 Zeichen"
            autoComplete="new-password"
            {...form.getInputProps('newPassword')}
          />
          <PasswordInput
            label="Neues Passwort wiederholen"
            autoComplete="new-password"
            {...form.getInputProps('repeat')}
          />
          <Button type="submit" loading={mutation.isPending}>
            Passwort ändern
          </Button>
        </Stack>
      </form>
    </>
  )
}
