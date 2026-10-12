import { useEffect, useState } from 'react';

// Re-renders every `intervalMs` so countdowns stay live.
const useNow = (intervalMs = 1000) => {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);

  return now;
};

export default useNow;
