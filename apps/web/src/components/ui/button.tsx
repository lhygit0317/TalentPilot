import * as React from "react";
import { type VariantProps, cva } from "class-variance-authority";
import { cn } from "./cn";

const buttonVariants = cva(
  "inline-flex min-h-11 items-center justify-center border px-4 py-2 text-sm font-medium transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-45 active:translate-y-px",
  {
    variants: {
      variant: {
        primary:
          "border-transparent bg-accent text-black hover:bg-[color-mix(in_oklch,oklch(73%_0.13_190),black_10%)] focus-visible:outline-accent",
        secondary:
          "border-white/15 bg-white/5 text-fg hover:border-white/25 hover:bg-white/10 focus-visible:outline-accent",
      },
    },
    defaultVariants: {
      variant: "secondary",
    },
  },
);

export type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants>;

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, ...props }, ref) => (
    <button ref={ref} className={cn(buttonVariants({ variant, className }))} {...props} />
  ),
);
Button.displayName = "Button";
