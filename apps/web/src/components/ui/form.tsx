import * as React from "react";
import { cn } from "./cn";

export type FormProps = React.FormHTMLAttributes<HTMLFormElement>;

export function Form({ className, ...props }: FormProps) {
  return <form className={cn("grid gap-4", className)} {...props} />;
}

export function Field({
  children,
  className,
  label,
}: {
  children: React.ReactNode;
  className?: string;
  label: string;
}) {
  const id = React.useId();

  return (
    <label className={cn("grid gap-2 text-sm font-medium text-fg", className)} htmlFor={id}>
      <span>{label}</span>
      {React.isValidElement(children)
        ? React.cloneElement(children as React.ReactElement<{ id?: string }>, { id })
        : children}
    </label>
  );
}
