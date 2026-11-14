import { useState } from 'react'
import { Alert, Card, NumberInput, Skeleton, Stack, Switch, Text, TextInput, Title } from '@mantine/core'
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
  dashboardWeatherQueryKey,
  fetchDashboardWeather,
  fetchWeatherSettings,
  saveWeatherSettings,
  weatherSettingsQueryKey,
  type WeatherSettings,
} from './api.ts'
import { weatherLine } from './status.ts'

type FormValues = {
  location: string
  apiKey: string
  clearApiKey: boolean
  showIcons: boolean
  refreshMinutes: number | string
}

const toValues = (s: WeatherSettings): FormValues => ({
  location: s.location,
  apiKey: '',
  clearApiKey: false,
  showIcons: s.showIcons,
  refreshMinutes: s.refreshMinutes,
})

function changedAreas(values: FormValues, saved: WeatherSettings): string[] {
  const areas: string[] = []
  if (values.location.trim() !== saved.location) areas.push('Ort')
  if (values.apiKey !== '' || values.clearApiKey) areas.push('API-Key')
  if (values.showIcons !== saved.showIcons || values.refreshMinutes !== saved.refreshMinutes) areas.push('Darstellung')
  return areas
}

export function WeatherPage() {
  const query = useQuery({ queryKey: weatherSettingsQueryKey, queryFn: fetchWeatherSettings })
  return (
    <>
      <DetailHeader title="Wetter" />
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
      {query.data && <WeatherForm saved={query.data} />}
    </>
  )
}

function StatusCard() {
  const query = useQuery({ queryKey: dashboardWeatherQueryKey, queryFn: fetchDashboardWeather })
  let text = 'Wird geladen …'
  if (query.isError) text = 'Status nicht verfügbar'
  else if (query.data) text = weatherLine(query.data, true)
  return (
    <Card withBorder component="section" aria-labelledby="weather-status">
      <Title order={2} size="h3" id="weather-status" mb="xs">
        Aktueller Stand
      </Title>
      <Text size="sm">{text}</Text>
    </Card>
  )
}

function WeatherForm({ saved }: { saved: WeatherSettings }) {
  const queryClient = useQueryClient()
  const form = useForm<FormValues>({ initialValues: toValues(saved) })
  const [saveError, setSaveError] = useState<string | null>(null)
  const [justSaved, setJustSaved] = useState(false)

  const areas = changedAreas(form.values, saved)
  const dirty = areas.length > 0
  const guard = useUnsavedChangesGuard(dirty)

  const mutation = useMutation({
    mutationFn: saveWeatherSettings,
    onSuccess: (stored) => {
      queryClient.setQueryData(weatherSettingsQueryKey, stored)
      void queryClient.invalidateQueries({ queryKey: dashboardWeatherQueryKey })
      form.setValues(toValues(stored))
      form.clearErrors()
      setSaveError(null)
      setJustSaved(true)
      notifications.show({ color: 'green', message: 'Gespeichert.' })
    },
    onError: (error) => {
      const fieldErrors = error instanceof ApiError ? (error.fieldErrors ?? {}) : {}
      const mapped: Record<string, string> = {}
      for (const key of ['location', 'apiKey', 'refreshMinutes']) if (fieldErrors[key]) mapped[key] = fieldErrors[key]
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

  const save = () => {
    const minutes = form.values.refreshMinutes
    if (typeof minutes !== 'number') {
      form.setFieldError('refreshMinutes', 'Gib eine Zahl ein.')
      return
    }
    form.clearErrors()
    mutation.mutate({
      location: form.values.location.trim(),
      ...(form.values.apiKey !== '' && !form.values.clearApiKey ? { apiKey: form.values.apiKey } : {}),
      ...(form.values.clearApiKey ? { clearApiKey: true } : {}),
      showIcons: form.values.showIcons,
      refreshMinutes: minutes,
    })
  }

  return (
    <>
      <Stack gap="xl">
        <Stack gap="xs" component="section" aria-labelledby="weather-location">
          <Title order={2} size="h3" id="weather-location">
            Ort
          </Title>
          <TextInput
            label="Ort"
            description="Stadt, z. B. „Berlin“ oder „Köln, DE“. Leer lassen schaltet das Wetter aus."
            {...form.getInputProps('location')}
          />
        </Stack>

        <Stack gap="xs" component="section" aria-labelledby="weather-key">
          <Title order={2} size="h3" id="weather-key">
            OpenWeather
          </Title>
          <SecretField
            label="API-Key"
            hasSecret={saved.hasApiKey}
            value={form.values.apiKey}
            onChange={(v) => form.setFieldValue('apiKey', v)}
            cleared={form.values.clearApiKey}
            onClearedChange={(v) => form.setFieldValue('clearApiKey', v)}
            removeLabel="Key entfernen"
            error={form.errors.apiKey as string | undefined}
          />
          <Text size="sm" c="dimmed">
            Nötig ist ein API-Key mit dem Abo „One Call API 3.0“ (1.000 Abrufe am Tag sind kostenlos).
          </Text>
        </Stack>

        <Stack gap="xs" component="section" aria-labelledby="weather-look">
          <Title order={2} size="h3" id="weather-look">
            Darstellung
          </Title>
          <Switch label="Wettersymbole anzeigen" {...form.getInputProps('showIcons', { type: 'checkbox' })} />
          <NumberInput
            label="Aktualisierung"
            suffix=" Minuten"
            min={10}
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
