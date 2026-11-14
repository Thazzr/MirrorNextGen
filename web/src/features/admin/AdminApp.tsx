import { createTheme, MantineProvider, Menu } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { Suspense } from 'react'
import { Route, Routes } from 'react-router-dom'
import '@mantine/core/styles.css'
import '@mantine/notifications/styles.css'
import './admin.css'
import { ProtectedLayout } from './components/ProtectedLayout.tsx'
import { PublicGate } from './components/PublicGate.tsx'
import { LoginPage } from './pages/LoginPage.tsx'
import { LogoutPage } from './pages/LogoutPage.tsx'
import { OverviewPage } from './pages/OverviewPage.tsx'
import { SetupPage } from './pages/SetupPage.tsx'
import { adminModules } from './modules.ts'

const font = '"IBM Plex Sans", system-ui, sans-serif'
const theme = createTheme({
  primaryColor: 'yellow',
  autoContrast: true,
  fontFamily: font,
  headings: { fontFamily: font },
  defaultRadius: 'md',
  components: {
    // Opens at once: no timers to wait for, in the browser as in the tests.
    Menu: Menu.extend({ defaultProps: { transitionProps: { duration: 0 } } }),
  },
})

/** Root of the lazily loaded admin chunk: the only place that brings Mantine in. */
export function AdminApp() {
  return (
    <MantineProvider theme={theme} forceColorScheme="dark">
      <Notifications position="top-center" />
      <Suspense fallback={null}>
      <Routes>
        <Route path="login" element={<PublicGate page="login"><LoginPage /></PublicGate>} />
        <Route path="setup" element={<PublicGate page="setup"><SetupPage /></PublicGate>} />
        <Route path="logout" element={<LogoutPage />} />
        <Route element={<ProtectedLayout />}>
          <Route index element={<OverviewPage />} />
          {adminModules.map(({ key, path, Page }) => (
            <Route key={key} path={path} element={<Page />} />
          ))}
          <Route path="*" element={<OverviewPage />} />
        </Route>
      </Routes>
      </Suspense>
    </MantineProvider>
  )
}
