import { Navigate, type RouteObject } from 'react-router-dom'
import { DashboardPage } from '../features/dashboard/DashboardPage.tsx'

export const routes: RouteObject[] = [
  { path: '/', element: <DashboardPage /> },
  {
    // A chunk of its own: Mantine and its global styles must never reach the dashboard.
    path: '/admin/*',
    lazy: async () => ({ Component: (await import('../features/admin/AdminApp.tsx')).AdminApp }),
    HydrateFallback: () => null,
  },
  { path: '*', element: <Navigate to="/" replace /> },
]
