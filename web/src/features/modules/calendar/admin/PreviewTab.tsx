import { useEffect, useRef } from 'react'
import { Alert, Badge, Button, Card, Group, Skeleton, Stack, Text } from '@mantine/core'
import { useMutation } from '@tanstack/react-query'
import { ApiError } from '../../../../shared/api/client.ts'
import { previewCalendar, type CalendarSettingsUpdate } from './api.ts'
import { toBody } from './draft.ts'

type Props = {
  draft: CalendarSettingsUpdate
  /** True while the tab is shown; opening it refreshes the preview. */
  active: boolean
}

export function PreviewTab({ draft, active }: Props) {
  const mutation = useMutation({ mutationFn: previewCalendar })
  const draftRef = useRef(draft)
  const { mutate } = mutation

  // Declared before the opening effect, so the latest draft is in the ref when it runs.
  useEffect(() => {
    draftRef.current = draft
  })
  useEffect(() => {
    if (active) mutate(toBody(draftRef.current))
  }, [active, mutate])

  const ruleNumber = (id: string) => draft.rules.findIndex((r) => r.id === id) + 1
  const data = mutation.data

  return (
    <Stack gap="md">
      <Group justify="space-between" align="center">
        <Text size="sm" c="dimmed" style={{ flex: '1 1 220px' }}>
          Zeigt mit echten Terminen, was am Spiegel erscheinen würde. Du musst dafür nicht speichern.
        </Text>
        <Button variant="light" loading={mutation.isPending} onClick={() => mutate(toBody(draft))}>
          Vorschau aktualisieren
        </Button>
      </Group>

      {mutation.isError && (
        <Alert color="red" title="Die Vorschau konnte nicht erstellt werden.">
          {mutation.error instanceof ApiError
            ? [mutation.error.detail ?? mutation.error.message, ...Object.values(mutation.error.fieldErrors ?? {})].join(' ')
            : mutation.error.message}
        </Alert>
      )}
      {mutation.isPending && !data && <Skeleton height={120} aria-label="Vorschau wird geladen" />}

      {data?.calendarErrors.map((e) => {
        const name = draft.calendars.find((c) => c.id === e.calendarId)?.name || e.calendarId
        return (
          <Alert key={e.calendarId} color="red" title={`Kalender „${name}“`}>
            {e.message}
          </Alert>
        )
      })}

      {data?.sections.map((section) => (
        <Card key={section.id} withBorder padding="md" role="group" aria-label={`Vorschau ${section.title}`}>
          <Stack gap="xs">
            <Text component="h2" fw={600} fz="md" m={0}>
              {section.title}
            </Text>
            {section.error && <Alert color="red">{section.error}</Alert>}
            {section.type === 'shifts' ? (
              <>
                {section.days?.length === 0 && !section.error && <Text size="sm" c="dimmed">Keine Schichten im Zeitraum.</Text>}
                {section.days?.map((day) => (
                  <Group key={day.date} gap="xs">
                    <Text size="sm" fw={500} w={90} style={{ color: day.highlightColor }}>
                      {day.dayLabel}
                    </Text>
                    <Text size="sm" c={day.free ? 'dimmed' : undefined} fw={day.ongoing ? 700 : undefined}>
                      {day.title}
                      {day.timeRange && (
                        <Text span inherit style={{ color: day.highlightColor }}>
                          {` ${day.timeRange}`}
                        </Text>
                      )}
                    </Text>
                  </Group>
                ))}
              </>
            ) : (
              <>
                {(section.items ?? []).length === 0 && !section.error && <Text size="sm" c="dimmed">Keine Termine im Zeitraum.</Text>}
                {section.items?.map((item, i) => {
                  const overLimit = !item.shown && !item.hidden
                  const renamed = item.title !== item.originalTitle
                  return (
                    <Stack key={`${item.calendarId}-${item.start}-${i}`} gap={2} data-testid="preview-item" pt="xs" style={{ borderTop: '1px solid var(--mantine-color-dark-4)' }}>
                      <Text size="sm" td={item.hidden ? 'line-through' : undefined} c={item.hidden || overLimit ? 'dimmed' : undefined}>
                        {renamed ? `${item.title} (vorher: ${item.originalTitle})` : item.title}
                      </Text>
                      <Group gap={6} align="center" wrap="wrap">
                        {(item.relative || item.timeLabel) && (
                          <Text size="xs" c={item.shown && item.highlightColor ? undefined : 'dimmed'} style={item.shown && item.highlightColor ? { color: item.highlightColor } : undefined}>
                            {item.relative || item.timeLabel}
                          </Text>
                        )}
                        {item.hidden && (
                          <Badge color="gray" variant="light">
                            ausgeblendet
                          </Badge>
                        )}
                        {overLimit && (
                          <Badge color="gray" variant="light">
                            über dem Limit
                          </Badge>
                        )}
                        {item.matchedRules.map((id) => (
                          <Badge key={id} size="sm" variant="outline">
                            Regel {ruleNumber(id)}
                          </Badge>
                        ))}
                      </Group>
                    </Stack>
                  )
                })}
              </>
            )}
          </Stack>
        </Card>
      ))}
    </Stack>
  )
}
