import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { VaultFile } from "../lib/files/vaultFile";
import { FileList } from "./FileList";

const files: VaultFile[] = [
  { id: "1", name: "receipt.jpg", contentType: "image/jpeg", size: 1, status: "ingested", createdAt: "", updatedAt: "" },
  { id: "2", name: "contract.pdf", contentType: "application/pdf", size: 2, status: "pending", createdAt: "", updatedAt: "" },
];

describe("FileList", () => {
  it("shows an empty state when there are no files", () => {
    // Arrange + Act
    render(<FileList files={[]} onDelete={vi.fn()} />);

    // Assert
    expect(screen.getByText(/no files yet/i)).toBeInTheDocument();
  });

  it("renders each file with its status", () => {
    // Arrange + Act
    render(<FileList files={files} onDelete={vi.fn()} />);

    // Assert
    expect(screen.getByText("receipt.jpg")).toBeInTheDocument();
    expect(screen.getByText("ingested")).toBeInTheDocument();
    expect(screen.getByText("contract.pdf")).toBeInTheDocument();
    expect(screen.getByText("pending")).toBeInTheDocument();
  });

  it("deletes only on the second tap of the same button", async () => {
    // Arrange
    const onDelete = vi.fn();
    render(<FileList files={files} onDelete={onDelete} />);

    // Act: first tap arms (the label flips to a confirm), still no delete.
    await userEvent.click(screen.getByRole("button", { name: /delete receipt.jpg/i }));
    expect(onDelete).not.toHaveBeenCalled();
    const confirm = screen.getByRole("button", { name: /confirm delete receipt.jpg/i });

    // Act: second tap on the same button deletes.
    await userEvent.click(confirm);

    // Assert
    expect(onDelete).toHaveBeenCalledWith("1");
  });

  it("reverts to the trash icon if the confirm window passes without a second tap", async () => {
    // Arrange
    vi.useFakeTimers();
    try {
      const onDelete = vi.fn();
      render(<FileList files={files} onDelete={onDelete} />);

      // Act: arm, then let the window elapse.
      fireEvent.click(screen.getByRole("button", { name: /delete contract.pdf/i }));
      expect(screen.getByRole("button", { name: /confirm delete contract.pdf/i })).toBeInTheDocument();
      act(() => vi.advanceTimersByTime(5000));

      // Assert: it is back to a plain delete control and nothing was deleted.
      expect(screen.getByRole("button", { name: /^delete contract.pdf/i })).toBeInTheDocument();
      expect(onDelete).not.toHaveBeenCalled();
    } finally {
      vi.useRealTimers();
    }
  });
});
