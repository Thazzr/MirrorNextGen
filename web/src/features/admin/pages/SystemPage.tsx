import { useEffect, useRef, useState } from 'react'
import { Alert, Button, Group, Modal, Skeleton, Stack, Text, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '../../../shared/api/client.ts'
import { fetchInfo, infoQueryKey } from '../api/info.ts'
import {
  fetchSystemUpdate,
  requestSystemUpdate,
  systemUpdateQueryKey,
  UPDATE_POLL_MS,
  updateIsActive,
  type UpdateStatus,
} from '../api/system.ts'
import { DetailHeader } from '../components/DetailHeader.tsx'

const dateTime = new Intl.DateTimeFormat('de-DE', { dateStyle: 'short', timeStyle: 'short' })
const clock = new Intl.DateTimeFormat('de-DE', { hour: '2-digit', minute: '2-digit' })

function formatBuildTime(value: string): string {
  const d = new Date(value)
  return value === '' || Number.isNaN(d.getTime()) ? 'unbekannt' : dateTime.format(d)
}

function VersionSection() {
  const active = useActiveUpdate()
  const info = useQuery({
    queryKey: infoQueryKey,
    queryFn: fetchInfo,
    // While the mirror restarts, ask until it answers with its new version.
    refetchInterval: active ? UPDATE_POLL_MS : false,
  })
  return (
    <section aria-labelledby="version-heading">
      <Title order={2} size="h3" mb="sm" id="version-heading">
        Version
      </Title>
      {info.isPending && <Skeleton height={60} width={240} aria-label="Version wird geladen" />}
      {info.isError && !info.data && (
        <Alert color="red" title="Die Version konnte nicht geladen werden.">
          {info.error.message}
        </Alert>
      )}
      {info.data && (
        <Stack gap={2}>
          <Text>Installierte Version: {info.data.version}</Text>
          <Text>Commit: {info.data.commit === 'unknown' ? 'unbekannt' : info.data.commit.slice(0, 7)}</Text>
          <Text>Gebaut am: {formatBuildTime(info.data.buildTime)}</Text>
        </Stack>
      )}
    </section>
  )
}

/** The update state as the page last saw it; polls while an update is active. */
function useUpdateQuery() {
  return useQuery({
    queryKey: systemUpdateQueryKey,
    queryFn: fetchSystemUpdate,
    // After a failed poll `state.data` still holds the last answer, so polling
    // goes on through the mirror's restart.
    refetchInterval: (q) => (updateIsActive(q.state.data) ? UPDATE_POLL_MS : false),
  })
}

function useActiveUpdate(): boolean {
  return updateIsActive(useUpdateQuery().data)
}

function statusLine(s: UpdateStatus) {
  if (s.state === 'running') {
    return s.startedAt ? `Update läuft seit ${clock.format(new Date(s.startedAt))} …` : 'Update läuft …'
  }
  if (s.stalled) {
    return 'Der Update-Dienst hat die Anfrage nicht übernommen. Ist er auf dem Spiegel eingerichtet? Siehe deploy/INSTALL.md, Abschnitt „Updates per Knopf“.'
  }
  if (s.pending) return 'Angefordert – wartet auf den Update-Dienst …'
  const when = s.finishedAt ?? s.startedAt
  switch (s.state) {
    case 'succeeded':
      return when ? `Zuletzt aktualisiert am ${dateTime.format(new Date(when))}.` : 'Zuletzt aktualisiert.'
    case 'failed':
      return when ? `Update fehlgeschlagen am ${dateTime.format(new Date(when))}:` : 'Update fehlgeschlagen:'
    case 'unknown':
      return s.message ?? 'Der Status des Updates ist nicht lesbar.'
    default:
      return 'Noch kein Update angefordert.'
  }
}

function UpdateSection() {
  const queryClient = useQueryClient()
  const query = useUpdateQuery()
  const [confirming, setConfirming] = useState(false)
  const status = query.data
  const active = updateIsActive(status)

  // When an update ends, look at the version once more: the mirror may have restarted into a new one.
  const wasActive = useRef(false)
  useEffect(() => {
    if (wasActive.current && !active) void queryClient.invalidateQueries({ queryKey: infoQueryKey })
    wasActive.current = active
  }, [active, queryClient])

  const request = useMutation({
    mutationFn: requestSystemUpdate,
    onSuccess: (data) => queryClient.setQueryData(systemUpdateQueryKey, data),
    onError: (error) => {
      // 409: someone else started it meanwhile; show what is going on.
      void queryClient.invalidateQueries({ queryKey: systemUpdateQueryKey })
      notifications.show({
        color: 'red',
        message: error instanceof ApiError ? (error.detail ?? error.message) : error.message,
      })
    },
  })

  const restarting = active && query.isError

  return (
    <section aria-labelledby="update-heading">
      <Title order={2} size="h3" mb="sm" id="update-heading">
        Aktualisieren
      </Title>
      {query.isPending && <Skeleton height={40} aria-label="Update-Status wird geladen" />}
      {query.isError && !restarting && (
        <Alert color="red" title="Der Update-Status konnte nicht geladen werden.">
          {query.error.message}
        </Alert>
      )}
      {status && !status.available && (
        <Text c="dimmed">
          Updates per Knopf sind auf diesem Spiegel nicht eingerichtet. Wie das geht, steht in deploy/INSTALL.md im
          Abschnitt „Updates per Knopf“.
        </Text>
      )}
      {status?.available && (
        <Stack gap="sm" align="flex-start">
          {restarting ? <Text role="status">Der Spiegel startet neu …</Text> : <Text role="status">{statusLine(status)}</Text>}
          {!restarting && status.state === 'failed' && !status.pending && status.message && (
            <Text size="xs" ff="monospace" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {status.message}
            </Text>
          )}
          {!active && (
            <Button onClick={() => setConfirming(true)} loading={request.isPending}>
              Jetzt aktualisieren
            </Button>
          )}
        </Stack>
      )}
      <Modal opened={confirming} onClose={() => setConfirming(false)} title="Jetzt aktualisieren?" centered>
        <Text size="sm" mb="lg">
          Der Spiegel lädt die neuen Images und startet neu. Das dauert einige Minuten; die Anzeige ist kurz schwarz.
        </Text>
        <Group justify="flex-end">
          <Button variant="default" onClick={() => setConfirming(false)}>
            Abbrechen
          </Button>
          <Button
            onClick={() => {
              setConfirming(false)
              request.mutate()
            }}
          >
            Aktualisieren
          </Button>
        </Group>
      </Modal>
    </section>
  )
}

export function SystemPage() {
  return (
    <>
      <DetailHeader title="System" />
      <Stack gap="xl">
        <VersionSection />
        <UpdateSection />
      </Stack>
    </>
  )
}
