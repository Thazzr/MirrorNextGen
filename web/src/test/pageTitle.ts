import { expect } from 'vitest'
import { screen } from '@testing-library/react'

/** Asserts the page has exactly one h1 and returns its text. */
export function expectSinglePageTitle(name?: string) {
  const headings = screen.getAllByRole('heading', { level: 1 })
  expect(headings).toHaveLength(1)
  if (name) expect(headings[0]).toHaveTextContent(name)
}
