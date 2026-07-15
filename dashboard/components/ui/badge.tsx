import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium transition-colors",
  {
    variants: {
      variant: {
        default: "border-pilot-accent/25 bg-pilot-accent/12 text-pilot-accent-light",
        success: "border-pilot-success/25 bg-pilot-success/12 text-pilot-success",
        warning: "border-pilot-warning/25 bg-pilot-warning/12 text-pilot-warning",
        danger: "border-pilot-danger/25 bg-pilot-danger/12 text-pilot-danger",
        muted: "border-pilot-border bg-pilot-surface-2 text-pilot-muted",
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
