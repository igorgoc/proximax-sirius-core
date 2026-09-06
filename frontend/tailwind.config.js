/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        sirius: {
          50: '#f0f9ff',
          100: '#e0f2fe',
          200: '#b9e6fe',
          300: '#82faf1', // Official Bright Cyan
          400: '#44fff1', // Official Neon Cyan
          500: '#64abff', // Official Science Blue
          600: '#2563eb', 
          700: '#0049e3', // Official Deep Sirius Blue
          800: '#003ee3', // Official Tealish Blue
          900: '#0a1a4a',
          950: '#050c26',
        },
        space: {
          bg: '#080c14',
          surface: '#0d1322',
          card: '#121a2d',
          cardHover: '#18233c',
          border: '#1e2942',
          borderLight: '#2a3a5e',
        },
        btc: {
          bg: '#f4f6fa',
          panel: '#ffffff',
          border: '#d5dbe6',
          darkbg: '#0b0f19',
          darkpanel: '#111827',
          darkborder: '#1e2942',
          header: '#0f172a',
        }
      },
      fontFamily: {
        sans: ['Segoe UI', 'Ubuntu', 'Manrope', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'],
        mono: ['SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace'],
      },
      boxShadow: {
        'sirius-glow': '0 0 20px -5px rgba(68, 255, 241, 0.3)',
        'sirius-blue': '0 0 20px -5px rgba(0, 73, 227, 0.4)',
      }
    },
  },
  plugins: [],
}
