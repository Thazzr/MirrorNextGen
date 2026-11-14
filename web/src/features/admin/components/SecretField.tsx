import { Button, Group, PasswordInput, Stack, Text } from '@mantine/core'

type Props = {
  label: string
  description?: string
  /** True when the server stores a value; it is never sent to the browser. */
  hasSecret: boolean
  /** Newly typed value; empty keeps the stored one. */
  value: string
  onChange: (value: string) => void
  /** True when the stored value is to be deleted on save. */
  cleared: boolean
  onClearedChange: (cleared: boolean) => void
  removeLabel: string
  /** Shown while no value is stored; a stored value gets the keep hint instead. */
  placeholder?: string
  error?: string
}

/** Secret input with keep (empty), replace (typed) and remove semantics. */
export function SecretField({
  label,
  description,
  hasSecret,
  value,
  onChange,
  cleared,
  onClearedChange,
  removeLabel,
  placeholder,
  error,
}: Props) {
  if (cleared) {
    return (
      <Stack gap={4}>
        <Text size="sm" fw={500}>
          {label}
        </Text>
        <Group gap="sm">
          <Text size="sm">Wird beim Speichern entfernt</Text>
          <Button variant="subtle" size="compact-sm" onClick={() => onClearedChange(false)}>
            Rückgängig
          </Button>
        </Group>
      </Stack>
    )
  }
  return (
    <Stack gap={4}>
      <PasswordInput
        label={label}
        description={description}
        placeholder={hasSecret ? 'Gespeichert – leer lassen zum Behalten' : placeholder}
        autoComplete="new-password"
        value={value}
        onChange={(event) => onChange(event.currentTarget.value)}
        error={error}
      />
      {hasSecret && (
        <div>
          <Button
            variant="subtle"
            color="gray"
            size="compact-sm"
            onClick={() => {
              onChange('')
              onClearedChange(true)
            }}
          >
            {removeLabel}
          </Button>
        </div>
      )}
    </Stack>
  )
}
