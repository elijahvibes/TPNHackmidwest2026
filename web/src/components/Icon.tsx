import type { SVGProps } from 'react'

const PATHS = {
  pin: 'M12 21s-6-5.2-6-10a6 6 0 1 1 12 0c0 4.8-6 10-6 10Z M12 13a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z',
  calendar: 'M7 3v3M17 3v3M4 9h16M6 5h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2Z',
  users: 'M16 19v-1.5a3.5 3.5 0 0 0-3.5-3.5h-5A3.5 3.5 0 0 0 4 17.5V19M10 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7ZM20 19v-1.5a3.5 3.5 0 0 0-2.5-3.4M15 4.2a3.5 3.5 0 0 1 0 6.6',
  wallet: 'M3 7a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v1M3 7v10a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2H5a2 2 0 0 1-2-2Zm13 7h.01',
  check: 'm5 12 4.5 4.5L19 7',
  arrow: 'M5 12h14M13 6l6 6-6 6',
  chevron: 'm6 9 6 6 6-6',
  pencil: 'M4 20h4l10.5-10.5a2.1 2.1 0 0 0-3-3L5 17v3ZM13.5 6.5l3 3',
  car: 'M5 13 6.5 8.5A2 2 0 0 1 8.4 7h7.2a2 2 0 0 1 1.9 1.5L19 13M5 13h14v4H5v-4Zm0 4v2m14-2v2M8 15h.01M16 15h.01',
  plane: 'M10.5 13.5 4 11l1-1.5 7 1 5-5a1.4 1.4 0 0 1 2 2l-5 5 1 7-1.5 1-2.5-6.5-3 3V20l-1.5 1-1-3.5L3 16.5 4 15h3Z',
  print: 'M7 9V4h10v5M7 18H5a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2M7 14h10v6H7v-6Z',
  share: 'M12 4v12M8 8l4-4 4 4M5 14v4a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4',
  swap: 'M4 8h13l-3-3M20 16H7l3 3',
  minus: 'M6 12h12',
  plus: 'M12 6v12M6 12h12',
  locate: 'M12 3v3m0 12v3M3 12h3m12 0h3M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z',
  sun: 'M12 17a5 5 0 1 0 0-10 5 5 0 0 0 0 10Zm0-14v2m0 14v2M3 12h2m14 0h2M5.6 5.6l1.4 1.4m10-1.4-1.4 1.4m0 10 1.4 1.4m-10-1.4-1.4 1.4',
  cloud: 'M7 18a4 4 0 0 1-.5-8 5.5 5.5 0 0 1 10.6 1.5A3.3 3.3 0 0 1 17 18H7Z',
  rain: 'M7 15a4 4 0 0 1-.5-8 5.5 5.5 0 0 1 10.6 1.5A3.3 3.3 0 0 1 17 15H7Zm2 3-1 3m5-3-1 3m5-3-1 3',
  snow: 'M7 15a4 4 0 0 1-.5-8 5.5 5.5 0 0 1 10.6 1.5A3.3 3.3 0 0 1 17 15H7Zm1 4h.01M12 19h.01M16 19h.01M10 22h.01M14 22h.01',
  storm: 'M7 15a4 4 0 0 1-.5-8 5.5 5.5 0 0 1 10.6 1.5A3.3 3.3 0 0 1 17 15H7Zm6-1-2.5 4H13l-2 4',
  wind: 'M4 9h10a2.5 2.5 0 1 0-2.5-2.5M4 14h13a2.5 2.5 0 1 1-2.5 2.5M4 19h7',
  fog: 'M7 13a4 4 0 0 1-.5-8 5.5 5.5 0 0 1 10.6 1.5A3.3 3.3 0 0 1 17 13H7Zm-2 4h14M7 21h10',
  star: 'm12 3 2.7 5.6 6.1.9-4.4 4.3 1 6.1L12 17l-5.4 2.9 1-6.1L3.2 9.5l6.1-.9L12 3Z',
  flag: 'M5 21V4m0 0h12l-2 4 2 4H5',
  bed: 'M3 18v-8a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v8M3 15h18M7 8V6h4v2m2 0V6h4v2',
  fork: 'M7 3v7a3 3 0 0 0 3 3v8M7 3v4M10 3v4M17 3c-2 1-3 3-3 6v3h3v9',
} as const

export type IconName = keyof typeof PATHS

/** Line icons drawn to match the design: 1.5px strokes, rounded caps. */
export default function Icon({ name, size = 16, ...rest }: { name: IconName; size?: number } & SVGProps<SVGSVGElement>) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5}
      strokeLinecap="round" strokeLinejoin="round" aria-hidden {...rest}>
      <path d={PATHS[name]} />
    </svg>
  )
}

/** Weather glyph for a forecast icon key ("sun", "rain", ...). */
export function WeatherIcon({ icon, size = 18, className }: { icon: string; size?: number; className?: string }) {
  const name = (icon in PATHS ? icon : 'sun') as IconName
  return <Icon name={name} size={size} className={className} />
}
