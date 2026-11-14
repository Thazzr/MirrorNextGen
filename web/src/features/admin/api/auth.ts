import { api, setCsrfToken, unwrap, unwrapEmpty } from '../../../shared/api/client.ts'
import { rememberSession } from './session.ts'

export async function setupAdmin(password: string) {
  return rememberSession(unwrap(await api.POST('/api/v1/auth/setup', { body: { password } })))
}

export async function login(password: string) {
  return rememberSession(unwrap(await api.POST('/api/v1/auth/login', { body: { password } })))
}

export async function logout() {
  unwrapEmpty(await api.POST('/api/v1/auth/logout'))
  setCsrfToken(undefined)
}

export async function changePassword(currentPassword: string, newPassword: string) {
  unwrapEmpty(await api.POST('/api/v1/auth/password', { body: { currentPassword, newPassword } }))
}
