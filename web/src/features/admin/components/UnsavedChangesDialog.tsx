import { Button, Group, Modal, Text } from '@mantine/core'

type Props = {
  opened: boolean
  onStay: () => void
  onLeave: () => void
}

/** Asks before a held navigation discards the page's unsaved changes. */
export function UnsavedChangesDialog({ opened, onStay, onLeave }: Props) {
  return (
    <Modal opened={opened} onClose={onStay} title="Änderungen verwerfen?" centered>
      <Text size="sm" mb="lg">
        Wenn du die Seite jetzt verlässt, gehen deine ungespeicherten Änderungen verloren.
      </Text>
      <Group justify="flex-end">
        <Button variant="default" onClick={onStay}>
          Weiter bearbeiten
        </Button>
        <Button color="red" onClick={onLeave}>
          Verwerfen
        </Button>
      </Group>
    </Modal>
  )
}
