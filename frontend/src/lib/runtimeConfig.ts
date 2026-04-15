// Runtime configuration - reads from window.__runtimeConfig (injected at container startup)
// with fallback to import.meta.env (Vite dev server).
//
// In production: entrypoint.sh generates /config.js with VITE_* env vars
// In development: Vite serves import.meta.env as usual

interface RuntimeConfig {
  [key: string]: string | undefined;
}

declare global {
  interface Window {
    __runtimeConfig?: RuntimeConfig;
  }
}

/**
 * Get a VITE_* configuration value.
 * Priority: window.__runtimeConfig (runtime) > import.meta.env (build-time) > fallback
 */
export function getConfig(key: string, fallback: string = ''): string {
  // Runtime config (from container entrypoint)
  const runtimeValue = window.__runtimeConfig?.[key];
  if (runtimeValue !== undefined && runtimeValue !== '') {
    return runtimeValue;
  }

  // Build-time config (from Vite)
  const buildValue = import.meta.env[key];
  if (buildValue !== undefined && buildValue !== '') {
    return String(buildValue);
  }

  return fallback;
}
