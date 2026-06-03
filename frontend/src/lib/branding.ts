// Centralised branding config. Every visual brand reference in the app reads
// from here. Override any value via VITE_ environment variables at runtime or
// build time.

import { getConfig } from './runtimeConfig';

function env(key: string, fallback: string): string {
  return getConfig(key, fallback);
}

export const branding = {
  // Identity
  name: env('VITE_BRAND_NAME', 'Gateway Explorer'),
  company: env('VITE_BRAND_COMPANY', 'Gateway.fm AS'),
  description: env('VITE_BRAND_DESCRIPTION', 'Block Explorer - Browse blocks, transactions, and accounts on the blockchain'),

  // Assets — paths relative to /public or absolute URLs
  logo: env('VITE_BRAND_LOGO', '/logo.svg'),
  icon: env('VITE_BRAND_ICON', '/mascot.png'),
  favicon: env('VITE_BRAND_FAVICON', ''),  // defaults to icon if empty

  // Primary brand color
  colorPrimary: env('VITE_BRAND_COLOR_PRIMARY', '#8950FA'),

  // Hero/search bar background gradient — overrides the default primary gradient
  colorHeroBg: env('VITE_BRAND_COLOR_HERO_BG', ''),

  // Links
  website: env('VITE_BRAND_WEBSITE', 'https://gateway.fm/'),
  docs: env('VITE_BRAND_DOCS', 'https://docs.gateway.fm/'),
  legal: env('VITE_BRAND_LEGAL', 'https://gateway.fm/legal/'),
  security: env('VITE_BRAND_SECURITY', 'https://gateway.fm/security/'),
  trust: env('VITE_BRAND_TRUST', 'https://trust.gateway.fm'),

  // Social
  twitter: env('VITE_BRAND_TWITTER', 'https://x.com/gateway_eth'),
  linkedin: env('VITE_BRAND_LINKEDIN', 'https://www.linkedin.com/company/gateway-fm/'),
  discord: env('VITE_BRAND_DISCORD', 'https://discord.gg/grPXnEbAyv'),
  telegram: env('VITE_BRAND_TELEGRAM', 'https://t.me/gateway_fm'),
} as const;

// Derived helpers
export function getFavicon(): string {
  return branding.favicon || branding.icon;
}

// Extract short company name from full name (e.g. "Gateway Explorer" -> "Gateway")
export function getShortName(): string {
  return branding.name.split(' ')[0];
}
