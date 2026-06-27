import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";

describe("App", () => {
  it("renders the foundation shell using project UI copy", () => {
    render(<App />);

    expect(screen.getByRole("main", { name: "TalentPilot foundation" })).toBeInTheDocument();
    expect(screen.getByText("TalentPilot")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "进入工作台" })).toBeInTheDocument();
  });
});
