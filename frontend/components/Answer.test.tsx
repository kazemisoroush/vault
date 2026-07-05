import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Answer } from "./Answer";

describe("Answer", () => {
  it("shows the answer and its source file", () => {
    // Arrange + Act
    render(<Answer answer="RA3495037" source="passport.jpg" />);

    // Assert
    expect(screen.getByText("RA3495037")).toBeInTheDocument();
    expect(screen.getByText(/from passport.jpg/i)).toBeInTheDocument();
  });

  it("omits the source line when there is no source", () => {
    // Arrange + Act
    render(<Answer answer="42" />);

    // Assert
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.queryByText(/^from /i)).not.toBeInTheDocument();
  });
});
