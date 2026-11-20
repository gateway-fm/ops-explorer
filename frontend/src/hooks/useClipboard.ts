import { useCallback, useEffect, useRef, useState } from 'react';

// Feedback belongs to this exact displayed value. Late clipboard completions
// cannot mark a replacement address/account as copied.
export function useClipboard(text: string, duration = 1500) {
  type Status = 'idle' | 'pending' | 'copied' | 'failed';
  const [feedback, setFeedback] = useState<{text: string; status: Status}>({text, status: 'idle'});
  const status = feedback.text === text ? feedback.status : 'idle';
  const setStatus = (next: Status) => setFeedback({text, status: next});
  const generation = useRef(0), timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const retire = useCallback(() => { ++generation.current; clearTimeout(timer.current); }, []);
  useEffect(() => { retire(); return retire; }, [text, retire]);
  const copy = async () => {
    const owned = ++generation.current;
    clearTimeout(timer.current); setStatus('pending');
    try {
      if (!navigator.clipboard?.writeText) throw Error('Clipboard unavailable');
      await navigator.clipboard.writeText(text);
      if (owned !== generation.current) return;
      setStatus('copied');
      timer.current = setTimeout(() => { if (owned === generation.current) setStatus('idle'); }, duration);
    } catch {
      if (owned === generation.current) setStatus('failed');
    }
  };
  return { status, copy };
}
