import { useState } from 'react'
import { Alert, Skeleton, Stack, Switch, Text, TextInput, Title, SegmentedControl } from '@mantine/core'
import { useForm } from '@mantine/form'
import { notifications } from '@mantine/notifications'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '../../../shared/api/client.ts'
import {
  displaySettingsQueryKey,
  fetchDisplaySettings,
  saveDisplaySettings,
  type DisplaySettings,
} from '../api/display.ts'
import { DetailHeader } from '../components/DetailHeader.tsx'
import { RotationPicker } from '../components/RotationPicker.tsx'
import { UnsavedChangesBar } from '../components/UnsavedChangesBar.tsx'
import { UnsavedChangesDialog } from '../components/UnsavedChangesDialog.tsx'
import { useUnsavedChangesGuard } from '../hooks/useUnsavedChangesGuard.ts'
import { changedAreas } from './displayForm.ts'

export function DisplayPage() {
  const query = useQuery({ queryKey: displaySettingsQueryKey, queryFn: fetchDisplaySettings })
  return (
    <>
      <DetailHeader title="Anzeige" />
      {query.isPending && <FormSkeleton />}
      {query.isError && (
        <Alert color="red" title="Die Einstellungen konnten nicht geladen werden.">
          {query.error.message}
        </Alert>
      )}
      {query.data && <DisplayForm saved={query.data} />}
    </>
  )
}

function FormSkeleton() {
  return (
    <Stack gap="xl" aria-busy="true" aria-label="Einstellungen werden geladen">
      {[0, 1, 2].map((i) => (
        <Stack key={i} gap="xs">
          <Skeleton height={24} width={160} />
          <Skeleton height={i === 0 ? 96 : 40} />
        </Stack>
      ))}
    </Stack>
  )
}

function DisplayForm({ saved }: { saved: DisplaySettings }) {
  const queryClient = useQueryClient()
  const form = useForm<DisplaySettings>({ initialValues: saved })
  const [specificOutput, setSpecificOutput] = useState(saved.output !== '')
  const [saveError, setSaveError] = useState<string | null>(null)

  const areas = changedAreas(form.values, saved)
  const dirty = areas.length > 0
  const guard = useUnsavedChangesGuard(dirty)

  const mutation = useMutation({
    mutationFn: saveDisplaySettings,
    onSuccess: (stored) => {
      // The answer is the new baseline; no refetch is needed to show it.
      queryClient.setQueryData(displaySettingsQueryKey, stored)
      form.setValues(stored)
      setSpecificOutput(stored.output !== '')
      setSaveError(null)
      notifications.show({ color: 'green', message: 'Gespeichert.' })
    },
    onError: (error) => {
      // The inputs keep their values; only the messages are added.
      const fieldErrors = error instanceof ApiError ? (error.fieldErrors ?? {}) : {}
      const mapped: Record<string, string> = {}
      if (fieldErrors.rotation) mapped.rotation = fieldErrors.rotation
      if (fieldErrors.output) mapped.output = fieldErrors.output
      form.setErrors(mapped)
      if (Object.keys(mapped).length === 0) {
        setSaveError(error instanceof ApiError ? (error.detail ?? error.message) : error.message)
      } else {
        setSaveError(null)
      }
    },
  })

  const discard = () => {
    form.setValues(saved)
    form.clearErrors()
    setSpecificOutput(saved.output !== '')
    setSaveError(null)
  }

  const save = () => {
    const output = specificOutput ? form.values.output.trim() : ''
    if (specificOutput && output === '') {
      form.setFieldError('output', 'Gib den Namen des Ausgangs ein.')
      return
    }
    form.clearErrors()
    mutation.mutate({ rotation: form.values.rotation, output, testPattern: form.values.testPattern })
  }

  return (
    <>
      <Stack gap="xl">
        <Stack gap="xs" component="section" aria-labelledby="display-rotation">
          <Title order={2} size="h3" id="display-rotation">
            Drehung
          </Title>
          <RotationPicker
            value={form.values.rotation}
            onChange={(rotation) => form.setFieldValue('rotation', rotation)}
            error={form.errors.rotation}
          />
          <Text size="sm" c="dimmed">
            Dreht das Bild am Monitor. Ein hochkant montierter Monitor braucht meist 90° oder 270° – welche Richtung passt, zeigt das Testbild.
          </Text>
        </Stack>

        <Stack gap="xs" component="section" aria-labelledby="display-output">
          <Title order={2} size="h3" id="display-output">
            Monitor-Ausgang
          </Title>
          <SegmentedControl
            fullWidth
            aria-labelledby="display-output"
            value={specificOutput ? 'specific' : 'auto'}
            onChange={(value) => {
              const specific = value === 'specific'
              setSpecificOutput(specific)
              form.clearFieldError('output')
              if (!specific) form.setFieldValue('output', '')
            }}
            data={[
              { value: 'auto', label: 'Automatisch' },
              { value: 'specific', label: 'Bestimmter Ausgang' },
            ]}
          />
          {specificOutput && (
            <TextInput
              label="Ausgang"
              placeholder="HDMI-A-1"
              description="Nur nötig, wenn mehrere Monitore angeschlossen sind."
              autoCapitalize="off"
              autoCorrect="off"
              spellCheck={false}
              {...form.getInputProps('output')}
            />
          )}
        </Stack>

        <Stack gap="xs" component="section" aria-labelledby="display-test">
          <Title order={2} size="h3" id="display-test">
            Testbild
          </Title>
          <Switch
            label="Testbild auf dem Spiegel zeigen"
            description="Zeigt Rahmen, Ausrichtung und Auflösung, um die Drehung zu prüfen."
            {...form.getInputProps('testPattern', { type: 'checkbox' })}
          />
        </Stack>
      </Stack>

      {dirty && (
        <UnsavedChangesBar
          areas={areas}
          saving={mutation.isPending}
          error={saveError}
          onDiscard={discard}
          onSave={save}
        />
      )}
      <UnsavedChangesDialog opened={guard.asking} onStay={guard.stay} onLeave={guard.leave} />
    </>
  )
}
