import type { ReactNode } from 'react'
import { Card, Group, SimpleGrid, Skeleton, Text, Title } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { IconDeviceDesktop, IconUserShield } from '@tabler/icons-react'
import { Link } from 'react-router-dom'
import { displaySettingsQueryKey, fetchDisplaySettings } from '../api/display.ts'

function Tile({ to, icon, name, children }: { to: string; icon: ReactNode; name: string; children: ReactNode }) {
  return (
    <Card component={Link} to={to} withBorder padding="md" style={{ textDecoration: 'none', color: 'inherit' }}>
      <Group wrap="nowrap" align="flex-start">
        {icon}
        <div style={{ minWidth: 0 }}>
          <Text fw={600}>{name}</Text>
          {children}
        </div>
      </Group>
    </Card>
  )
}

export function OverviewPage() {
  const display = useQuery({ queryKey: displaySettingsQueryKey, queryFn: fetchDisplaySettings })
  let displayStatus: ReactNode
  if (display.isPending) displayStatus = <Skeleton height={14} mt={6} width={160} />
  else if (display.isError) displayStatus = <Text size="sm" c="dimmed">Status nicht verfügbar</Text>
  else
    displayStatus = (
      <Text size="sm" c="dimmed">
        Drehung {display.data.rotation}° · Testbild {display.data.testPattern ? 'an' : 'aus'}
      </Text>
    )

  return (
    <>
      <Title order={1} mb="lg">
        Übersicht
      </Title>
      <SimpleGrid cols={{ base: 1, xs: 2 }}>
        <Tile to="/admin/display" icon={<IconDeviceDesktop size={28} aria-hidden />} name="Anzeige">
          {displayStatus}
        </Tile>
        <Tile to="/admin/account" icon={<IconUserShield size={28} aria-hidden />} name="Konto">
          <Text size="sm" c="dimmed">
            Passwort ändern
          </Text>
        </Tile>
      </SimpleGrid>
    </>
  )
}
