import { useState } from 'react'
import { ActionIcon, Button, Card, Group, Stack, Text, TextInput } from '@mantine/core'
import { IconPlus, IconTrash } from '@tabler/icons-react'
import { IconPicker } from '../../../admin/components/IconPicker.tsx'
import type { CalendarSettings, CalendarSettingsUpdate, CalendarSourceUpdate } from './api.ts'
import { newCalendar, removeAt, replaceAt } from './draft.ts'

type Props = {
  draft: CalendarSettingsUpdate
  saved: CalendarSettings
  onChange: (calendars: CalendarSourceUpdate[]) => void
  errors: Record<string, string>
}

const HELP =
  'Den privaten iCal-Link findest du z. B. in Google Kalender unter Einstellungen → Kalender → „Privatadresse im iCal-Format“.'

export function CalendarsTab({ draft, saved, onChange, errors }: Props) {
  // Stored calendars show their address hint until the user asks to change it.
  const [changing, setChanging] = useState<Set<string>>(new Set())
  const storedById = new Map(saved.calendars.map((c) => [c.id, c]))

  return (
    <Stack gap="md">
      {draft.calendars.length === 0 && <Text c="dimmed">Noch kein Kalender angelegt.</Text>}
      {draft.calendars.map((calendar, i) => {
        const stored = storedById.get(calendar.id)
        const showHint = stored?.hasUrl === true && !changing.has(calendar.id)
        const path = `calendars.${i}`
        const update = (patch: Partial<CalendarSourceUpdate>) => onChange(replaceAt(draft.calendars, i, { ...calendar, ...patch }))
        return (
          <Card key={calendar.id} withBorder padding="md" role="group" aria-label={`Kalender ${i + 1}`}>
            <Stack gap="sm">
              <Group justify="space-between" wrap="nowrap" align="flex-start">
                <Text fw={600}>{calendar.name || 'Neuer Kalender'}</Text>
                <ActionIcon
                  variant="subtle"
                  color="red"
                  aria-label={`Kalender „${calendar.name || i + 1}“ entfernen`}
                  onClick={() => onChange(removeAt(draft.calendars, i))}
                >
                  <IconTrash size={18} aria-hidden />
                </ActionIcon>
              </Group>
              {errors[path] && (
                <Text size="sm" c="red">
                  {errors[path]}
                </Text>
              )}
              <TextInput
                label="Name"
                value={calendar.name}
                onChange={(e) => update({ name: e.currentTarget.value })}
                error={errors[`${path}.name`]}
              />
              <IconPicker
                label="Symbol"
                value={calendar.icon}
                onChange={(icon) => update({ icon })}
                error={errors[`${path}.icon`]}
              />
              {showHint ? (
                <Stack gap={4}>
                  <Text size="sm" fw={500}>
                    Adresse
                  </Text>
                  <Text size="sm" c="dimmed" style={{ overflowWrap: 'anywhere' }}>
                    {stored.urlHint}
                  </Text>
                  <div>
                    <Button variant="subtle" size="compact-sm" onClick={() => setChanging(new Set(changing).add(calendar.id))}>
                      Adresse ändern
                    </Button>
                  </div>
                </Stack>
              ) : (
                <TextInput
                  label="Adresse (iCal-Link)"
                  placeholder="https://… oder webcal://…"
                  description={stored?.hasUrl ? 'Leer lassen, um die gespeicherte Adresse zu behalten.' : HELP}
                  required={!stored?.hasUrl}
                  autoCapitalize="off"
                  autoCorrect="off"
                  spellCheck={false}
                  value={calendar.url ?? ''}
                  onChange={(e) => update({ url: e.currentTarget.value })}
                  error={errors[`${path}.url`]}
                />
              )}
              {showHint && errors[`${path}.url`] && (
                <Text size="sm" c="red">
                  {errors[`${path}.url`]}
                </Text>
              )}
            </Stack>
          </Card>
        )
      })}
      <div>
        <Button variant="light" leftSection={<IconPlus size={16} aria-hidden />} onClick={() => onChange([...draft.calendars, newCalendar()])}>
          Kalender hinzufügen
        </Button>
      </div>
    </Stack>
  )
}
