import * as React from "react";

export function highlightLiteral(value: string, query: string): React.ReactNode {
  const trimmedQuery = query.trim();
  if (!trimmedQuery) {
    return value;
  }

  const index = value.toLocaleLowerCase().indexOf(trimmedQuery.toLocaleLowerCase());
  if (index < 0) {
    return value;
  }

  return (
    <>
      {value.slice(0, index)}
      <mark>{value.slice(index, index + trimmedQuery.length)}</mark>
      {value.slice(index + trimmedQuery.length)}
    </>
  );
}
