/**
 * Redirect the browser to the login page via the backend,
 * which will redirect to privacy-proxy's OAuth authorize endpoint.
 */
export function redirectToLogin(returnUrl?: string): void {
  const url = returnUrl || window.location.pathname + window.location.search;
  window.location.href = `/api/auth/login?return_url=${encodeURIComponent(url)}`;
}
