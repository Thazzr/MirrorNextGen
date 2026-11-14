import '@testing-library/jest-dom/vitest'
import { cleanup, configure } from '@testing-library/react'
import { afterEach } from 'vitest'
import { setCsrfToken } from '../shared/api/client.ts'

// The gate runs the Go tests in parallel and CI runners are slower; findBy's default of 1 s is too tight
// for the lazily loaded admin chunk.
configure({ asyncUtilTimeout: 5000 })

// Mantine reads the colour scheme and element sizes, which jsdom does not provide.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  }),
})

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver = ResizeObserverStub
window.HTMLElement.prototype.scrollIntoView = () => {}

afterEach(() => {
  cleanup()
  setCsrfToken(undefined)
})
