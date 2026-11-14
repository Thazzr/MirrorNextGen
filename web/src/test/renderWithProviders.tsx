import { QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { routes } from '../app/routes.tsx'
import { createQueryClient } from '../shared/api/queryClient.ts'

export function renderWithProviders(initialEntry = '/') {
  const queryClient = createQueryClient({ retry: false })
  // A data router: the admin pages use useBlocker, which throws under <MemoryRouter>.
  const router = createMemoryRouter(routes, { initialEntries: [initialEntry] })
  const result = render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return { ...result, router, queryClient }
}
