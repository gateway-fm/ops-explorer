import { Navigate } from 'react-router-dom';

/** Redirect /shared-logs to /privacy (Shared Logs is now a tab there). */
export function SharedLogs() {
  return <Navigate to="/privacy" replace />;
}
