import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { LlmCall } from "../lib/calls/llmCall";
import { Trace } from "./Trace";

const call: LlmCall = {
  op: "retrieve",
  model: "haiku",
  prompt: "the prompt",
  reply: "the reply",
  latencyMs: 12,
  inputTokens: 5,
  outputTokens: 3,
  ok: true,
  createdAt: "2026-01-01T00:00:00Z",
};

describe("Trace", () => {
  it("renders nothing when there are no calls", () => {
    // Arrange + Act
    const { container } = render(<Trace calls={[]} />);

    // Assert
    expect(container).toBeEmptyDOMElement();
  });

  it("shows a row and reveals the prompt and reply on click", async () => {
    // Arrange
    render(<Trace calls={[call]} />);

    // Assert (collapsed)
    expect(screen.getByText("retrieve")).toBeInTheDocument();
    expect(screen.queryByText("the prompt")).not.toBeInTheDocument();

    // Act
    await userEvent.click(screen.getByText("retrieve"));

    // Assert (expanded)
    expect(screen.getByText("the prompt")).toBeInTheDocument();
    expect(screen.getByText("the reply")).toBeInTheDocument();
  });
});
