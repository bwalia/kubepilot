/**
 * ThemeToggle — flips the deck between Daylight (light) and Night (dark).
 * The icon + label show the CURRENT theme (moon = Night, sun = Daylight);
 * the tooltip/aria-label say which theme a click switches to. Choice is
 * persisted (see useTheme / _document no-flash script).
 */
import { Sun, Moon } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useTheme } from "@/lib/useTheme";

export function ThemeToggle() {
  const { theme, toggle } = useTheme();
  const reduce = useReducedMotion();
  const isDark = theme === "dark";
  const currentLabel = isDark ? "Night" : "Daylight";
  const nextLabel = isDark ? "Daylight" : "Night";

  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={`Switch to ${nextLabel} mode`}
      title={`Switch to ${nextLabel} mode`}
      className="inline-flex items-center gap-2 h-8 pl-2 pr-2.5 rounded-md border border-pilot-border bg-pilot-surface-2 text-pilot-text-secondary hover:text-pilot-text-primary hover:border-pilot-border-hover transition-colors"
    >
      <span className="relative flex h-4 w-4 items-center justify-center">
        <AnimatePresence mode="wait" initial={false}>
          <motion.span
            key={isDark ? "moon" : "sun"}
            initial={reduce ? false : { rotate: -90, opacity: 0, scale: 0.5 }}
            animate={{ rotate: 0, opacity: 1, scale: 1 }}
            exit={reduce ? undefined : { rotate: 90, opacity: 0, scale: 0.5 }}
            transition={{ duration: 0.2 }}
            className="absolute inset-0 flex items-center justify-center text-pilot-accent"
          >
            {/* Icon reflects the CURRENT theme: moon at Night, sun in Daylight. */}
            {isDark ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
          </motion.span>
        </AnimatePresence>
      </span>
      <span className="hidden sm:inline text-[13px] font-semibold">{currentLabel}</span>
    </button>
  );
}
