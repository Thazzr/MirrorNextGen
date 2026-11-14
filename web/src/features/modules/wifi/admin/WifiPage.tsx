import { useState } from 'react'
import { Alert, Input, SegmentedControl, Skeleton, Stack, Text, TextInput } from '@mantine/core'
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
  dashboardWifiQueryKey,
  fetchWifiSettings,
  saveWifiSettings,
  wifiSettingsQueryKey,
  type WifiSettings,
} from './api.ts'

type FormValues = { ssid: string; security: 'WPA' | 'nopass'; password: string; clearPassword: boolean }

const toValues = (s: WifiSettings): FormValues => ({
  ssid: s.ssid,
  security: s.security,
  password: '',
  clearPassword: false,
})

function changedAreas(values: FormValues, saved: WifiSettings): string[] {
  const areas: string[] = []
  if (values.ssid.trim() !== saved.ssid) areas.push('Netzwerk')
  if (values.security !== saved.security) areas.push('Sicherheit')
  if (values.security === 'WPA' && (values.password !== '' || values.clearPassword)) areas.push('Passwort')
  return areas
}

export function WifiPage() {
  const query = useQuery({ queryKey: wifiSettingsQueryKey, queryFn: fetchWifiSettings })
  return (
    <>
      <DetailHeader title="WLAN-QR-Code" />
      {query.isPending && (
        <Stack gap="md" aria-busy="true" aria-label="Einstellungen werden geladen">
          <Skeleton height={40} />
          <Skeleton height={36} />
          <Skeleton height={40} />
        </Stack>
      )}
      {query.isError && (
        <Alert color="red" title="Die Einstellungen konnten nicht geladen werden.">
          {query.error.message}
        </Alert>
      )}
      {query.data && <WifiForm saved={query.data} />}
    </>
  )
}

function WifiForm({ saved }: { saved: WifiSettings }) {
  const queryClient = useQueryClient()
  const form = useForm<FormValues>({ initialValues: toValues(saved) })
  const [saveError, setSaveError] = useState<string | null>(null)

  const areas = changedAreas(form.values, saved)
  const dirty = areas.length > 0
  const guard = useUnsavedChangesGuard(dirty)

  const mutation = useMutation({
    mutationFn: saveWifiSettings,
    onSuccess: (stored) => {
      queryClient.setQueryData(wifiSettingsQueryKey, stored)
      void queryClient.invalidateQueries({ queryKey: dashboardWifiQueryKey })
      form.setValues(toValues(stored))
      form.clearErrors()
      setSaveError(null)
      notifications.show({ color: 'green', message: 'Gespeichert.' })
    },
    onError: (error) => {
      const fieldErrors = error instanceof ApiError ? (error.fieldErrors ?? {}) : {}
      const mapped: Record<string, string> = {}
      for (const key of ['ssid', 'security', 'password']) if (fieldErrors[key]) mapped[key] = fieldErrors[key]
      form.setErrors(mapped)
      setSaveError(
        Object.keys(mapped).length === 0 ? (error instanceof ApiError ? (error.detail ?? error.message) : error.message) : null,
      )
    },
  })

  const discard = () => {
    form.setValues(toValues(saved))
    form.clearErrors()
    setSaveError(null)
  }

  const open = form.values.security === 'nopass'
  const save = () => {
    form.clearErrors()
    mutation.mutate({
      ssid: form.values.ssid.trim(),
      security: form.values.security,
      ...(!open && form.values.password !== '' && !form.values.clearPassword ? { password: form.values.password } : {}),
      ...(!open && form.values.clearPassword ? { clearPassword: true } : {}),
    })
  }

  return (
    <>
      <Stack gap="lg">
        <Text size="sm" c="dimmed">
          Gäste scannen den QR-Code unten am Spiegel und verbinden sich ohne Tippen.
        </Text>
        <TextInput
          label="Netzwerkname (SSID)"
          description="Leer lassen schaltet den QR-Code aus."
          autoCapitalize="off"
          autoCorrect="off"
          spellCheck={false}
          {...form.getInputProps('ssid')}
        />
        <Input.Wrapper label="Sicherheit" error={form.errors.security as string | undefined}>
          <SegmentedControl
            fullWidth
            mt={4}
            aria-label="Sicherheit"
            value={form.values.security}
            onChange={(v) => form.setFieldValue('security', v as FormValues['security'])}
            data={[
              { value: 'WPA', label: 'WPA/WPA2' },
              { value: 'nopass', label: 'Offen' },
            ]}
          />
        </Input.Wrapper>
        {!open && (
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
        )}
      </Stack>

      {dirty && (
        <UnsavedChangesBar areas={areas} saving={mutation.isPending} error={saveError} onDiscard={discard} onSave={save} />
      )}
      <UnsavedChangesDialog opened={guard.asking} onStay={guard.stay} onLeave={guard.leave} />
    </>
  )
}
