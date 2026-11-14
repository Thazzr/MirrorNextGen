import type { ReactNode } from 'react'
import { Card, Group, SimpleGrid, Skeleton, Text, Title } from '@mantine/core'
import { Link } from 'react-router-dom'
import { adminModules, type AdminModule } from '../modules.ts'

function Tile({ module }: { module: AdminModule }) {
  const Icon = module.icon
  const status = module.useStatus()
  let line: ReactNode
  if (status.loading) line = <Skeleton height={14} mt={6} width={160} />
  else
    line = (
      <Text size="sm" c="dimmed">
        {status.text}
      </Text>
    )
  return (
    <Card
      component={Link}
      to={`/admin/${module.path}`}
      withBorder
      padding="md"
      style={{ textDecoration: 'none', color: 'inherit' }}
    >
      <Group wrap="nowrap" align="flex-start">
        {/* Fixed size: a long status line must not squeeze the icon. */}
        <div style={{ flex: 'none', display: 'flex' }}>
          <Icon size={28} aria-hidden />
        </div>
        <div style={{ minWidth: 0 }}>
          <Text fw={600}>{module.title}</Text>
          {line}
        </div>
      </Group>
    </Card>
  )
}

export function OverviewPage() {
  return (
    <>
      <Title order={1} mb="lg">
        Übersicht
      </Title>
      <SimpleGrid cols={{ base: 1, xs: 2 }}>
        {adminModules.map((module) => (
          <Tile key={module.key} module={module} />
        ))}
      </SimpleGrid>
    </>
  )
}
