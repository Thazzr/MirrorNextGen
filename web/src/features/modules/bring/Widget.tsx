import { useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { BringItem } from './api.ts'
import { dashboardBringQueryKey, fetchDashboardBring } from './api.ts'
import { FitName } from './FitName.tsx'
import './bring.css'

export const columns = 6
/** Three rows of six: all the mirror has room for, however many the server delivers. */
const maxVisible = 3 * columns

/** A name that is nothing but one emoji (with an optional variation selector) is shown as that emoji. */
function isEmojiOnly(value: string) {
  return /^\p{Extended_Pictographic}️?$/u.test(value.trim())
}

/** One tile. The emoji is shown as the symbol when no picture is, and then needs no second appearance as the name. */
function BringTile({ item }: { item: BringItem }) {
  const [failed, setFailed] = useState(false)
  const name = item.name.trim()
  const hasPicture = Boolean(item.iconUrl) && !failed
  const emojiSymbol = !hasPicture && isEmojiOnly(name)
  let icon: ReactNode
  if (hasPicture) icon = <img className="mirror-bring__icon" src={item.iconUrl} alt="" onError={() => setFailed(true)} />
  else if (emojiSymbol) icon = <span className="mirror-bring__emoji">{name}</span>
  else icon = <span className="mirror-bring__fallback">{Array.from(name)[0]?.toLocaleUpperCase('de-DE')}</span>
  return (
    <li className="mirror-bring__item">
      <div className="mirror-bring__icon-slot">{icon}</div>
      <div className="mirror-bring__label">
        {!emojiSymbol && <FitName name={item.name} />}
        {item.specification && <span className="mirror-bring__specification">{item.specification}</span>}
      </div>
    </li>
  )
}

export function BringWidget() {
  const query = useQuery({
    queryKey: dashboardBringQueryKey,
    queryFn: fetchDashboardBring,
    refetchInterval: 60_000,
  })
  const bring = query.data
  if (!bring) {
    // Still loading shows nothing; a failed request says so where the list would be.
    return query.isError ? (
      <aside className="mirror-bring" aria-label="Einkaufsliste">
        <div className="mirror-error">Die Bring!-Liste konnte nicht geladen werden.</div>
      </aside>
    ) : null
  }
  if (!bring.configured) return null
  if (bring.error) {
    return (
      <aside className="mirror-bring" aria-label="Einkaufsliste">
        <div className="mirror-error">{bring.error}</div>
      </aside>
    )
  }
  const items = bring.items.slice(0, maxVisible)
  if (items.length === 0) return null
  const rows = Array.from({ length: Math.ceil(items.length / columns) }, (_, row) => items.slice(row * columns, (row + 1) * columns))
  return (
    <aside className="mirror-bring" aria-label="Einkaufsliste">
      <div className="mirror-bring__rows">
        {rows.map((row, rowIndex) => (
          <ul className="mirror-bring__list" key={rowIndex}>
            {row.map((item) => (
              <BringTile key={`${item.name}\u001f${item.specification ?? ''}`} item={item} />
            ))}
          </ul>
        ))}
      </div>
    </aside>
  )
}
