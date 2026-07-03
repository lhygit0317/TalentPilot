import * as React from "react";
import { cn } from "./cn";

export type NavLinkProps = React.AnchorHTMLAttributes<HTMLAnchorElement> & {
  active?: boolean;
};

export const NavLink = React.forwardRef<HTMLAnchorElement, NavLinkProps>(
  ({ active = false, className, ...props }, ref) => (
    <a
      ref={ref}
      className={cn(
        "inline-flex min-h-10 items-center border px-3 text-sm font-medium transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
        active
          ? "border-accent bg-accent text-black"
          : "border-white/15 bg-white/5 text-fg hover:border-white/25 hover:bg-white/10",
        className,
      )}
      {...props}
    />
  ),
);
NavLink.displayName = "NavLink";
