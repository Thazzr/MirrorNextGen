import { useState } from 'react'
import { Alert, Button, Card, Group, NumberInput, Select, Skeleton, Stack, Text, TextInput, Title } from '@mantine/core'
import { useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '../../../../shared/api/client.ts'
import { DetailHeader } from '../../../admin/components/DetailHeader.tsx'
import { SecretField } from '../../../admin/components/SecretField.tsx'
import { UnsavedChangesBar } from '../../../admin/components/UnsavedChangesBar.tsx'
import { UnsavedChangesDialog } from '../../../admin/components/UnsavedChangesDialog.tsx'
import { useUnsavedChangesGuard } from '../../../admin/hooks/useUnsavedChangesGuard.ts'
import {
  bringSettingsQueryKey,
  dashboardBringQueryKey,
  fetchBringLists,
  fetchBringSettings,
  fetchDashboardBring,
  saveBringSettings,
  type BringSettings,
} from '../api.ts'
import { bringLine } from './status.ts'

type FormValues = {
  email: string
  password: string
  clearPassword: boolean
  listUuid: string
  maxItems: number | string
  refreshMinutes: number | string
}

const toValues = (s: BringSettings): FormValues => ({
  email: s.email,
  password: '',
  clearPassword: false,
  listUuid: s.listUuid,
  maxItems: s.maxItems,
  refreshMinutes: s.refreshMinutes,
})

function changedAreas(values: FormValues, saved: BringSettings): string[] {
  const areas: string[] = []
  if (values.email.trim() !== saved.email || values.password !== '' || values.clearPassword) areas.push('Zugang')
  if (values.listUuid !== saved.listUuid) areas.push('Liste')
  if (values.maxItems !== saved.maxItems || values.refreshMinutes !== saved.refreshMinutes) areas.push('Darstellung')
  return areas
}

export function BringPage() {
  const query = useQuery({ queryKey: bringSettingsQueryKey, queryFn: fetchBringSettings })
  return (
    <>
      <DetailHeader title="Bring!-Einkaufsliste" />
      {query.isPending && (
        <Stack gap="xl" aria-busy="true" aria-label="Einstellungen werden geladen">
          {[0, 1, 2].map((i) => (
            <Stack key={i} gap="xs">
              <Skeleton height={24} width={160} />
              <Skeleton height={40} />
            </Stack>
          ))}
        </Stack>
      )}
      {query.isError && (
        <Alert color="red" title="Die Einstellungen konnten nicht geladen werden.">
          {query.error.message}
        </Alert>
      )}
      {query.data && <BringForm saved={query.data} />}
    </>
  )
}

function StatusCard() {
  const query = useQuery({ queryKey: dashboardBringQueryKey, queryFn: fetchDashboardBring })
  let text = 'Wird geladen …'
  if (query.isError) text = 'Status nicht verfügbar'
  else if (query.data && !query.isFetching) text = bringLine(query.data)
  return (
    <Card withBorder component="section" aria-labelledby="bring-status">
      <Title order={2} size="h3" id="bring-status" mb="xs">
        Aktueller Stand
      </Title>
      <Text size="sm">{text}</Text>
    </Card>
  )
}

const errorText = (error: Error) => (error instanceof ApiError ? (error.detail ?? error.message) : error.message)

function BringForm({ saved }: { saved: BringSettings }) {
  const queryClient = useQueryClient()
  const form = useForm<FormValues>({ initialValues: toValues(saved) })
  const [saveError, setSaveError] = useState<string | null>(null)
  const [justSaved, setJustSaved] = useState(false)

  const areas = changedAreas(form.values, saved)
  const dirty = areas.length > 0
  const guard = useUnsavedChangesGuard(dirty)

  // Loading the picker's lists is an action on the stored login, not a page load: it happens on a click.
  const lists = useMutation({ mutationFn: fetchBringLists })

  const mutation = useMutation({
    mutationFn: saveBringSettings,
    onSuccess: (stored) => {
      // A different account has other lists; the loaded ones no longer apply.
      if (stored.email !== saved.email || stored.hasPassword !== saved.hasPassword || form.values.password !== '') lists.reset()
      queryClient.setQueryData(bringSettingsQueryKey, stored)
      void queryClient.invalidateQueries({ queryKey: dashboardBringQueryKey })
      form.setValues(toValues(stored))
      form.clearErrors()
      setSaveError(null)
      setJustSaved(true)
      notifications.show({ color: 'green', message: 'Gespeichert.' })
    },
    onError: (error) => {
      const fieldErrors = error instanceof ApiError ? (error.fieldErrors ?? {}) : {}
      const mapped: Record<string, string> = {}
      for (const key of ['email', 'password', 'listUuid', 'maxItems', 'refreshMinutes']) if (fieldErrors[key]) mapped[key] = fieldErrors[key]
      form.setErrors(mapped)
      setSaveError(Object.keys(mapped).length === 0 ? errorText(error) : null)
    },
  })

  const discard = () => {
    form.setValues(toValues(saved))
    form.clearErrors()
    setSaveError(null)
  }

  const save = () => {
    const { maxItems, refreshMinutes } = form.values
    const errors: Record<string, string> = {}
    if (typeof maxItems !== 'number') errors.maxItems = 'Gib eine Zahl ein.'
    if (typeof refreshMinutes !== 'number') errors.refreshMinutes = 'Gib eine Zahl ein.'
    if (typeof maxItems !== 'number' || typeof refreshMinutes !== 'number') {
      form.setErrors(errors)
      return
    }
    form.clearErrors()
    mutation.mutate({
      email: form.values.email.trim(),
      ...(form.values.password !== '' && !form.values.clearPassword ? { password: form.values.password } : {}),
      ...(form.values.clearPassword ? { clearPassword: true } : {}),
      listUuid: form.values.listUuid,
      maxItems,
      refreshMinutes,
    })
  }

  // "Standardliste" first; a stored list that is not among the loaded ones stays selectable under a neutral name.
  const options = [{ value: '', label: 'Standardliste' }, ...(lists.data ?? []).map((l) => ({ value: l.uuid, label: l.name }))]
  if (form.values.listUuid !== '' && !options.some((o) => o.value === form.values.listUuid)) {
    options.push({ value: form.values.listUuid, label: 'Gewählte Liste' })
  }

  return (
    <>
      <Stack gap="xl">
        <Text size="sm" c="dimmed">
          Zeigt die offenen Einträge deiner Bring!-Einkaufsliste am Spiegel.
        </Text>

        <Stack gap="xs" component="section" aria-labelledby="bring-account">
          <Title order={2} size="h3" id="bring-account">
            Konto
          </Title>
          <TextInput
            label="E-Mail"
            description="Leer lassen schaltet die Einkaufsliste aus."
            type="email"
            autoComplete="off"
            autoCapitalize="off"
            spellCheck={false}
            {...form.getInputProps('email')}
          />
          <SecretField
            label="Passwort"
            hasSecret={saved.hasPassword}
            value={form.values.password}
            onChange={(v) => form.setFieldValue('password', v)}
            cleared={form.values.clearPassword}
            onClearedChange={(v) => form.setFieldValue('clearPassword', v)}
            removeLabel="Passwort entfernen"
            error={form.errors.password as string | undefined}
          />
        </Stack>

        <Stack gap="xs" component="section" aria-labelledby="bring-list">
          <Title order={2} size="h3" id="bring-list">
            Liste wählen
          </Title>
          <Select
            label="Liste"
            description="Die Standardliste ist die Liste, die Bring! in der App zuerst zeigt."
            data={options}
            allowDeselect={false}
            value={form.values.listUuid}
            onChange={(v) => form.setFieldValue('listUuid', v ?? '')}
            error={form.errors.listUuid as string | undefined}
          />
          {!lists.data && (
            <Group gap="sm">
              <Button variant="default" size="compact-sm" loading={lists.isPending} onClick={() => lists.mutate()}>
                Listen laden
              </Button>
              <Text size="sm" c="dimmed">
                Nutzt die gespeicherte E-Mail und das gespeicherte Passwort.
              </Text>
            </Group>
          )}
          {lists.isError && (
            <Text size="sm" c="red" role="alert">
              {errorText(lists.error)}
            </Text>
          )}
        </Stack>

        <Stack gap="xs" component="section" aria-labelledby="bring-look">
          <Title order={2} size="h3" id="bring-look">
            Darstellung
          </Title>
          <NumberInput
            label="Anzahl Einträge"
            description="Der Spiegel zeigt höchstens 18 Einträge in drei Reihen."
            prefix="Höchstens "
            suffix=" Einträge"
            min={1}
            max={18}
            allowDecimal={false}
            clampBehavior="none"
            {...form.getInputProps('maxItems')}
          />
          <NumberInput
            label="Aktualisierung (Minuten)"
            suffix=" Minuten"
            min={1}
            max={1440}
            allowDecimal={false}
            clampBehavior="none"
            {...form.getInputProps('refreshMinutes')}
          />
        </Stack>

        {justSaved && <StatusCard />}
      </Stack>

      {dirty && (
        <UnsavedChangesBar areas={areas} saving={mutation.isPending} error={saveError} onDiscard={discard} onSave={save} />
      )}
      <UnsavedChangesDialog opened={guard.asking} onStay={guard.stay} onLeave={guard.leave} />
    </>
  )
}
