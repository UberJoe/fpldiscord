// usePoll drives a single /api/* endpoint: fetch on mount, then re-fetch on the
// server-provided cadence (meta.pollAfterMs — ~20s while a match is live, ~60s
// idle). The pre-first-snapshot 503 schedules a short retry from its
// retryAfterMs. Last good data is kept on screen while a later fetch fails.

import { useEffect, useRef, useState } from "react";
import { Envelope, StartingUpError } from "./api";

export interface PollState<T> {
  envelope: Envelope<T> | null;
  error: Error | null;
  loading: boolean;
}

export function usePoll<T>(fetcher: () => Promise<Envelope<T>>): PollState<T> {
  const [state, setState] = useState<PollState<T>>({
    envelope: null,
    error: null,
    loading: true,
  });
  // Keep the latest fetcher without making it a re-subscribe trigger.
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  useEffect(() => {
    let cancelled = false;
    let timer: number | undefined;

    const tick = async () => {
      try {
        const envelope = await fetcherRef.current();
        if (cancelled) return;
        setState({ envelope, error: null, loading: false });
        timer = window.setTimeout(tick, envelope.meta.pollAfterMs);
      } catch (err) {
        if (cancelled) return;
        const error = err instanceof Error ? err : new Error(String(err));
        setState((prev) => ({ ...prev, error, loading: false }));
        const retry =
          error instanceof StartingUpError ? error.retryAfterMs : 15000;
        timer = window.setTimeout(tick, retry);
      }
    };

    void tick();
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, []);

  return state;
}
