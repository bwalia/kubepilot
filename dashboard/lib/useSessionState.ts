import { useCallback, useEffect, useState } from "react";

/**
 * String state persisted to sessionStorage: it survives page refreshes but is
 * cleared automatically when the tab/window closes (or when the user resets it).
 * Used e.g. for the dashboard namespace selection so a refresh doesn't lose it.
 * SSR-safe: starts from `initial`, then hydrates from storage after mount.
 */
export function useSessionState(
  key: string,
  initial: string
): [string, (value: string) => void] {
  const [value, setValue] = useState(initial);

  useEffect(() => {
    try {
      const stored = sessionStorage.getItem(key);
      if (stored !== null) setValue(stored);
    } catch {
      /* storage unavailable — keep in-memory value */
    }
  }, [key]);

  const set = useCallback(
    (next: string) => {
      setValue(next);
      try {
        sessionStorage.setItem(key, next);
      } catch {
        /* ignore */
      }
    },
    [key]
  );

  return [value, set];
}
