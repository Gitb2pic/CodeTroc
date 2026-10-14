import { useCallback, useEffect, useRef, useState } from 'react';

export interface Resource<T> {
  data: T | undefined;
  loading: boolean;
  error: Error | undefined;
  reload(): void;
}

/** Charge une ressource et la recharge quand `deps` change ; ignore les réponses périmées. */
export function useResource<T>(load: () => Promise<T>, deps: unknown[]): Resource<T> {
  const [state, setState] = useState<{ data?: T; loading: boolean; error?: Error }>({ loading: true });
  const [tick, setTick] = useState(0);
  const seq = useRef(0);
  const run = useCallback(load, deps);

  useEffect(() => {
    const n = ++seq.current;
    setState((s) => ({ data: s.data, loading: true }));
    run().then(
      (data) => n === seq.current && setState({ data, loading: false }),
      (error: Error) => n === seq.current && setState({ loading: false, error }),
    );
  }, [run, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  return { data: state.data, loading: state.loading, error: state.error, reload };
}
