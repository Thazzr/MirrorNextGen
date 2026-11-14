import { useQuery } from '@tanstack/react-query'
import { fetchIcons, iconsQueryKey } from './icons.ts'

export function useIconsTileStatus() {
  const query = useQuery({ queryKey: iconsQueryKey, queryFn: fetchIcons })
  if (query.isPending) return { loading: true as const }
  if (query.isError) return { loading: false as const, text: 'Status nicht verfügbar' }
  const n = query.data.length
  return { loading: false as const, text: n === 0 ? 'Keine eigenen Symbole' : `${n} eigene${n === 1 ? 's' : ''} Symbol${n === 1 ? '' : 'e'}` }
}
