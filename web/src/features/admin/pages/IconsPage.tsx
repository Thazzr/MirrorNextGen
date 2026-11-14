import { useState } from 'react'
import { Alert, Button, Card, FileInput, Group, SimpleGrid, Skeleton, Stack, Text, TextInput, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '../../../shared/api/client.ts'
import { DetailHeader } from '../components/DetailHeader.tsx'
import { deleteIcon, fetchIcons, iconFileUrl, iconsQueryKey, uploadIcon, type Icon } from '../api/icons.ts'

function IconCard({ icon }: { icon: Icon }) {
  const queryClient = useQueryClient()
  const [confirming, setConfirming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const remove = useMutation({
    mutationFn: () => deleteIcon(icon.id),
    onSuccess: () => {
      setError(null)
      void queryClient.invalidateQueries({ queryKey: iconsQueryKey })
    },
    onError: (e) => {
      setConfirming(false)
      setError(e instanceof ApiError && e.status === 409 ? 'Das Symbol wird noch verwendet.' : e.message)
    },
  })
  return (
    <Card withBorder padding="sm" role="group" aria-label={icon.name}>
      <Stack gap="xs" align="center">
        <img src={iconFileUrl(icon.id)} alt="" width={48} height={48} style={{ objectFit: 'contain' }} />
        <Text size="sm" fw={500} ta="center" style={{ overflowWrap: 'anywhere' }}>
          {icon.name}
        </Text>
        {error && (
          <Text size="sm" c="red" ta="center" role="alert">
            {error}
          </Text>
        )}
        {confirming ? (
          <Group gap="xs" justify="center">
            <Button size="compact-sm" color="red" loading={remove.isPending} onClick={() => remove.mutate()}>
              Wirklich löschen
            </Button>
            <Button size="compact-sm" variant="default" onClick={() => setConfirming(false)}>
              Abbrechen
            </Button>
          </Group>
        ) : (
          <Button size="compact-sm" variant="subtle" color="red" onClick={() => setConfirming(true)} aria-label={`${icon.name} löschen`}>
            Löschen
          </Button>
        )}
      </Stack>
    </Card>
  )
}

function UploadForm() {
  const queryClient = useQueryClient()
  const [file, setFile] = useState<File | null>(null)
  const [name, setName] = useState('')
  const [error, setError] = useState<string | null>(null)
  // Changing the key empties the uncontrolled file input after a successful upload.
  const [round, setRound] = useState(0)

  const upload = useMutation({
    mutationFn: uploadIcon,
    onSuccess: () => {
      setFile(null)
      setName('')
      setError(null)
      setRound((r) => r + 1)
      void queryClient.invalidateQueries({ queryKey: iconsQueryKey })
      notifications.show({ color: 'green', message: 'Symbol hochgeladen.' })
    },
    onError: (e) => {
      const fieldErrors = e instanceof ApiError ? e.fieldErrors : undefined
      setError(fieldErrors?.file ?? fieldErrors?.name ?? (e instanceof ApiError ? (e.detail ?? e.message) : e.message))
    },
  })

  return (
    <Card withBorder padding="md">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          if (!file) {
            setError('Wähle zuerst eine Datei aus.')
            return
          }
          setError(null)
          upload.mutate({ file, name: name.trim() || undefined })
        }}
      >
        <Stack gap="sm">
          <Title order={2} size="h4">
            Symbol hochladen
          </Title>
          <FileInput
            key={round}
            label="Datei"
            description="SVG oder PNG, höchstens 256 KB."
            accept=".svg,.png,image/svg+xml,image/png"
            placeholder="Datei auswählen"
            value={file}
            onChange={setFile}
            clearable
          />
          <TextInput
            label="Name (optional)"
            description="Ohne Angabe gilt der Dateiname."
            maxLength={60}
            value={name}
            onChange={(e) => setName(e.currentTarget.value)}
          />
          {error && (
            <Alert color="red" variant="light" role="alert">
              {error}
            </Alert>
          )}
          <div>
            <Button type="submit" loading={upload.isPending}>
              Hochladen
            </Button>
          </div>
        </Stack>
      </form>
    </Card>
  )
}

export function IconsPage() {
  const query = useQuery({ queryKey: iconsQueryKey, queryFn: fetchIcons })
  return (
    <>
      <DetailHeader title="Symbole" />
      <Stack gap="lg">
        <Text size="sm" c="dimmed">
          Eigene Symbole kannst du bei Kalendern, Regeln und Schichten auswählen.
        </Text>
        <UploadForm />
        <Title order={2} size="h4">
          Eigene Symbole
        </Title>
        {query.isPending && (
          <SimpleGrid cols={{ base: 2, xs: 3, sm: 4 }} aria-busy="true" aria-label="Symbole werden geladen">
            <Skeleton height={120} />
            <Skeleton height={120} />
          </SimpleGrid>
        )}
        {query.isError && (
          <Alert color="red" title="Die Symbole konnten nicht geladen werden.">
            {query.error.message}
          </Alert>
        )}
        {query.data && query.data.length === 0 && <Text c="dimmed">Noch keine eigenen Symbole hochgeladen.</Text>}
        {query.data && query.data.length > 0 && (
          <SimpleGrid cols={{ base: 2, xs: 3, sm: 4 }}>
            {query.data.map((icon) => (
              <IconCard key={icon.id} icon={icon} />
            ))}
          </SimpleGrid>
        )}
      </Stack>
    </>
  )
}
