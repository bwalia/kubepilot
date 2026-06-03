import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium transition-colors",
  {
    variants: {
      variant: {
        default: "border-transparent bg-pilot-accent/15 text-pilot-accent-light",
        success: "border-transparent bg-emerald-500/15 text-pilot-success",
        warning: "border-transparent bg-amber-500/15 text-pilot-warning",
        danger: "border-transparent bg-red-500/15 text-pilot-danger",
        muted: "border-pilot-border bg-pilot-surface text-pilot-muted",
      },
    },
    defaultVariants: { variant: "default" },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />;
}

export { badgeVariants };
