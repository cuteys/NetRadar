/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        macaron: {
          mint: {
            DEFAULT: '#4ade80',
            soft: '#86efac',
            deep: '#22c55e',
            bg: '#f0fdf4',
          },
          lavender: {
            DEFAULT: '#a78bfa',
            soft: '#c4b5fd',
            deep: '#8b5cf6',
            bg: '#f5f3ff',
          },
          peach: {
            DEFAULT: '#fca5a5',
            soft: '#fecaca',
            deep: '#ef4444',
            bg: '#fef2f2',
          },
          blue: {
            DEFAULT: '#93c5fd',
            soft: '#bfdbfe',
            deep: '#3b82f6',
            bg: '#eff6ff',
          },
          butter: {
            DEFAULT: '#fde68a',
            soft: '#fef08a',
            deep: '#f59e0b',
            bg: '#fffbeb',
          },
          rose: {
            DEFAULT: '#f472b6',
            soft: '#fbcfe8',
            deep: '#ec4899',
            bg: '#fdf2f8',
          },
        },
      },
      fontFamily: {
        apple: [
          '-apple-system',
          'BlinkMacSystemFont',
          '"SF Pro Text"',
          '"SF Pro Icons"',
          '"Helvetica Neue"',
          'Helvetica',
          'Arial',
          'sans-serif',
        ],
      },
      boxShadow: {
        'apple-subtle': '0 4px 20px -2px rgba(0, 0, 0, 0.05)',
        'apple-card': '0 10px 30px -5px rgba(0, 0, 0, 0.04), 0 4px 6px -2px rgba(0, 0, 0, 0.02)',
        'apple-glow': '0 0 25px -5px rgba(167, 139, 250, 0.3)',
      },
    },
  },
  plugins: [],
}
