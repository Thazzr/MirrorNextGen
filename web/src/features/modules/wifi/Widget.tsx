import { useQuery } from '@tanstack/react-query'
import { api, unwrap } from '../../../shared/api/client.ts'
import './wifi.css'

const quietZone = 4

export function WifiWidget() {
  // No error text: a QR code with an error message is useless on the mirror, so a failure shows nothing.
  const query = useQuery({
    queryKey: ['dashboard', 'wifi'],
    queryFn: async () => unwrap(await api.GET('/api/v1/dashboard/wifi')),
    refetchInterval: 5 * 60_000,
  })
  const qr = query.data?.configured ? query.data.qr : undefined
  if (!qr) return null
  const total = qr.size + 2 * quietZone
  // One path of unit squares for all dark modules; the black background is the light part.
  const path = qr.rows
    .flatMap((row, y) => Array.from(row, (cell, x) => (cell === '1' ? `M${x + quietZone} ${y + quietZone}h1v1h-1z` : '')))
    .join('')
  return (
    <aside className="mirror-wifi" aria-label="WLAN QR-Code">
      <svg className="mirror-wifi__code" viewBox={`0 0 ${total} ${total}`} role="img" aria-label={`WLAN ${query.data?.ssid ?? ''}`}>
        <rect width={total} height={total} fill="#000" />
        <path d={path} fill="#fff" />
      </svg>
    </aside>
  )
}
