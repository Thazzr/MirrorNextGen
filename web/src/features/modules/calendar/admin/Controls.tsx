import { ActionIcon, Group } from '@mantine/core'
import { IconArrowDown, IconArrowUp, IconTrash } from '@tabler/icons-react'

type Props = {
  /** Spoken name of the item, for example "Regel 2". */
  name: string
  index: number
  count: number
  onMove: (delta: -1 | 1) => void
  onRemove: () => void
  removeLabel?: string
}

/** Up, down and remove buttons of a list item; the aria-labels name the item. */
export function ItemControls({ name, index, count, onMove, onRemove, removeLabel }: Props) {
  return (
    <Group gap={4} wrap="nowrap">
      <ActionIcon variant="subtle" color="gray" aria-label={`${name} nach oben`} disabled={index === 0} onClick={() => onMove(-1)}>
        <IconArrowUp size={18} aria-hidden />
      </ActionIcon>
      <ActionIcon
        variant="subtle"
        color="gray"
        aria-label={`${name} nach unten`}
        disabled={index === count - 1}
        onClick={() => onMove(1)}
      >
        <IconArrowDown size={18} aria-hidden />
      </ActionIcon>
      <ActionIcon variant="subtle" color="red" aria-label={removeLabel ?? `${name} entfernen`} onClick={onRemove}>
        <IconTrash size={18} aria-hidden />
      </ActionIcon>
    </Group>
  )
}
