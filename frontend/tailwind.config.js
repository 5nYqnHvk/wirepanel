/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts,tsx,js}'],
  theme: {
    extend: {
      colors: {
        bg:      '#0b0f14',
        surface: '#111821',
        panel:   '#172230',
        border:  '#22303f',
        muted:   '#7d8d9f',
        accent:  '#3ad6c0',
        accent2: '#7c5cff',
        danger:  '#ff5d6c',
        warn:    '#ffb13c',
        ok:      '#3ad693'
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'monospace']
      }
    }
  },
  plugins: []
}
