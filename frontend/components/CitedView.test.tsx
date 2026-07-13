import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "../lib/api/client";
import type { Check } from "../lib/checks/check";
import { CitedView } from "./CitedView";

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

// fakeApi answers POST /checks with a pending check and GET /checks/{id} with the finished one,
// which drives the component through submit, poll, and render.
function fakeApi(): ApiClient {
  return {
    POST: vi.fn().mockResolvedValue({ data: { ...doneCheck, status: "pending", claims: undefined } }),
    GET: vi.fn().mockResolvedValue({ data: doneCheck }),
  } as unknown as ApiClient;
}

describe("CitedView", () => {
  it("runs a check and renders the verdict highlight", async () => {
    // Arrange: a fast real-timer poll so the finished check arrives within the test.
    render(<CitedView api={fakeApi()} files={[]} pollMs={20} />);

    // Act: paste and submit, then let the poll fetch the finished check.
    fireEvent.change(screen.getByLabelText("Text to check"), { target: { value: checkText } });
    fireEvent.click(screen.getByRole("button", { name: "Check it" }));
    await screen.findByText(/Checking…/);

    // Assert
    const claim = await screen.findByRole("button", { name: checkText });
    expect(claim.className).toContain("disputed");
    expect(screen.getByText("1 disputed")).toBeInTheDocument();
  });

  it("shows the claim's references in the record panel on click", async () => {
    // Arrange
    render(<CitedView api={fakeApi()} files={[]} pollMs={20} />);
    fireEvent.change(screen.getByLabelText("Text to check"), { target: { value: checkText } });
    fireEvent.click(screen.getByRole("button", { name: "Check it" }));
    const claim = await screen.findByRole("button", { name: checkText });

    // Act
    fireEvent.click(claim);

    // Assert: both references appear with their relations, contradiction included.
    expect(await screen.findByText("contradicts")).toBeInTheDocument();
    expect(screen.getByText("paraphrase")).toBeInTheDocument();
    expect(screen.getByText(/the deposit was not paid within seven days/)).toBeInTheDocument();

    // Act: back returns to the record list.
    fireEvent.click(screen.getByRole("button", { name: "← The record" }));
    expect(screen.getByText(/The record/)).toBeInTheDocument();
  });
});
