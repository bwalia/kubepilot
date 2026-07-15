/**
 * Shared Framer Motion (motion/react) primitives for the KubePilot deck.
 * Everything here honours prefers-reduced-motion via Motion's own
 * useReducedMotion(): when set, entrances resolve instantly (no transform) so
 * the UI stays perfectly usable for motion-sensitive operators.
 */
import * as React from "react";
import { motion, useReducedMotion, type Variants } from "motion/react";

/** A card/section that rises + fades in on mount. Optional stagger index. */
export function Reveal({
  children,
  delay = 0,
  y = 14,
  className,
  as = "div",
}: {
  children: React.ReactNode;
  delay?: number;
  y?: number;
  className?: string;
  as?: "div" | "section" | "li";
}) {
  const reduce = useReducedMotion();
  const MotionTag = motion[as] as typeof motion.div;
  return (
    <MotionTag
      className={className}
      initial={reduce ? false : { opacity: 0, y }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, delay, ease: [0.22, 1, 0.36, 1] }}
    >
      {children}
    </MotionTag>
  );
}

/** Container that staggers its <Stagger.Item> children in a natural wave. */
export const staggerContainer: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.06, delayChildren: 0.04 } },
};

export const staggerItem: Variants = {
  hidden: { opacity: 0, y: 16, scale: 0.98 },
  show: { opacity: 1, y: 0, scale: 1, transition: { duration: 0.45, ease: [0.22, 1, 0.36, 1] } },
};

/** Grid/list wrapper that orchestrates a staggered reveal of its children. */
export function StaggerGroup({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const reduce = useReducedMotion();
  if (reduce) return <div className={className}>{children}</div>;
  return (
    <motion.div
      className={className}
      variants={staggerContainer}
      initial="hidden"
      animate="show"
    >
      {children}
    </motion.div>
  );
}

/** One member of a StaggerGroup. Lifts slightly on hover for tactility. */
export function StaggerCard({
  children,
  className,
  lift = true,
}: {
  children: React.ReactNode;
  className?: string;
  lift?: boolean;
}) {
  const reduce = useReducedMotion();
  return (
    <motion.div
      className={className}
      variants={staggerItem}
      whileHover={reduce || !lift ? undefined : { y: -3 }}
      transition={{ type: "spring", stiffness: 400, damping: 28 }}
    >
      {children}
    </motion.div>
  );
}

export { motion, useReducedMotion };
