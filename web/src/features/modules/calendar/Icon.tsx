import { MaskIcon } from '../../../shared/MaskIcon.tsx'
import type { components } from '../../../shared/api/schema.gen.ts'

type IconRef = components['schemas']['IconRef']

/** Bootstrap icon of an event without an own symbol. */
const defaultIcon = 'calendar-event'

/** One calendar symbol: a Bootstrap icon (loaded globally), an uploaded image, or the default. An uploaded SVG is a mask in the text colour (white); a PNG stays an image. */
export function CalendarIcon({ icon }: { icon: IconRef }) {
  if (icon.kind === 'upload' && icon.file) {
    const src = `/api/v1/icons/${encodeURIComponent(icon.file)}/file`
    if (icon.contentType === 'image/svg+xml') return <MaskIcon className="mirror-agenda__icon mirror-agenda__icon--mask" src={src} />
    return <img className="mirror-agenda__icon" src={src} alt="" />
  }
  const name = icon.kind === 'bootstrap' && icon.name ? icon.name : defaultIcon
  return <i className={`bi bi-${name}`} aria-hidden="true" />
}
