import { useEffect, type ReactNode } from 'react'
import { Alert, Box, Button, Container, Group, Paper, Stack, Text } from '@mantine/core'

type Props = {
  /** Names of the changed areas, shown after the label. */
  areas: string[]
  saving: boolean
  error?: ReactNode
  onDiscard: () => void
  onSave: () => void
}

/** Sticky bar at the foot of a settings page; it only exists while there are changes. */
export function UnsavedChangesBar({ areas, saving, error, onDiscard, onSave }: Props) {
  // Focused or scrolled-to fields must not end up under the fixed bar.
  useEffect(() => {
    const root = document.documentElement
    root.style.scrollPaddingBottom = '160px'
    return () => {
      root.style.scrollPaddingBottom = ''
    }
  }, [])

  return (
    <>
      {/* Keeps the last section clear of the fixed bar. */}
      <Box h={error ? 190 : 110} aria-hidden />
      <Paper
        role="region"
        aria-label="Ungespeicherte Änderungen"
        withBorder
        radius={0}
        p="sm"
        style={{ position: 'fixed', left: 0, right: 0, bottom: 0, zIndex: 200 }}
      >
        <Container size={640} px={0}>
          <Stack gap="xs">
            {error && (
              <Alert color="red" variant="light">
                {error}
              </Alert>
            )}
            <Group justify="space-between" align="center" wrap="wrap" gap="sm">
              <Text size="sm" style={{ flex: '1 1 200px' }}>
                Ungespeicherte Änderungen: {areas.join(', ')}
              </Text>
              <Group gap="xs" wrap="nowrap">
                <Button variant="default" onClick={onDiscard} disabled={saving}>
                  Verwerfen
                </Button>
                <Button onClick={onSave} loading={saving}>
                  Speichern
                </Button>
              </Group>
            </Group>
          </Stack>
        </Container>
      </Paper>
    </>
  )
}
