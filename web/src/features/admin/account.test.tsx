import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { authedSession, info, json, mockApi, noContent, problem } from '../../test/mockApi.ts'
import { expectSinglePageTitle } from '../../test/pageTitle.ts'
import { renderWithProviders } from '../../test/renderWithProviders.tsx'

afterEach(() => vi.unstubAllGlobals())

function arrange(change: () => Response) {
  return mockApi({
    'GET /api/v1/auth/session': () => json(authedSession),
    'GET /api/v1/info': () => json(info),
    'POST /api/v1/auth/password': change,
  })
}

async function fill(current: string, next: string) {
  await screen.findByRole('heading', { level: 1, name: 'Konto' })
  await userEvent.type(screen.getByLabelText('Aktuelles Passwort'), current)
  await userEvent.type(screen.getByLabelText('Neues Passwort', { selector: 'input' }), next)
  await userEvent.type(screen.getByLabelText('Neues Passwort wiederholen'), next)
  await userEvent.click(screen.getByRole('button', { name: 'Passwort ändern' }))
}

describe('account page', () => {
  it('has one h1, a back link and an h2', async () => {
    arrange(() => noContent())
    renderWithProviders('/admin/account')
    await screen.findByRole('heading', { level: 1, name: 'Konto' })
    expectSinglePageTitle()
    expect(screen.getByRole('heading', { level: 2, name: 'Passwort ändern' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '← Übersicht' })).toHaveAttribute('href', '/admin')
  })

  it('shows the error on the current password field for a wrong one', async () => {
    arrange(() => problem(401, 'Nicht angemeldet'))
    renderWithProviders('/admin/account')
    await fill('falsch-falsch', 'neues-passwort-1')
    expect(await screen.findByText('Das aktuelle Passwort stimmt nicht.')).toBeInTheDocument()
  })

  it('shows a server message for the new password on its field', async () => {
    arrange(() => problem(400, 'Ungültig', { errors: { newPassword: 'Zu einfach.' } }))
    renderWithProviders('/admin/account')
    await fill('aktuell-aktuell', 'neues-passwort-1')
    expect(await screen.findByText('Zu einfach.')).toBeInTheDocument()
  })

  it('confirms with a notification, sends the CSRF token and clears the fields', async () => {
    const api = arrange(() => noContent())
    renderWithProviders('/admin/account')
    await fill('aktuell-aktuell', 'neues-passwort-1')
    expect(await screen.findByText('Passwort geändert. Andere Geräte wurden abgemeldet.')).toBeInTheDocument()
    expect(screen.getByLabelText('Aktuelles Passwort')).toHaveValue('')
    expect(screen.getByLabelText('Neues Passwort', { selector: 'input' })).toHaveValue('')
    const call = api.callsTo('POST /api/v1/auth/password')[0]
    expect(call.csrf).toBe('tok-123')
    expect(call.body).toEqual({ currentPassword: 'aktuell-aktuell', newPassword: 'neues-passwort-1' })
  })

  describe('new password length limits', () => {
    async function submitLength(length: number) {
      renderWithProviders('/admin/account')
      await screen.findByRole('heading', { level: 1, name: 'Konto' })
      const password = 'a'.repeat(length)
      await userEvent.type(screen.getByLabelText('Aktuelles Passwort'), 'aktuell-aktuell')
      await userEvent.click(screen.getByLabelText('Neues Passwort', { selector: 'input' }))
      await userEvent.paste(password)
      await userEvent.click(screen.getByLabelText('Neues Passwort wiederholen'))
      await userEvent.paste(password)
      await userEvent.click(screen.getByRole('button', { name: 'Passwort ändern' }))
    }

    it.each([
      [9, 'Das Passwort muss mindestens 10 Zeichen lang sein.'],
      [129, 'Das Passwort darf höchstens 128 Zeichen lang sein.'],
    ])('rejects %i characters on the client', async (length, message) => {
      const api = arrange(() => noContent())
      await submitLength(length)
      expect(await screen.findByText(message)).toBeInTheDocument()
      expect(api.callsTo('POST /api/v1/auth/password')).toHaveLength(0)
    })

    it.each([10, 128])('accepts %i characters', async (length) => {
      const api = arrange(() => noContent())
      await submitLength(length)
      await waitFor(() => expect(api.callsTo('POST /api/v1/auth/password')).toHaveLength(1))
    })
  })
})
