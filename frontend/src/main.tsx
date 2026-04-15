import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { branding, getFavicon } from './lib/branding'

// Apply branding to document head
document.title = branding.name;

// Apply primary brand color to CSS variables
if (branding.colorPrimary && branding.colorPrimary !== '#8950FA') {
  const hex = branding.colorPrimary.replace('#', '');
  const r = parseInt(hex.substring(0, 2), 16);
  const g = parseInt(hex.substring(2, 4), 16);
  const b = parseInt(hex.substring(4, 6), 16);
  const root = document.documentElement.style;
  root.setProperty('--primary', branding.colorPrimary);
  root.setProperty('--primary-rgb', `${r}, ${g}, ${b}`);
  // Generate a darker hover variant (reduce brightness by ~20%)
  root.setProperty('--primary-hover', `#${Math.max(0, r - 30).toString(16).padStart(2, '0')}${Math.max(0, g - 30).toString(16).padStart(2, '0')}${Math.max(0, b - 30).toString(16).padStart(2, '0')}`);
  // Generate a light variant
  root.setProperty('--primary-light', `rgba(${r}, ${g}, ${b}, 0.05)`);
}

const favicon = getFavicon();
if (favicon) {
  let link = document.querySelector<HTMLLinkElement>("link[rel='icon']");
  if (!link) {
    link = document.createElement('link');
    link.rel = 'icon';
    document.head.appendChild(link);
  }
  link.type = favicon.endsWith('.svg') ? 'image/svg+xml' : 'image/png';
  link.href = favicon;
}

const metaDesc = document.querySelector<HTMLMetaElement>("meta[name='description']");
if (metaDesc && branding.description) {
  metaDesc.content = branding.description;
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
