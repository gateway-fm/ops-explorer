import { useState, useEffect } from 'react';
import { formatTimeAgo } from '../lib/utils';

export function LiveTimeAgo({ timestamp, className }: { timestamp: number; className?: string }) {
  const [, setTick] = useState(0);

  useEffect(() => {
    const interval = setInterval(() => {
      setTick(t => t + 1);
    }, 1000);
    return () => clearInterval(interval);
  }, []);

  return <span className={className}>{formatTimeAgo(timestamp)}</span>;
}
