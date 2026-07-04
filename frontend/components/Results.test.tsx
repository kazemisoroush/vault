import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { AskResult } from "../lib/ask/askResult";
import { Results } from "./Results";

describe("Results", () => {
  it("shows a no-matches message for an empty result", () => {
    // Arrange + Act
    render(<Results results={[]} />);

    // Assert
    expect(screen.getByText(/no matches/i)).toBeInTheDocument();
  });

  it("renders each match as a link to its download URL", () => {
    // Arrange
    const results: AskResult[] = [
      {
        file: { id: "1", name: "ticket.pdf", contentType: "application/pdf", size: 1, status: "ready", createdAt: "", updatedAt: "" },
        downloadUrl: "https://get/1",
      },
    ];

    // Act
    render(<Results results={results} />);

    // Assert
    expect(screen.getByRole("link", { name: "ticket.pdf" })).toHaveAttribute("href", "https://get/1");
  });
});
