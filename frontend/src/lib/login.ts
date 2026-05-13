import { getConfig } from './runtimeConfig';

/**
 * Redirect the browser to the login page via the backend,
 * which will redirect to privacy-proxy's OAuth authorize endpoint.
 * Uses VITE_API_URL so it works behind a reverse-proxy path prefix
 * (e.g. /proxy/18201/api/auth/login instead of /api/auth/login).
 *
 * When running inside an iframe (e.g. embedded in the GasStorm dashboard),
 * navigates the top-level window so the OAuth redirect chain works correctly —
 * the privacy UI SPA can't run inside a proxied iframe path prefix.
 */
export function redirectToLogin(returnUrl?: string): void {
  const apiBase = getConfig('VITE_API_URL', '/api');
  const basePath = getConfig('VITE_BASE_PATH', '');
  const inIframe = window.self !== window.top;

  let url: string;
  if (returnUrl) {
    url = returnUrl;
  } else if (inIframe && window.top) {
    // Try to read the parent URL so the callback redirects back to the
    // main server, not the explorer where the SSO flow happens. When the
    // parent is cross-origin (e.g. gasstorm dashboard at :18000 framing
    // the explorer at :18201) this throws SecurityError — fall back to
    // the iframe's own path. The backend strips absolute URLs anyway as
    // an open-redirect guard, so this is best-effort.
    try {
      url = window.top.location.href;
    } catch {
      url = basePath || window.location.pathname + window.location.search;
    }
  } else {
    url = basePath || window.location.pathname + window.location.search;
  }
  const loginUrl = `${apiBase}/auth/login?return_url=${encodeURIComponent(url)}`;

  // If we're in an iframe, navigate the top-level window so the full
  // OAuth redirect chain (explorer → privacy proxy → privacy UI → callback)
  // works without SPA routing issues inside the iframe.
  const target = inIframe ? window.top : window;
  if (target) {
    target.location.href = loginUrl;
  } else {
    window.location.href = loginUrl;
  }
}
