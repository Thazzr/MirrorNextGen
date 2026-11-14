import type { CSSProperties } from 'react'

/**
 * An uploaded SVG drawn as a mask over `currentColor`. An SVG inside an <img>
 * never inherits the page colour, so its `currentColor` would be black; as a
 * mask every opaque part takes the surrounding text colour instead, and no SVG
 * markup is injected into the page. No Mantine here: the dashboard uses it too.
 */
export function MaskIcon({ src, className, size }: { src: string; className?: string; size?: number }) {
  const image = `url("${src}")`
  const style: CSSProperties = {
    backgroundColor: 'currentColor',
    maskImage: image,
    WebkitMaskImage: image,
    maskSize: 'contain',
    WebkitMaskSize: 'contain',
    maskRepeat: 'no-repeat',
    WebkitMaskRepeat: 'no-repeat',
    maskPosition: 'center',
    WebkitMaskPosition: 'center',
    ...(size ? { display: 'inline-block', flex: 'none', width: size, height: size } : {}),
  }
  return <span className={className} style={style} aria-hidden="true" />
}
