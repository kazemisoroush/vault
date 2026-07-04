import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AskBox } from "./AskBox";

describe("AskBox", () => {
  it("submits the trimmed query", async () => {
    // Arrange
    const onAsk = vi.fn();
    render(<AskBox onAsk={onAsk} busy={false} />);

    // Act
    await userEvent.type(screen.getByLabelText(/ask for a file/i), "  petrol receipts  ");
    await userEvent.click(screen.getByRole("button", { name: /ask/i }));

    // Assert
    expect(onAsk).toHaveBeenCalledWith("petrol receipts");
  });

  it("does not submit an empty query", async () => {
    // Arrange
    const onAsk = vi.fn();
    render(<AskBox onAsk={onAsk} busy={false} />);

    // Act
    await userEvent.click(screen.getByRole("button", { name: /ask/i }));

    // Assert
    expect(onAsk).not.toHaveBeenCalled();
  });
});
