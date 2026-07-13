import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Claim } from "../lib/checks/check";
import { References } from "./References";

describe("References", () => {
  it("is honest about silence for an unsupported claim", () => {
    // Arrange
    const claim: Claim = {
      text: "The parties agreed to waive the penalty clause.",
      start: 0,
      end: 47,
      verdict: "unsupported",
    };

    // Act
    render(<References claim={claim} onBack={() => undefined} />);

    // Assert
    expect(screen.getByText(/unsupported: no supporting passage was confirmed/)).toBeInTheDocument();
    expect(screen.getByText(/silence is where to look hardest/)).toBeInTheDocument();
  });

  it("goes back to the record on the back button", () => {
    // Arrange
    const onBack = vi.fn();
    const claim: Claim = { text: "x", start: 0, end: 1, verdict: "verified", references: [] };
    render(<References claim={claim} onBack={onBack} />);

    // Act
    fireEvent.click(screen.getByRole("button", { name: "← The record" }));

    // Assert
    expect(onBack).toHaveBeenCalledOnce();
  });
});
