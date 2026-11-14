import { Anchor, Stack, Title } from '@mantine/core'
import { Link } from 'react-router-dom'

/** Back link to the overview and the page's one h1. */
export function DetailHeader({ title }: { title: string }) {
  return (
    <Stack gap="xs" mb="lg">
      <Anchor component={Link} to="/admin" size="sm">
        ← Übersicht
      </Anchor>
      <Title order={1}>{title}</Title>
    </Stack>
  )
}
