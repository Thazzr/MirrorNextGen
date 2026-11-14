import { Button, Card, Group, MultiSelect, NumberInput, SegmentedControl, Select, SimpleGrid, Stack, Switch, Text, TextInput } from '@mantine/core'
import { IconPlus } from '@tabler/icons-react'
import { IconPicker } from '../../../admin/components/IconPicker.tsx'
import type { CalendarSection, CalendarSettingsUpdate, CalendarShift } from './api.ts'
import { HighlightEditor } from './HighlightEditor.tsx'
import { ItemControls } from './Controls.tsx'
import { move, newSection, newShift, removeAt, replaceAt } from './draft.ts'

type Props = {
  draft: CalendarSettingsUpdate
  onChange: (sections: CalendarSection[]) => void
  errors: Record<string, string>
}

const MATCH_MODES = [
  { value: 'equals', label: 'ist gleich' },
  { value: 'startsWith', label: 'beginnt mit' },
  { value: 'contains', label: 'enthält' },
]

const HHMM = /^([01]\d|2[0-3]):[0-5]\d$/

/** Client hint only; the server validates the pair as well. */
function timeHint(shift: CalendarShift): { start?: string; end?: string } {
  const hint: { start?: string; end?: string } = {}
  if ((shift.start === '') !== (shift.end === '')) {
    const message = 'Beginn und Ende gehören zusammen.'
    if (shift.start === '') hint.start = message
    else hint.end = message
  }
  if (shift.start !== '' && !HHMM.test(shift.start)) hint.start = 'Bitte als HH:mm angeben.'
  if (shift.end !== '' && !HHMM.test(shift.end)) hint.end = 'Bitte als HH:mm angeben.'
  return hint
}

type ShiftProps = {
  shift: CalendarShift
  index: number
  path: string
  errors: Record<string, string>
  onChange: (shift: CalendarShift) => void
  onRemove: () => void
}

function ShiftRow({ shift, index, path, errors, onChange, onRemove }: ShiftProps) {
  const hint = timeHint(shift)
  const set = (patch: Partial<CalendarShift>) => onChange({ ...shift, ...patch })
  const n = index + 1
  return (
    <Card withBorder padding="sm" role="group" aria-label={`Schicht ${n}`}>
      <Stack gap="sm">
        <SimpleGrid cols={{ base: 1, sm: 2 }}>
          <TextInput
            label="Kürzel/Stichwort"
            value={shift.match}
            onChange={(e) => set({ match: e.currentTarget.value })}
            error={errors[`${path}.match`]}
          />
          <Select
            label="Vergleich"
            data={MATCH_MODES}
            allowDeselect={false}
            value={shift.matchMode}
            onChange={(v) => set({ matchMode: (v ?? 'equals') as CalendarShift['matchMode'] })}
            error={errors[`${path}.matchMode`]}
          />
          <TextInput
            label="Name"
            value={shift.label}
            onChange={(e) => set({ label: e.currentTarget.value })}
            error={errors[`${path}.label`]}
          />
          <IconPicker label="Symbol" value={shift.icon} onChange={(icon) => set({ icon })} error={errors[`${path}.icon`]} />
          <TextInput
            label="Beginn"
            type="time"
            value={shift.start}
            onChange={(e) => set({ start: e.currentTarget.value })}
            error={errors[`${path}.start`] ?? hint.start}
          />
          <TextInput
            label="Ende"
            type="time"
            value={shift.end}
            onChange={(e) => set({ end: e.currentTarget.value })}
            error={errors[`${path}.end`] ?? hint.end}
          />
        </SimpleGrid>
        <div>
          <Button variant="subtle" color="red" size="compact-sm" onClick={onRemove}>
            Schicht {n} entfernen
          </Button>
        </div>
      </Stack>
    </Card>
  )
}

export function SectionsTab({ draft, onChange, errors }: Props) {
  const calendarOptions = draft.calendars.map((c) => ({ value: c.id, label: c.name || 'Ohne Namen' }))
  const sections = draft.sections

  return (
    <Stack gap="md">
      <Text size="sm" c="dimmed">
        Neue Bereiche erscheinen oben links am Spiegel; verschieben kannst du sie unter „Anordnung“.
      </Text>
      {sections.length === 0 && <Text c="dimmed">Noch kein Bereich angelegt.</Text>}
      {sections.map((section, i) => {
        const path = `sections.${i}`
        const set = (patch: Partial<CalendarSection>) => onChange(replaceAt(sections, i, { ...section, ...patch }))
        const name = `Bereich ${i + 1}`
        const num = (v: number | string) => (typeof v === 'number' ? v : 0)
        return (
          <Card key={section.id} withBorder padding="md" role="group" aria-label={name}>
            <Stack gap="sm">
              <Group justify="space-between" wrap="nowrap">
                <Text fw={600}>{section.title || 'Neuer Bereich'}</Text>
                <ItemControls
                  name={name}
                  index={i}
                  count={sections.length}
                  onMove={(d) => onChange(move(sections, i, d))}
                  onRemove={() => onChange(removeAt(sections, i))}
                />
              </Group>
              {errors[path] && (
                <Text size="sm" c="red">
                  {errors[path]}
                </Text>
              )}
              <TextInput
                label="Titel"
                value={section.title}
                onChange={(e) => set({ title: e.currentTarget.value })}
                error={errors[`${path}.title`]}
              />
              <Stack gap={4}>
                <Text size="sm" fw={500}>
                  Art
                </Text>
                <SegmentedControl
                  fullWidth
                  aria-label={`Art von ${name}`}
                  value={section.type}
                  onChange={(v) => set({ type: v as CalendarSection['type'] })}
                  data={[
                    { value: 'list', label: 'Termine' },
                    { value: 'shifts', label: 'Schichten' },
                  ]}
                />
                {errors[`${path}.type`] && (
                  <Text size="sm" c="red">
                    {errors[`${path}.type`]}
                  </Text>
                )}
              </Stack>
              <MultiSelect
                label="Kalender"
                data={calendarOptions}
                value={section.calendarIds}
                onChange={(calendarIds) => set({ calendarIds })}
                placeholder={section.calendarIds.length === 0 ? 'Kalender wählen' : undefined}
                error={errors[`${path}.calendarIds`]}
              />
              {section.type === 'list' ? (
                <>
                  <SimpleGrid cols={{ base: 1, sm: 2 }}>
                    <NumberInput
                      label="Höchstens N Termine"
                      min={1}
                      max={30}
                      allowDecimal={false}
                      value={section.maxEvents}
                      onChange={(v) => set({ maxEvents: num(v) })}
                      error={errors[`${path}.maxEvents`]}
                    />
                    <NumberInput
                      label="Vorschau-Tage"
                      min={1}
                      max={60}
                      allowDecimal={false}
                      value={section.lookAheadDays}
                      onChange={(v) => set({ lookAheadDays: num(v) })}
                      error={errors[`${path}.lookAheadDays`]}
                    />
                  </SimpleGrid>
                  <HighlightEditor
                    value={section.highlight}
                    path={`${path}.highlight`}
                    errors={errors}
                    onChange={(highlight) => set({ highlight })}
                  />
                </>
              ) : (
                <Stack gap="sm">
                  <SimpleGrid cols={{ base: 1, sm: 2 }}>
                    <NumberInput
                      label="Anzahl Einträge"
                      description="So viele Arbeitstage zeigt der Spiegel – mit freien Tagen: so viele Tage ab heute."
                      min={1}
                      max={14}
                      allowDecimal={false}
                      value={section.shiftDays}
                      onChange={(v) => set({ shiftDays: num(v) })}
                      error={errors[`${path}.shiftDays`]}
                    />
                    <NumberInput
                      label="Vorschau-Tage"
                      description="So weit voraus sucht der Spiegel nach Arbeitstagen."
                      min={1}
                      max={60}
                      allowDecimal={false}
                      value={section.lookAheadDays}
                      onChange={(v) => set({ lookAheadDays: num(v) })}
                      error={errors[`${path}.lookAheadDays`]}
                    />
                  </SimpleGrid>
                  <Switch
                    label="Freie Tage anzeigen"
                    description="Zeigt auch Tage ohne Schicht als „frei“. Aus: nur die nächsten Arbeitstage."
                    checked={section.showFreeDays}
                    onChange={(e) => set({ showFreeDays: e.currentTarget.checked })}
                  />
                  <HighlightEditor
                    value={section.highlight}
                    path={`${path}.highlight`}
                    errors={errors}
                    onChange={(highlight) => set({ highlight })}
                  />
                  <Text size="sm" fw={500}>
                    Schichten
                  </Text>
                  {errors[`${path}.shifts`] && (
                    <Text size="sm" c="red">
                      {errors[`${path}.shifts`]}
                    </Text>
                  )}
                  {section.shifts.map((shift, s) => (
                    <ShiftRow
                      key={shift.id}
                      shift={shift}
                      index={s}
                      path={`${path}.shifts.${s}`}
                      errors={errors}
                      onChange={(next) => set({ shifts: replaceAt(section.shifts, s, next) })}
                      onRemove={() => set({ shifts: removeAt(section.shifts, s) })}
                    />
                  ))}
                  <div>
                    <Button
                      variant="light"
                      size="compact-md"
                      leftSection={<IconPlus size={16} aria-hidden />}
                      onClick={() => set({ shifts: [...section.shifts, newShift()] })}
                    >
                      Schicht hinzufügen
                    </Button>
                  </div>
                </Stack>
              )}
            </Stack>
          </Card>
        )
      })}
      <div>
        <Button variant="light" leftSection={<IconPlus size={16} aria-hidden />} onClick={() => onChange([...sections, newSection()])}>
          Bereich hinzufügen
        </Button>
      </div>
    </Stack>
  )
}
