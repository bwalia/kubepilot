import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-semibold transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-pilot-accent/50 disabled:opacity-50 disabled:pointer-events-none",
  {
    variants: {
      variant: {
        default: "bg-pilot-accent text-pilot-bg hover:bg-pilot-accent-light hover:shadow-glow-blue",
        success: "bg-pilot-success text-pilot-bg hover:brightness-110",
        warning: "bg-pilot-warning text-pilot-bg hover:brightness-110",
        danger: "bg-pilot-danger text-pilot-bg hover:brightness-110 hover:shadow-glow-red",
        outline:
          "border border-pilot-border bg-transparent text-pilot-text-secondary hover:border-pilot-accent/50 hover:text-pilot-text-primary hover:bg-pilot-hover/[0.04]",
        ghost: "bg-transparent text-pilot-muted hover:text-pilot-text-primary hover:bg-pilot-hover/[0.06]",
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 px-3 text-xs",
        icon: "h-8 w-8",
      },
    },
    defaultVariants: { variant: "default", size: "default" },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, size }), className)} {...props} />
  )
);
Button.displayName = "Button";

export { Button, buttonVariants };
