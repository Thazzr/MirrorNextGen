import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { api, unwrap } from '../../shared/api/client.ts'
import { resolveWidget, type Region as RegionName } from '../modules/widgets.ts'
import { DiagnoseOverlay } from './DiagnoseOverlay.tsx'
import { RotatedViewport } from './RotatedViewport.tsx'
import { normalizeRotation, useBrowserRotation } from './useBrowserRotation.ts'
import './dashboard.css'

const kioskStateInterval = 5000

// Same as the server default: the mirror is never empty while the layout is not known yet or its request failed.
const defaultLayout = {
  topLeft: ['clock'],
  topRight: ['weather'],
  middle: [],
  bottomLeft: ['bring'],
  bottomRight: ['wifi'],
} satisfies Record<RegionName, string[]>

const layoutInterval = 30_000

/** The only place that drops keys without a widget ("toString" is no module, "calendar:<id>" is). */
function knownKeys(keys: string[]) {
  return keys.filter((key) => resolveWidget(key) !== undefined)
}

/** The widgets of one region in order; `keys` are already known ones, an empty region renders nothing. */
function Region({ name, keys }: { name: RegionName; keys: string[] }) {
  if (keys.length === 0) return null
  return (
    <div className={`mirror-region mirror-region--${name}`} data-region={name}>
      {keys.map((key) => {
        const found = resolveWidget(key)
        if (!found) return null
        return found.instanceId === undefined ? (
          <found.Widget key={key} region={name} />
        ) : (
          <found.Widget key={key} region={name} instanceId={found.instanceId} />
        )
      })}
    </div>
  )
}

export function DashboardPage() {
  const [searchParams] = useSearchParams()
  // A failing request only means "no test pattern": the mirror shows no error text for it.
  const kioskState = useQuery({
    queryKey: ['kiosk', 'state'],
    queryFn: async () => unwrap(await api.GET('/api/v1/kiosk/state')),
    refetchInterval: kioskStateInterval,
  })

  const layoutQuery = useQuery({
    queryKey: ['dashboard', 'layout'],
    queryFn: async () => unwrap(await api.GET('/api/v1/dashboard/layout')),
    refetchInterval: layoutInterval,
  })
  const layout = layoutQuery.data ?? defaultLayout
  const known = {
    topLeft: knownKeys(layout.topLeft),
    topRight: knownKeys(layout.topRight),
    middle: knownKeys(layout.middle),
    bottomLeft: knownKeys(layout.bottomLeft),
    bottomRight: knownKeys(layout.bottomRight),
  } satisfies Record<RegionName, string[]>

  // Not loaded yet or failed: rotation 0.
  const rotation = normalizeRotation(kioskState.data?.rotation)
  const browserMode = searchParams.get('rotation') === 'browser'
  const browser = useBrowserRotation(browserMode ? rotation : 0)

  useEffect(() => {
    document.body.classList.add('mirror-kiosk')
    return () => document.body.classList.remove('mirror-kiosk')
  }, [])

  const page = (
    <main className="mirror-dashboard">
      <h1 className="visually-hidden">Spiegel</h1>
      {(known.topLeft.length > 0 || known.topRight.length > 0) && (
        <header className="mirror-header">
          <Region name="topLeft" keys={known.topLeft} />
          <Region name="topRight" keys={known.topRight} />
        </header>
      )}
      <Region name="middle" keys={known.middle} />
      <Region name="bottomLeft" keys={known.bottomLeft} />
      <Region name="bottomRight" keys={known.bottomRight} />
      {(kioskState.data?.testPattern || searchParams.has('diagnose')) && (
        <DiagnoseOverlay
          size={browserMode ? browser.size : undefined}
          rotation={{ mode: browserMode ? 'browser' : 'monitor', degrees: rotation }}
        />
      )}
    </main>
  )
  // Only an explicit false switches off: a failed request or an older server
  // without the field must never black out the mirror.
  const blackout = kioskState.data?.powerOn === false
  return (
    <>
      {browserMode ? <RotatedViewport style={browser.style}>{page}</RotatedViewport> : page}
      {/* Outside the rotated wrapper, so it covers the whole window whatever the rotation. */}
      {blackout && <div className="mirror-blackout" data-testid="blackout" aria-hidden />}
    </>
  )
}
