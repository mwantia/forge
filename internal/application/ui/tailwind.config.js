/** @type {import('tailwindcss').Config} */
export const content = [
  './templates/**/*.templ',
  './templates/**/*.go',
];
export const theme = {
  extend: {
    colors: {
      bg: '#0b0c0f',
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
      '2xs': ['10px', '14px'],
    },
  },
};
export const plugins = [];
