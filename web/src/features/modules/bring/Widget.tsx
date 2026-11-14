import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { BringItem } from './api.ts'
import { dashboardBringQueryKey, fetchDashboardBring } from './api.ts'
import './bring.css'

export const columns = 6
/** Three rows of six: all the mirror has room for, however many the server delivers. */
const maxVisible = 3 * columns

/** A name that is nothing but one emoji (with an optional variation selector) is shown as that emoji. */
function isEmojiOnly(value: string) {
  return /^\p{Extended_Pictographic}️?$/u.test(value.trim())
}

function BringIcon({ item }: { item: BringItem }) {
  const [failed, setFailed] = useState(false)
  if (item.iconUrl && !failed) {
    return <img className="mirror-bring__icon" src={item.iconUrl} alt="" onError={() => setFailed(true)} />
  }
  const name = item.name.trim()
  if (isEmojiOnly(name)) return <span className="mirror-bring__emoji">{name}</span>
  return <span className="mirror-bring__fallback">{Array.from(name)[0]?.toLocaleUpperCase('de-DE')}</span>
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
              <li key={`${item.name}\u001f${item.specification ?? ''}`} className="mirror-bring__item">
                <div className="mirror-bring__icon-slot">
                  <BringIcon item={item} />
                </div>
                <div className="mirror-bring__label">
                  <span className="mirror-bring__name">{item.name}</span>
                  {item.specification && <span className="mirror-bring__specification">{item.specification}</span>}
                </div>
              </li>
            ))}
          </ul>
        ))}
      </div>
    </aside>
  )
}
