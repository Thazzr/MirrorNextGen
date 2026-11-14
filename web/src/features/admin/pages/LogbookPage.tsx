import { useState } from 'react'
import { Alert, Badge, Button, Group, SegmentedControl, Skeleton, Stack, Text } from '@mantine/core'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { fetchLogbook, logbookQueryKey, type LogbookEntry, type LogLevel } from '../api/logbook.ts'
import { DetailHeader } from '../components/DetailHeader.tsx'

const REFRESH_MS = 10_000

const FILTERS: { label: string; value: LogLevel }[] = [
  { label: 'Alles', value: 'info' },
  { label: 'Warnungen', value: 'warn' },
  { label: 'Fehler', value: 'error' },
]

const LEVELS: Record<LogLevel, { label: string; color: string }> = {
  debug: { label: 'Debug', color: 'gray' },
  info: { label: 'Info', color: 'gray' },
  warn: { label: 'Warnung', color: 'yellow' },
  error: { label: 'Fehler', color: 'red' },
}

const timeFormat = new Intl.DateTimeFormat('de-DE', { dateStyle: 'short', timeStyle: 'medium' })

function Entry({ entry }: { entry: LogbookEntry }) {
  const level = LEVELS[entry.level]
  return (
    <li style={{ listStyle: 'none' }}>
      <Group gap="xs" align="center">
        <Text size="xs" c="dimmed" component="time" dateTime={entry.time}>
          {timeFormat.format(new Date(entry.time))}
        </Text>
        <Badge color={level.color} variant="light" size="sm">
          {level.label}
        </Badge>
      </Group>
      <Text size="sm" style={{ wordBreak: 'break-word' }}>
        {entry.message}
      </Text>
      {entry.attrs.length > 0 && (
        <Text size="xs" c="dimmed" ff="monospace" style={{ wordBreak: 'break-word' }}>
          {entry.attrs.map((a) => `${a.key}=${a.value}`).join(' ')}
        </Text>
      )}
    </li>
  )
}

export function LogbookPage() {
  const [minLevel, setMinLevel] = useState<LogLevel>('info')
  const query = useQuery({
    queryKey: logbookQueryKey(minLevel),
    queryFn: () => fetchLogbook(minLevel),
    refetchInterval: REFRESH_MS,
    placeholderData: keepPreviousData,
  })

  return (
    <>
      <DetailHeader title="Logbuch" />
      <Text size="sm" c="dimmed" mb="md">
        Die letzten 500 Meldungen des Spiegels seit dem letzten Start. Nach einem Neustart beginnt das Logbuch leer.
      </Text>
      <Group justify="space-between" mb="md">
        <SegmentedControl
          aria-label="Meldungen filtern"
          data={FILTERS}
          value={minLevel}
          onChange={(v) => setMinLevel(v as LogLevel)}
        />
        <Button variant="default" onClick={() => void query.refetch()} loading={query.isFetching && !query.isPending}>
          Neu laden
        </Button>
      </Group>
      {query.isPending && (
        <Stack gap="md" aria-busy="true" aria-label="Logbuch wird geladen">
          <Skeleton height={48} />
          <Skeleton height={48} />
          <Skeleton height={48} />
        </Stack>
      )}
      {query.isError && (
        <Alert color="red" title="Das Logbuch konnte nicht geladen werden.">
          {query.error.message}
        </Alert>
      )}
      {query.data && query.data.entries.length === 0 && <Text c="dimmed">Keine Meldungen.</Text>}
      {query.data && query.data.entries.length > 0 && (
        <Stack component="ul" gap="md" p={0} m={0} aria-label="Meldungen">
          {query.data.entries.map((entry, i) => (
            <Entry key={`${entry.time}-${i}`} entry={entry} />
          ))}
        </Stack>
      )}
    </>
  )
}
