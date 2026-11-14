import { Button, Card, Group, MultiSelect, SimpleGrid, Select, Stack, Text, TextInput } from '@mantine/core'
import { IconPlus } from '@tabler/icons-react'
import { IconPicker } from '../../../admin/components/IconPicker.tsx'
import type { CalendarRule, CalendarSettingsUpdate } from './api.ts'
import { ItemControls } from './Controls.tsx'
import { move, newRule, removeAt, replaceAt } from './draft.ts'

type Props = {
  draft: CalendarSettingsUpdate
  onChange: (rules: CalendarRule[]) => void
  errors: Record<string, string>
}

const MATCH_MODES = [
  { value: 'contains', label: 'Titel enthält' },
  { value: 'startsWith', label: 'Titel beginnt mit' },
  { value: 'equals', label: 'Titel ist gleich' },
]

const ACTIONS = [
  { value: 'hide', label: 'Ausblenden' },
  { value: 'icon', label: 'Symbol setzen' },
  { value: 'rename', label: 'Titel umbenennen' },
]

export function RulesTab({ draft, onChange, errors }: Props) {
  const rules = draft.rules
  const calendarOptions = draft.calendars.map((c) => ({ value: c.id, label: c.name || 'Ohne Namen' }))

  return (
    <Stack gap="md">
      <Text size="sm" c="dimmed">
        Regeln gelten von oben nach unten. „Ausblenden“ beendet die Prüfung, bei Symbol und Titel gewinnt die erste passende Regel.
      </Text>
      {rules.length === 0 && <Text c="dimmed">Noch keine Regel angelegt.</Text>}
      {rules.map((rule, i) => {
        const path = `rules.${i}`
        const name = `Regel ${i + 1}`
        const set = (patch: Partial<CalendarRule>) => onChange(replaceAt(rules, i, { ...rule, ...patch }))
        return (
          <Card key={rule.id} withBorder padding="md" role="group" aria-label={name}>
            <Stack gap="sm">
              <Group justify="space-between" wrap="nowrap">
                <Text fw={600}>{name}</Text>
                <ItemControls
                  name={name}
                  index={i}
                  count={rules.length}
                  onMove={(d) => onChange(move(rules, i, d))}
                  onRemove={() => onChange(removeAt(rules, i))}
                />
              </Group>
              {errors[path] && (
                <Text size="sm" c="red">
                  {errors[path]}
                </Text>
              )}
              <MultiSelect
                label="Kalender"
                data={calendarOptions}
                value={rule.calendarIds}
                onChange={(calendarIds) => set({ calendarIds })}
                placeholder={rule.calendarIds.length === 0 ? 'Alle Kalender' : undefined}
                error={errors[`${path}.calendarIds`]}
              />
              <SimpleGrid cols={{ base: 1, sm: 2 }}>
                <Select
                  label="Bedingung"
                  data={MATCH_MODES}
                  allowDeselect={false}
                  value={rule.matchMode}
                  onChange={(v) => set({ matchMode: (v ?? 'contains') as CalendarRule['matchMode'] })}
                  error={errors[`${path}.matchMode`]}
                />
                <TextInput
                  label="Text"
                  value={rule.text}
                  onChange={(e) => set({ text: e.currentTarget.value })}
                  error={errors[`${path}.text`]}
                />
                <Select
                  label="Aktion"
                  data={ACTIONS}
                  allowDeselect={false}
                  value={rule.action}
                  onChange={(v) => set({ action: (v ?? 'hide') as CalendarRule['action'] })}
                  error={errors[`${path}.action`]}
                />
                {rule.action === 'icon' && (
                  <IconPicker label="Symbol" value={rule.icon} onChange={(icon) => set({ icon })} error={errors[`${path}.icon`]} />
                )}
                {rule.action === 'rename' && (
                  <TextInput
                    label="Neuer Titel"
                    value={rule.renameTo}
                    onChange={(e) => set({ renameTo: e.currentTarget.value })}
                    error={errors[`${path}.renameTo`]}
                  />
                )}
              </SimpleGrid>
            </Stack>
          </Card>
        )
      })}
      <div>
        <Button variant="light" leftSection={<IconPlus size={16} aria-hidden />} onClick={() => onChange([...rules, newRule()])}>
          Regel hinzufügen
        </Button>
      </div>
    </Stack>
  )
}
