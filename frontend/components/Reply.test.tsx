import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Reply } from "./Reply";
import type { AskOutcome } from "../lib/ask/askOutcome";

function outcomeWith(answer: string): AskOutcome {
  return {
    answer,
    results: [
      {
        file: { id: "1", name: "petrol.jpg", contentType: "image/jpeg" },
        downloadUrl: "https://example.test/petrol.jpg",
      },
      {
        file: { id: "2", name: "receipt.pdf", contentType: "application/pdf" },
        downloadUrl: "https://example.test/receipt.pdf",
      },
    ],
  } as AskOutcome;
}

describe("Reply", () => {
  it("shows the answer prose and each grounded source as an openable link", () => {
    // Arrange + Act
    render(<Reply outcome={outcomeWith("Your last fill was 52.30 at Shell.")} />);

    // Assert
    expect(screen.getByText(/your last fill was 52.30/i)).toBeInTheDocument();
    expect(screen.getByText("Grounded in")).toBeInTheDocument();
    const source = screen.getByRole("link", { name: /open petrol.jpg/i });
    expect(source).toHaveAttribute("href", "https://example.test/petrol.jpg");
  });

  it("falls back to a found count when there is no prose answer", () => {
    // Arrange + Act
    render(<Reply outcome={outcomeWith("")} />);

    // Assert
    expect(screen.getByText(/found · 2 files/i)).toBeInTheDocument();
    expect(screen.queryByText("Grounded in")).not.toBeInTheDocument();
  });
});
