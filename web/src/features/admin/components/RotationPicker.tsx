import type { ReactNode } from 'react'
import { Input, SimpleGrid, Stack, Text } from '@mantine/core'
import type { DisplaySettings } from '../api/display.ts'
import { RotationPreview } from './RotationPreview.tsx'
import classes from './RotationPicker.module.css'

type Rotation = DisplaySettings['rotation']
const rotations: Rotation[] = [0, 90, 180, 270]

type Props = {
  value: Rotation
  onChange: (value: Rotation) => void
  error?: ReactNode
}

/**
 * Native radio inputs of one group: the browser provides the arrow-key navigation.
 * The inputs are visually hidden; the card shows focus and the checked state.
 */
export function RotationPicker({ value, onChange, error }: Props) {
  return (
    <Stack gap={4}>
      <SimpleGrid cols={{ base: 2, xs: 4 }} spacing="xs" role="radiogroup" aria-labelledby="display-rotation">
        {rotations.map((degrees) => (
          <label key={degrees} className={classes.card} data-checked={value === degrees || undefined}>
            <input
              type="radio"
              name="rotation"
              className={classes.input}
              value={degrees}
              checked={value === degrees}
              onChange={() => onChange(degrees)}
            />
            <Stack gap={4} align="center">
              <RotationPreview degrees={degrees} />
              <Text fw={500}>{degrees}°</Text>
            </Stack>
          </label>
        ))}
      </SimpleGrid>
      {error && <Input.Error>{error}</Input.Error>}
    </Stack>
  )
}
