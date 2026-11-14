import { Navigate, type RouteObject } from 'react-router-dom'
import { DashboardPage } from '../features/dashboard/DashboardPage.tsx'

export const routes: RouteObject[] = [
  { path: '/', element: <DashboardPage /> },
  { path: '*', element: <Navigate to="/" replace /> },
]
