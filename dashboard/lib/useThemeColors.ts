import { useEffect, useState } from "react";

/**
 * Resolve theme palette tokens to concrete `rgb(r g b)` strings for use where a
 * CSS class won't reach — SVG presentation attributes (fill/stroke) and canvas.
 * Re-reads whenever the active theme flips (watches <html data-theme>), so
 * charts and instrument graphics recolour instantly with the Daylight/Night
 * toggle instead of staying frozen on their initial palette.
 *
 * Pass token names without the `--p-` prefix, e.g. useThemeColors(["success",
 * "surface-2", "border"]).
 */
export function useThemeColors(names: string[]): Record<string, string> {
  const key = names.join(",");
  const [colors, setColors] = useState<Record<string, string>>({});

  useEffect(() => {
    const read = () => {
      const cs = getComputedStyle(document.documentElement);
      const out: Record<string, string> = {};
      for (const n of names) {
        const raw = cs.getPropertyValue(`--p-${n}`).trim();
        out[n] = raw ? `rgb(${raw})` : "";
      }
      setColors(out);
    };
    read();
    const obs = new MutationObserver(read);
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    return () => obs.disconnect();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  return colors;
}
