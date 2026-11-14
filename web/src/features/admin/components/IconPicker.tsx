import { useMemo } from 'react'
import { Group, Select, type ComboboxItem, type ComboboxItemGroup } from '@mantine/core'
import { useQuery } from '@tanstack/react-query'
import 'bootstrap-icons/font/bootstrap-icons.css'
import { MaskIcon } from '../../../shared/MaskIcon.tsx'
import { fetchIcons, iconFileUrl, iconsQueryKey, type IconRef } from '../api/icons.ts'
import { BOOTSTRAP_ICONS } from './bootstrapIcons.ts'

const NONE = 'none'

const encode = (ref: IconRef): string =>
  ref.kind === 'bootstrap' && ref.name ? `bootstrap:${ref.name}` : ref.kind === 'upload' && ref.file ? `upload:${ref.file}` : NONE

function decode(value: string): IconRef {
  if (value.startsWith('bootstrap:')) return { kind: 'bootstrap', name: value.slice('bootstrap:'.length) }
  if (value.startsWith('upload:')) return { kind: 'upload', file: value.slice('upload:'.length) }
  return { kind: 'none' }
}

function Preview({ value, svgIds }: { value: string; svgIds: ReadonlySet<string> }) {
  if (value.startsWith('bootstrap:')) return <i className={`bi bi-${value.slice('bootstrap:'.length)}`} aria-hidden />
  if (value.startsWith('upload:')) {
    const id = value.slice('upload:'.length)
    if (svgIds.has(id)) return <MaskIcon src={iconFileUrl(id)} size={18} />
    return <img src={iconFileUrl(id)} alt="" width={18} height={18} style={{ objectFit: 'contain' }} />
  }
  return null
}

type Props = {
  label: string
  value: IconRef
  onChange: (value: IconRef) => void
  error?: string
}

/** Searchable symbol chooser: own uploads, a curated Bootstrap set, or none. */
export function IconPicker({ label, value, onChange, error }: Props) {
  const uploads = useQuery({ queryKey: iconsQueryKey, queryFn: fetchIcons })
  const current = encode(value)
  // Ids of SVG uploads; the reference itself may carry the type when the list is not loaded yet.
  const svgIds = useMemo(() => {
    const ids = new Set((uploads.data ?? []).filter((i) => i.contentType === 'image/svg+xml').map((i) => i.id))
    if (value.kind === 'upload' && value.file && value.contentType === 'image/svg+xml') ids.add(value.file)
    return ids
  }, [uploads.data, value])

  const data = useMemo<Array<ComboboxItem | ComboboxItemGroup<ComboboxItem>>>(() => {
    const own: ComboboxItem[] = (uploads.data ?? []).map((i) => ({ value: `upload:${i.id}`, label: i.name }))
    const standard: ComboboxItem[] = BOOTSTRAP_ICONS.map(([name, text]) => ({ value: `bootstrap:${name}`, label: text }))
    // A stored reference outside the lists (deleted upload, other Bootstrap name) stays selectable.
    const known = new Set([...own, ...standard].map((o) => o.value))
    if (current !== NONE && !known.has(current)) {
      const extra = { value: current, label: current.replace(/^[a-z]+:/, '') }
      if (current.startsWith('upload:')) own.push(extra)
      else standard.push(extra)
    }
    const groups: Array<ComboboxItem | ComboboxItemGroup<ComboboxItem>> = [{ value: NONE, label: 'Kein Symbol' }]
    if (own.length > 0) groups.push({ group: 'Eigene Symbole', items: own })
    groups.push({ group: 'Standard', items: standard })
    return groups
  }, [uploads.data, current])

  return (
    <Select
      label={label}
      data={data}
      value={current}
      onChange={(v) => onChange(decode(v ?? NONE))}
      searchable
      allowDeselect={false}
      nothingFoundMessage="Kein Symbol gefunden"
      leftSection={<Preview value={current} svgIds={svgIds} />}
      renderOption={({ option }) => (
        <Group gap="xs" wrap="nowrap">
          <span style={{ display: 'inline-flex', width: 20, justifyContent: 'center' }}>
            <Preview value={option.value} svgIds={svgIds} />
          </span>
          <span>{option.label}</span>
        </Group>
      )}
      error={error}
    />
  )
}
