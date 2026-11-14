import { useState } from 'react'
import { ActionIcon, Alert, Card, Group, Menu, Select, Skeleton, Stack, Text, Title } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { IconArrowDown, IconArrowUp, IconArrowsExchange, IconX } from '@tabler/icons-react'
import { ApiError } from '../../../shared/api/client.ts'
import {
  fetchLayout,
  fetchModules,
  layoutQueryKey,
  modulesQueryKey,
  regions,
  saveLayout,
  type Layout,
  type ModuleInfo,
  type Region,
} from '../api/layout.ts'
import { DetailHeader } from '../components/DetailHeader.tsx'
import { UnsavedChangesBar } from '../components/UnsavedChangesBar.tsx'
import { UnsavedChangesDialog } from '../components/UnsavedChangesDialog.tsx'
import { useUnsavedChangesGuard } from '../hooks/useUnsavedChangesGuard.ts'
import { add, move, moveToRegion, placedKeys, regionErrors, remove, sameLayout } from './layoutEdit.ts'
import styles from './LayoutPage.module.css'

export function LayoutPage() {
  const layout = useQuery({ queryKey: layoutQueryKey, queryFn: fetchLayout })
  const modules = useQuery({ queryKey: modulesQueryKey, queryFn: fetchModules })
  const failed = layout.error ?? modules.error
  return (
    <>
      <DetailHeader title="Anordnung" />
      {(layout.isPending || modules.isPending) && !failed && (
        <Stack gap="md" aria-busy="true" aria-label="Einstellungen werden geladen">
          <Skeleton height={120} />
          <Skeleton height={120} />
          <Skeleton height={120} />
        </Stack>
      )}
      {failed && (
        <Alert color="red" title="Die Anordnung konnte nicht geladen werden.">
          {failed.message}
        </Alert>
      )}
      {layout.data && modules.data && <LayoutEditor saved={layout.data} modules={modules.data} />}
    </>
  )
}

function LayoutEditor({ saved, modules }: { saved: Layout; modules: ModuleInfo[] }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<Layout>(saved)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [saveError, setSaveError] = useState<string | null>(null)

  const dirty = !sameLayout(draft, saved)
  const guard = useUnsavedChangesGuard(dirty)

  const nameOf = (key: string) => modules.find((m) => m.key === key)?.name ?? key
  const placed = new Set(placedKeys(draft))
  const unplaced = modules.filter((m) => !placed.has(m.key))

  const mutation = useMutation({
    mutationFn: saveLayout,
    onSuccess: (stored) => {
      queryClient.setQueryData(layoutQueryKey, stored)
      void queryClient.invalidateQueries({ queryKey: ['dashboard', 'layout'] })
      setDraft(stored)
      setErrors({})
      setSaveError(null)
      notifications.show({ color: 'green', message: 'Gespeichert.' })
    },
    onError: (error) => {
      // The draft stays as it is; only the messages are added.
      const fieldErrors = error instanceof ApiError ? (error.fieldErrors ?? {}) : {}
      setErrors(fieldErrors)
      const known = regions.some((r) => regionErrors(fieldErrors, r.key).length > 0)
      setSaveError(known ? null : error instanceof ApiError ? (error.detail ?? error.message) : error.message)
    },
  })

  const discard = () => {
    setDraft(saved)
    setErrors({})
    setSaveError(null)
  }

  return (
    <>
      <Stack gap="md">
        <Text size="sm" c="dimmed">
          So sieht der Spiegel von vorn aus. Ordne die Module den Bereichen zu und bring sie in die gewünschte Reihenfolge.
        </Text>
        <div className={styles.frame}>
          {regions.map((r) => (
            <RegionCard
              key={r.key}
              className={r.key === 'middle' ? styles.middle : r.key.startsWith('bottom') ? styles.bottom : undefined}
              region={r.key}
              label={r.label}
              keys={draft[r.key]}
              nameOf={nameOf}
              unplaced={unplaced}
              errors={regionErrors(errors, r.key)}
              onChange={setDraft}
              layout={draft}
            />
          ))}
        </div>
        <Text size="sm">
          Nicht angezeigt: {unplaced.length > 0 ? unplaced.map((m) => m.name).join(', ') : 'nichts'}
        </Text>
      </Stack>

      {dirty && (
        <UnsavedChangesBar
          areas={['Anordnung']}
          saving={mutation.isPending}
          error={saveError}
          onDiscard={discard}
          onSave={() => mutation.mutate(draft)}
        />
      )}
      <UnsavedChangesDialog opened={guard.asking} onStay={guard.stay} onLeave={guard.leave} />
    </>
  )
}

type CardProps = {
  className?: string
  region: Region
  label: string
  keys: string[]
  layout: Layout
  nameOf: (key: string) => string
  unplaced: ModuleInfo[]
  errors: string[]
  onChange: (layout: Layout) => void
}

function RegionCard({ className, region, label, keys, layout, nameOf, unplaced, errors, onChange }: CardProps) {
  const headingId = `region-${region}`
  return (
    <Card withBorder padding="xs" className={className} component="section" aria-labelledby={headingId} style={{ minWidth: 0 }}>
      <Title order={2} size="h5" id={headingId} mb="xs">
        {label}
      </Title>
      <Stack gap={6}>
        {keys.length === 0 && (
          <Text size="sm" c="dimmed">
            Leer
          </Text>
        )}
        {keys.map((key, index) => {
          const name = nameOf(key)
          return (
            <Group key={key} gap={4} wrap="nowrap" justify="space-between">
              <Text size="sm" fw={500} style={{ minWidth: 0, overflowWrap: 'anywhere', flex: 1 }}>
                {name}
              </Text>
              <Group gap={2} wrap="nowrap">
                <ActionIcon
                  variant="subtle"
                  color="gray"
                  aria-label={`Nach oben: ${name}`}
                  disabled={index === 0}
                  onClick={() => onChange(move(layout, region, index, -1))}
                >
                  <IconArrowUp size={16} />
                </ActionIcon>
                <ActionIcon
                  variant="subtle"
                  color="gray"
                  aria-label={`Nach unten: ${name}`}
                  disabled={index === keys.length - 1}
                  onClick={() => onChange(move(layout, region, index, 1))}
                >
                  <IconArrowDown size={16} />
                </ActionIcon>
                <Menu position="bottom-end" withinPortal>
                  <Menu.Target>
                    <ActionIcon variant="subtle" color="gray" aria-label={`Verschieben nach …: ${name}`}>
                      <IconArrowsExchange size={16} />
                    </ActionIcon>
                  </Menu.Target>
                  <Menu.Dropdown>
                    <Menu.Label>Verschieben nach</Menu.Label>
                    {regions
                      .filter((r) => r.key !== region)
                      .map((r) => (
                        <Menu.Item key={r.key} onClick={() => onChange(moveToRegion(layout, region, index, r.key))}>
                          {r.label}
                        </Menu.Item>
                      ))}
                  </Menu.Dropdown>
                </Menu>
                <ActionIcon
                  variant="subtle"
                  color="red"
                  aria-label={`Entfernen: ${name}`}
                  onClick={() => onChange(remove(layout, region, index))}
                >
                  <IconX size={16} />
                </ActionIcon>
              </Group>
            </Group>
          )
        })}
        {unplaced.length > 0 && (
          <Select
            label="Modul hinzufügen"
            placeholder="Auswählen …"
            size="xs"
            mt={4}
            data={unplaced.map((m) => ({ value: m.key, label: m.name }))}
            comboboxProps={{ keepMounted: false }}
            value={null}
            onChange={(key) => key && onChange(add(layout, region, key))}
          />
        )}
        {errors.map((message) => (
          <Text key={message} size="sm" c="red" role="alert">
            {message}
          </Text>
        ))}
      </Stack>
    </Card>
  )
}
