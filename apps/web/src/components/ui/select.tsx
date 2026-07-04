import * as React from "react";
import { cn } from "./cn";

export type SelectProps = React.SelectHTMLAttributes<HTMLSelectElement>;

export const Select = React.forwardRef<HTMLSelectElement, SelectProps>(({ className, ...props }, ref) => (
  <select
    ref={ref}
    className={cn(
      "min-h-11 w-full border border-white/15 bg-white/5 px-3 text-sm text-fg outline-none transition focus-visible:border-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
      className,
    )}
    {...props}
  />
));
Select.displayName = "Select";
