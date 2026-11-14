import { Box, Button, Container, Text } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import { IconLogout } from '@tabler/icons-react'
import { Link, Outlet } from 'react-router-dom'
import { fetchInfo, infoQueryKey } from '../api/info.ts'

/** Frame of the signed-in pages: product name, logout, narrow content, version footer. */
export function AdminLayout() {
  const info = useQuery({ queryKey: infoQueryKey, queryFn: fetchInfo, staleTime: Infinity })
  return (
    <Box mih="100vh" style={{ display: 'flex', flexDirection: 'column' }}>
      <Box component="header" style={{ borderBottom: '1px solid var(--mantine-color-dark-4)' }}>
        <Container size={640} h={56} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Text fw={600}>Spiegel-Verwaltung</Text>
          <Button component={Link} to="/admin/logout" variant="subtle" color="gray" leftSection={<IconLogout size={18} />}>
            Abmelden
          </Button>
        </Container>
      </Box>
      <Container size={640} component="main" w="100%" py="lg" style={{ flex: 1 }}>
        <Outlet />
      </Container>
      <Box component="footer" py="md">
        <Container size={640}>
          {info.data && (
            <Text size="xs" c="dimmed">
              Version {info.data.version} ({info.data.commit})
            </Text>
          )}
        </Container>
      </Box>
    </Box>
  )
}
