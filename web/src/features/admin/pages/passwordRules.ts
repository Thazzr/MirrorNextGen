/** Same limits as the server: 10 to 128 characters. */
export function passwordLengthError(value: string): string | null {
  if (value.length < 10) return 'Das Passwort muss mindestens 10 Zeichen lang sein.'
  if (value.length > 128) return 'Das Passwort darf höchstens 128 Zeichen lang sein.'
  return null
}
