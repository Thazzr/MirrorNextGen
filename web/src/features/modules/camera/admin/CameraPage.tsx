import { useState } from 'react'
import { Alert, Box, Input, Skeleton, Slider, Stack, Switch, Text, TextInput, Title } from '@mantine/core'
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
  cameraFilter,
  cameraSettingsQueryKey,
  dashboardCameraQueryKey,
  fetchCameraSettings,
  saveCameraSettings,
  type CameraSettings,
} from '../api.ts'

type FormValues = {
  enabled: boolean
  title: string
  streamUrl: string
  clearStreamUrl: boolean
  brightness: number
  contrast: number
  saturation: number
  grayscale: boolean
}

const toValues = (s: CameraSettings): FormValues => ({
  enabled: s.enabled,
  title: s.title,
  streamUrl: '',
  clearStreamUrl: false,
  brightness: s.brightness,
  contrast: s.contrast,
  saturation: s.saturation,
  grayscale: s.grayscale,
})

function changedAreas(values: FormValues, saved: CameraSettings): string[] {
  const areas: string[] = []
  if (values.enabled !== saved.enabled || values.title.trim() !== saved.title) areas.push('Anzeige')
  if (values.streamUrl !== '' || values.clearStreamUrl) areas.push('Stream-Adresse')
  if (
    values.brightness !== saved.brightness ||
    values.contrast !== saved.contrast ||
    values.saturation !== saved.saturation ||
    values.grayscale !== saved.grayscale
  )
    areas.push('Bild')
  return areas
}

const errorText = (error: Error) => (error instanceof ApiError ? (error.detail ?? error.message) : error.message)

export function CameraPage() {
  const query = useQuery({ queryKey: cameraSettingsQueryKey, queryFn: fetchCameraSettings })
  return (
    <>
      <DetailHeader title="Kamera" />
      {query.isPending && (
        <Stack gap="md" aria-busy="true" aria-label="Einstellungen werden geladen">
          <Skeleton height={28} width={200} />
          <Skeleton height={40} />
          <Skeleton height={40} />
          <Skeleton height={120} />
        </Stack>
      )}
      {query.isError && (
        <Alert color="red" title="Die Einstellungen konnten nicht geladen werden.">
          {query.error.message}
        </Alert>
      )}
      {query.data && <CameraForm saved={query.data} />}
    </>
  )
}

function PercentSlider({
  label,
  value,
  onChange,
  error,
}: {
  label: string
  value: number
  onChange: (value: number) => void
  error?: string
}) {
  return (
    <Input.Wrapper label={`${label}: ${value} %`} error={error}>
      <Box px="xs" pt={6} pb="md">
        <Slider
          min={0}
          max={200}
          step={1}
          value={value}
          onChange={onChange}
          label={(v) => `${v} %`}
          thumbLabel={label}
          marks={[{ value: 0 }, { value: 100, label: '100 %' }, { value: 200 }]}
        />
      </Box>
    </Input.Wrapper>
  )
}

function CameraForm({ saved }: { saved: CameraSettings }) {
  const queryClient = useQueryClient()
  const form = useForm<FormValues>({ initialValues: toValues(saved) })
  const [saveError, setSaveError] = useState<string | null>(null)

  const areas = changedAreas(form.values, saved)
  const dirty = areas.length > 0
  const guard = useUnsavedChangesGuard(dirty)

  const mutation = useMutation({
    mutationFn: saveCameraSettings,
    onSuccess: (stored) => {
      queryClient.setQueryData(cameraSettingsQueryKey, stored)
      void queryClient.invalidateQueries({ queryKey: dashboardCameraQueryKey })
      form.setValues(toValues(stored))
      form.clearErrors()
      setSaveError(null)
      notifications.show({ color: 'green', message: 'Gespeichert.' })
    },
    onError: (error) => {
      const fieldErrors = error instanceof ApiError ? (error.fieldErrors ?? {}) : {}
      const mapped: Record<string, string> = {}
      for (const key of ['title', 'streamUrl', 'brightness', 'contrast', 'saturation']) if (fieldErrors[key]) mapped[key] = fieldErrors[key]
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
    const v = form.values
    form.clearErrors()
    mutation.mutate({
      enabled: v.enabled,
      title: v.title.trim(),
      ...(v.streamUrl.trim() !== '' && !v.clearStreamUrl ? { streamUrl: v.streamUrl.trim() } : {}),
      ...(v.clearStreamUrl ? { clearStreamUrl: true } : {}),
      brightness: v.brightness,
      contrast: v.contrast,
      saturation: v.saturation,
      grayscale: v.grayscale,
    })
  }

  const hint = 'Nur RTSP und RTSPS. Die Adresse bleibt auf dem Spiegel.'
  return (
    <>
      <Stack gap="xl">
        <Text size="sm" c="dimmed">
          Zeigt das Live-Bild einer Kamera am Spiegel. Das Video läuft über den Spiegel, die Kamera selbst ist von außen nicht erreichbar.
        </Text>

        <Stack gap="xs" component="section" aria-labelledby="camera-source">
          <Title order={2} size="h3" id="camera-source">
            Kamera
          </Title>
          <Switch label="Kamera anzeigen" {...form.getInputProps('enabled', { type: 'checkbox' })} />
          <TextInput label="Titel" description="Steht über dem Bild. Leer lassen für kein Titelfeld." {...form.getInputProps('title')} />
          <SecretField
            label="Stream-Adresse"
            description={saved.hasStreamUrl && saved.streamUrlHint ? `Gespeichert für ${saved.streamUrlHint}. ${hint}` : hint}
            placeholder="rtsp://benutzer:passwort@192.168.1.20:554/stream1"
            hasSecret={saved.hasStreamUrl}
            value={form.values.streamUrl}
            onChange={(v) => form.setFieldValue('streamUrl', v)}
            cleared={form.values.clearStreamUrl}
            onClearedChange={(v) => form.setFieldValue('clearStreamUrl', v)}
            removeLabel="Stream-Adresse entfernen"
            error={form.errors.streamUrl as string | undefined}
          />
        </Stack>

        <Stack gap="xs" component="section" aria-labelledby="camera-picture">
          <Title order={2} size="h3" id="camera-picture">
            Bild
          </Title>
          <PercentSlider
            label="Helligkeit"
            value={form.values.brightness}
            onChange={(v) => form.setFieldValue('brightness', v)}
            error={form.errors.brightness as string | undefined}
          />
          <PercentSlider
            label="Kontrast"
            value={form.values.contrast}
            onChange={(v) => form.setFieldValue('contrast', v)}
            error={form.errors.contrast as string | undefined}
          />
          <PercentSlider
            label="Sättigung"
            value={form.values.saturation}
            onChange={(v) => form.setFieldValue('saturation', v)}
            error={form.errors.saturation as string | undefined}
          />
          <Switch label="Schwarzweiß" {...form.getInputProps('grayscale', { type: 'checkbox' })} />
          {/* A fixed gradient instead of the stream: the admin never opens a video connection. */}
          <div
            role="img"
            aria-label="Vorschau der Bildeinstellungen"
            data-testid="camera-preview"
            style={{
              aspectRatio: '16 / 9',
              width: '100%',
              maxWidth: 480,
              borderRadius: 8,
              background: 'linear-gradient(135deg, #e63946 0%, #f6b73c 25%, #2a9d8f 50%, #457b9d 75%, #f1faee 100%)',
              filter: cameraFilter(form.values),
            }}
          />
        </Stack>
      </Stack>

      {dirty && (
        <UnsavedChangesBar areas={areas} saving={mutation.isPending} error={saveError} onDiscard={discard} onSave={save} />
      )}
      <UnsavedChangesDialog opened={guard.asking} onStay={guard.stay} onLeave={guard.leave} />
    </>
  )
}
