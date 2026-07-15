import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";
const STORAGE_KEY = "kubepilot-theme";

/**
 * Theme controller. Daylight ("light") is the default; the operator can flip to
 * Night ("dark"), and the choice is remembered in localStorage and applied to
 * <html data-theme> (the pre-paint script in _document.tsx avoids a flash on
 * reload). Hydration-safe: the first client render mirrors the DOM attribute
 * that the pre-paint script already set.
 */
export function useTheme(): { theme: Theme; toggle: () => void; setTheme: (t: Theme) => void } {
  const [theme, setThemeState] = useState<Theme>("light");

  // Sync from the DOM once mounted (the no-flash script may have set "dark").
  useEffect(() => {
    const attr = document.documentElement.getAttribute("data-theme");
    setThemeState(attr === "dark" ? "dark" : "light");
  }, []);

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next);
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      /* private mode / storage disabled — theme still applies for this session */
    }
  }, []);

  const toggle = useCallback(() => {
    setTheme(document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark");
  }, [setTheme]);

  return { theme, toggle, setTheme };
}
