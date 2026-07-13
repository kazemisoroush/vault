import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../lib/api/client";
import type { Check } from "../lib/checks/check";
import { CheckPanel } from "./CheckPanel";

// encodeOffsets returns the UTF-8 byte range of part inside text, as the contract defines.
function encodeOffsets(text: string, part: string): { start: number; end: number } {
  const head = text.indexOf(part);
  const start = new TextEncoder().encode(text.slice(0, head)).length;
  return { start, end: start + new TextEncoder().encode(part).length };
}

const checkText = "The deposit was paid on time.";
const offsets = encodeOffsets(checkText, checkText);

// doneCheck is what GET /checks/{id} returns once the pipeline lands: one disputed claim with
// both a supporting and a contradicting reference.
const doneCheck: Check = {
  id: "chk-1",
  text: checkText,
  status: "done",
  createdAt: "2026-07-13T00:00:00Z",
  updatedAt: "2026-07-13T00:00:00Z",
  claims: [
    {
      text: checkText,
      start: offsets.start,
      end: offsets.end,
      verdict: "disputed",
      references: [
        {
          fileId: "f1",
          fileName: "Contract of Sale.pdf",
          spanText: "payable within seven days",
          start: 0,
          end: 25,
          relation: "paraphrase",
        },
        {
          fileId: "f2",
          fileName: "Email chain.pdf",
          spanText: "the deposit was not paid within seven days",
          start: 0,
          end: 42,
          relation: "contradicts",
        },
      ],
    },
  ],
};

// fakeApi answers POST /checks with a pending check and GET /checks/{id} with the finished one.
function fakeApi(): ApiClient {
  return {
    POST: vi.fn().mockResolvedValue({ data: { ...doneCheck, status: "pending", claims: undefined } }),
    GET: vi.fn().mockResolvedValue({ data: doneCheck }),
  } as unknown as ApiClient;
}

// runToDone drives the panel through paste, submit, and one fast poll to the finished check.
async function runToDone() {
  render(<CheckPanel api={fakeApi()} pollMs={20} />);
  fireEvent.change(screen.getByLabelText("Text to check"), { target: { value: checkText } });
  fireEvent.click(screen.getByRole("button", { name: "Check" }));
  return screen.findByRole("button", { name: checkText });
}

describe("CheckPanel", () => {
  it("runs a check and renders the verdict highlight with the tally", async () => {
    // Arrange + Act
    const claim = await runToDone();

    // Assert
    expect(claim.className).toContain("disputed");
    expect(screen.getByText("1 disputed")).toBeInTheDocument();
  });

  it("unfolds references inline under the clicked sentence and folds them back", async () => {
    // Arrange
    const claim = await runToDone();

    // Act
    fireEvent.click(claim);

    // Assert: both references appear with their relations, contradiction included.
    expect(await screen.findByText("contradicts")).toBeInTheDocument();
    expect(screen.getByText("paraphrase")).toBeInTheDocument();
    expect(screen.getByText(/the deposit was not paid within seven days/)).toBeInTheDocument();

    // Act: a second click folds the references away.
    fireEvent.click(claim);
    expect(screen.queryByText("contradicts")).not.toBeInTheDocument();
  });

  it("resets to a fresh paste field on Check another", async () => {
    // Arrange
    await runToDone();

    // Act
    fireEvent.click(screen.getByRole("button", { name: "Check another" }));

    // Assert
    expect(screen.getByLabelText("Text to check")).toBeInTheDocument();
    expect(screen.queryByText("1 disputed")).not.toBeInTheDocument();
  });
});
