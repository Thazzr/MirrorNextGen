import { useState } from 'react'
import { Alert, Skeleton, Stack, Tabs } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '../../../../shared/api/client.ts'
import { DetailHeader } from '../../../admin/components/DetailHeader.tsx'
import { UnsavedChangesBar } from '../../../admin/components/UnsavedChangesBar.tsx'
import { UnsavedChangesDialog } from '../../../admin/components/UnsavedChangesDialog.tsx'
import { useUnsavedChangesGuard } from '../../../admin/hooks/useUnsavedChangesGuard.ts'
import {
  calendarSettingsQueryKey,
  dashboardCalendarQueryKey,
  dashboardLayoutQueryKey,
  fetchCalendarSettings,
  saveCalendarSettings,
  type CalendarSettings,
  type CalendarSettingsUpdate,
} from './api.ts'
import { CalendarsTab } from './CalendarsTab.tsx'
import { changedTabs, TAB_LABEL, tabOfError, toBody, toDraft, type TabKey } from './draft.ts'
import { PreviewTab } from './PreviewTab.tsx'
import { RulesTab } from './RulesTab.tsx'
import { SectionsTab } from './SectionsTab.tsx'

export function CalendarPage() {
  const query = useQuery({ queryKey: calendarSettingsQueryKey, queryFn: fetchCalendarSettings })
  return (
    <>
      <DetailHeader title="Kalender" />
      {query.isPending && (
        <Stack gap="md" aria-busy="true" aria-label="Einstellungen werden geladen">
          <Skeleton height={40} />
          <Skeleton height={160} />
          <Skeleton height={160} />
        </Stack>
      )}
      {query.isError && (
        <Alert color="red" title="Die Einstellungen konnten nicht geladen werden.">
          {query.error.message}
        </Alert>
      )}
      {query.data && <CalendarForm saved={query.data} />}
    </>
  )
}

function CalendarForm({ saved }: { saved: CalendarSettings }) {
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState<CalendarSettingsUpdate>(() => toDraft(saved))
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [saveError, setSaveError] = useState<string | null>(null)
  const [tab, setTab] = useState<TabKey>('calendars')

  const areas = changedTabs(draft, saved).map((t) => TAB_LABEL[t])
  const dirty = areas.length > 0
  const guard = useUnsavedChangesGuard(dirty)

  const mutation = useMutation({
    mutationFn: saveCalendarSettings,
    onSuccess: (stored) => {
      queryClient.setQueryData(calendarSettingsQueryKey, stored)
      void queryClient.invalidateQueries({ queryKey: dashboardCalendarQueryKey })
      void queryClient.invalidateQueries({ queryKey: dashboardLayoutQueryKey })
      setDraft(toDraft(stored))
      setErrors({})
      setSaveError(null)
      notifications.show({ color: 'green', message: 'Gespeichert.' })
    },
    onError: (error) => {
      const fieldErrors = error instanceof ApiError ? (error.fieldErrors ?? {}) : {}
      setErrors(fieldErrors)
      const failing = [...new Set(Object.keys(fieldErrors).map(tabOfError))].filter((t) => t !== undefined)
      if (failing.length > 0) {
        if (!failing.includes(tab as never)) setTab(failing[0])
        setSaveError(`Bitte prüfe die Eingaben in: ${failing.map((t) => TAB_LABEL[t]).join(', ')}.`)
      } else {
        setSaveError(error instanceof ApiError ? (error.detail ?? error.message) : error.message)
      }
    },
  })

  const discard = () => {
    setDraft(toDraft(saved))
    setErrors({})
    setSaveError(null)
  }

  const failingTabs = new Set(Object.keys(errors).map(tabOfError))
  const label = (key: Exclude<TabKey, 'preview'>) => (failingTabs.has(key) ? `${TAB_LABEL[key]} (Fehler)` : TAB_LABEL[key])

  return (
    <>
      <Tabs value={tab} onChange={(v) => setTab((v ?? 'calendars') as TabKey)} keepMounted>
        <Tabs.List mb="md" style={{ flexWrap: 'wrap' }}>
          <Tabs.Tab value="calendars" color={failingTabs.has('calendars') ? 'red' : undefined}>
            {label('calendars')}
          </Tabs.Tab>
          <Tabs.Tab value="sections" color={failingTabs.has('sections') ? 'red' : undefined}>
            {label('sections')}
          </Tabs.Tab>
          <Tabs.Tab value="rules" color={failingTabs.has('rules') ? 'red' : undefined}>
            {label('rules')}
          </Tabs.Tab>
          <Tabs.Tab value="preview">Vorschau</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="calendars">
          <CalendarsTab draft={draft} saved={saved} errors={errors} onChange={(calendars) => setDraft({ ...draft, calendars })} />
        </Tabs.Panel>
        <Tabs.Panel value="sections">
          <SectionsTab draft={draft} errors={errors} onChange={(sections) => setDraft({ ...draft, sections })} />
        </Tabs.Panel>
        <Tabs.Panel value="rules">
          <RulesTab draft={draft} errors={errors} onChange={(rules) => setDraft({ ...draft, rules })} />
        </Tabs.Panel>
        <Tabs.Panel value="preview">
          <PreviewTab draft={draft} active={tab === 'preview'} />
        </Tabs.Panel>
      </Tabs>

      {dirty && (
        <UnsavedChangesBar
          areas={areas}
          saving={mutation.isPending}
          error={saveError}
          onDiscard={discard}
          onSave={() => {
            setErrors({})
            setSaveError(null)
            mutation.mutate(toBody(draft))
          }}
        />
      )}
      <UnsavedChangesDialog opened={guard.asking} onStay={guard.stay} onLeave={guard.leave} />
    </>
  )
}
