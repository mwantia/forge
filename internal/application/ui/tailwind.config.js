/** @type {import('tailwindcss').Config} */
export const content = [
  './templates/**/*.templ',
  './templates/**/*.go',
];
export const theme = {
  extend: {
    colors: {
      bg: '#0e0f13',
      'bg-1': '#111318',
      'bg-2': '#15181f',
      'bg-3': '#1b1f27',
      panel: '#14171d',
      'panel-2': '#181c23',
      line: '#262a33',
      'line-soft': '#1d2027',
      'line-strong': '#343844',
      ink: '#e6e7ea',
      'ink-2': '#b9bcc4',
      'ink-3': '#7e828c',
      'ink-4': '#565963',
      accent: '#e8a531',
      'accent-soft': 'rgba(232,165,49,0.12)',
      'accent-line': 'rgba(232,165,49,0.32)',
      'accent-ai': '#a48afa',
      'accent-ai-soft': 'rgba(164,138,250,0.12)',
      'accent-ai-line': 'rgba(164,138,250,0.30)',
      ok: '#6dc97a',
      add: '#7eb86a',
      'add-soft': 'rgba(126,184,106,0.10)',
      rem: '#d27a78',
      'rem-soft': 'rgba(210,122,120,0.10)',
    },
    fontFamily: {
      sans: ['Inter', 'system-ui', 'sans-serif'],
      mono: ['"JetBrains Mono"', '"SFMono-Regular"', 'Menlo', 'monospace'],
    },
    borderRadius: {
      '2px': '2px',
      '3px': '3px',
    },
    boxShadow: {
      'accent-glow': '0 0 6px rgba(232,165,49,0.55)',
    },
    fontSize: {
      // Legacy alias kept for any stale references.
      '2xs':     ['11px', '15px'],
      // Forge UI scale — use these instead of arbitrary text-[Npx] values.
      'ui-dot':  ['13px',   { lineHeight: '13px' }], // collapse arrows ▶
      'ui-label':['12px',   { lineHeight: '16px' }], // section caps, badge letters, meta keys
      'ui-args': ['12px', { lineHeight: '16px' }], // JSON / args pre blocks
      'ui-meta': ['12px',   { lineHeight: '17px' }], // sidebar body, hashes, loading text
      'ui-sys':  ['13px', { lineHeight: '18px' }], // system-message pre content
      'ui-code': ['13px',   { lineHeight: '17px' }], // inputs, inline code, raw toggle
      'ui-ctrl': ['14px',   { lineHeight: '19px' }], // buttons, action controls
      'ui-body': ['15px', { lineHeight: '21px' }], // message content, textarea
      'ui-h1':   ['23px',   { lineHeight: '29px' }], // page title
    },
  },
};
export const safelist = [
  'text-rem', 'border-rem/30',
  'text-ok', 'border-ok/30',
  'text-ink-4', 'border-line-soft',
];
export const plugins = [];
