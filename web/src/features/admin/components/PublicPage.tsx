import type { ReactNode } from 'react'
import { Container, Stack, Text, Title } from '@mantine/core'

/** Frame of the pages before sign-in: product name above one h1 and the form. */
export function PublicPage({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Container size={420} component="main" py="xl">
      <Stack gap="md">
        <Text fw={600} c="dimmed">
          Spiegel-Verwaltung
        </Text>
        <Title order={1} size="h2">
          {title}
        </Title>
        {children}
      </Stack>
    </Container>
  )
}
