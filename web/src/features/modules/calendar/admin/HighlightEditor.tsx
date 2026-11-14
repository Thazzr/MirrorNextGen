import { ColorInput, NumberInput, SimpleGrid, Stack, Switch } from '@mantine/core'
import type { CalendarHighlight } from './api.ts'

const SWATCHES = ['#f6b73c', '#ff8a65', '#ff5c5c', '#8fd3ff', '#7ee787', '#ffffff']

type Props = {
  value: CalendarHighlight
  /** Server field path of the highlight, for example `sections.0.highlight`. */
  path: string
  errors: Record<string, string>
  onChange: (next: CalendarHighlight) => void
}

type TierKey = 'ongoing' | 'soon' | 'today'

/** Colour of the time text by how soon an item starts; shared by both section types. */
export function HighlightEditor({ value, path, errors, onChange }: Props) {
  const setTier = (key: TierKey, patch: Partial<CalendarHighlight[TierKey]>) => onChange({ ...value, [key]: { ...value[key], ...patch } })

  const colorInput = (key: TierKey, name: string) => (
    <ColorInput
      label="Farbe"
      aria-label={`Farbe für ${name}`}
      format="hex"
      swatches={SWATCHES}
      swatchesPerRow={6}
      value={value[key].color}
      onChange={(color) => setTier(key, { color })}
      error={errors[`${path}.${key}.color`]}
    />
  )

  // Switch left, colour right; on narrow widths the colour stacks under the switch.
  const row = (key: TierKey, name: string, description: string, extra?: React.ReactNode) => (
    <Stack gap="xs" role="group" aria-label={name}>
      <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
        <Switch
          label={name}
          description={description}
          checked={value[key].enabled}
          onChange={(e) => setTier(key, { enabled: e.currentTarget.checked })}
        />
        {colorInput(key, name)}
      </SimpleGrid>
      {extra}
    </Stack>
  )

  return (
    <Stack gap="sm">
      <Switch
        label="Zeitangabe einfärben"
        description="Färbt rechts die Zeit ein, je nachdem wie bald ein Termin beginnt."
        checked={value.enabled}
        onChange={(e) => onChange({ ...value, enabled: e.currentTarget.checked })}
      />
      {value.enabled && (
        <Stack gap="md" pl={{ base: 0, sm: 'md' }}>
          {row('ongoing', 'Läuft gerade', 'Der Termin hat schon begonnen.')}
          {row(
            'soon',
            'Beginnt bald',
            'in weniger als X Stunden',
            <NumberInput
              label="Stunden vorher"
              min={1}
              max={48}
              allowDecimal={false}
              value={value.soon.hours}
              // Mantine reports a half-typed number such as "06" as a string.
              onChange={(v) => setTier('soon', { hours: typeof v === 'number' ? v : Number(v) || 0 })}
              error={errors[`${path}.soon.hours`]}
              w={{ base: '100%', sm: 200 }}
            />,
          )}
          {row('today', 'Beginnt heute', 'später am heutigen Tag')}
        </Stack>
      )}
    </Stack>
  )
}
