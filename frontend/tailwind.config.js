/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ["class"],
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    screens: {
      'xs': '475px',
      'sm': '640px',
      'md': '768px',
      'lg': '1024px',
      'xl': '1280px',
      '2xl': '1536px',
    },
    extend: {
      borderRadius: {
        'lg': '12px',
        'xl': '16px',
        '2xl': '24px',
      },
      colors: {
        // Gateway Primary Purple
        primary: {
          50: '#F5F3FF',
          100: '#EDE9FE',
          200: '#C4A8FD',
          300: '#A478FC',
          400: '#8950FA',
          500: '#8950FA',
          600: '#6B3DD4',
          700: '#5B32B0',
          800: '#4C2889',
          900: '#3D1F6D',
          DEFAULT: '#8950FA',
        },
        // Neutrals for light theme
        neutral: {
          50: '#FFFFFF',
          100: '#F1F5F9',
          200: '#E2E8F0',
          300: '#CBD5E1',
          400: '#94A3B8',
          500: '#6B7280',
          600: '#475569',
          700: '#374151',
          800: '#1A1A1A',
          900: '#0F0F0F',
        },
        // Status colors
        success: {
          50: '#DCFCE7',
          100: '#BBF7D0',
          500: '#22C55E',
          600: '#16A34A',
          700: '#166534',
        },
        warning: {
          50: '#FEF9C3',
          100: '#FEF08A',
          500: '#EAB308',
          600: '#CA8A04',
          700: '#854D0E',
        },
        error: {
          50: '#FEE2E2',
          100: '#FECACA',
          500: '#EF4444',
          600: '#DC2626',
          700: '#991B1B',
        },
      },
      boxShadow: {
        'card': '0 1px 3px rgba(0, 0, 0, 0.05), 0 1px 2px rgba(0, 0, 0, 0.1)',
        'card-hover': '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
        'elevated': '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)',
        'primary': '0 0 20px rgba(137, 80, 250, 0.3)',
        'primary-lg': '0 0 30px rgba(137, 80, 250, 0.4)',
      },
      animation: {
        'fade-in': 'fade-in 0.3s ease-out',
        'fade-in-up': 'fade-in-up 0.4s ease-out',
        'scale-in': 'scale-in 0.2s ease-out',
      },
      keyframes: {
        'fade-in': {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        'fade-in-up': {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        'scale-in': {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'Avenir', 'Helvetica', 'Arial', 'sans-serif'],
        mono: ['JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', 'monospace'],
      },
    }
  },
  plugins: [
    require("tailwindcss-animate"),
  ],
}
