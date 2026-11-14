import type { components } from '../../../shared/api/schema.gen.ts'

type IconRef = components['schemas']['IconRef']

/** Bootstrap icon of an event without an own symbol. */
const defaultIcon = 'calendar-event'

/** One calendar symbol: a Bootstrap icon (loaded globally), an uploaded image, or the default. Uploads stay unfiltered. */
export function CalendarIcon({ icon }: { icon: IconRef }) {
  if (icon.kind === 'upload' && icon.file) {
    return <img className="mirror-agenda__icon" src={`/api/v1/icons/${encodeURIComponent(icon.file)}/file`} alt="" />
  }
  const name = icon.kind === 'bootstrap' && icon.name ? icon.name : defaultIcon
  return <i className={`bi bi-${name}`} aria-hidden="true" />
}
