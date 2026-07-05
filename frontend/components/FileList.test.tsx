import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { VaultFile } from "../lib/files/vaultFile";
import { FileList } from "./FileList";

const files: VaultFile[] = [
  { id: "1", name: "receipt.jpg", contentType: "image/jpeg", size: 1, status: "ready", createdAt: "", updatedAt: "" },
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
    expect(screen.getByText("ready")).toBeInTheDocument();
    expect(screen.getByText("contract.pdf")).toBeInTheDocument();
    expect(screen.getByText("pending")).toBeInTheDocument();
  });

  it("deletes a file only after confirming", async () => {
    // Arrange
    const onDelete = vi.fn();
    render(<FileList files={files} onDelete={onDelete} />);

    // Act: open the confirm, then confirm
    await userEvent.click(screen.getByRole("button", { name: /delete receipt.jpg/i }));
    expect(onDelete).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));

    // Assert
    expect(onDelete).toHaveBeenCalledWith("1");
  });

  it("cancels without deleting", async () => {
    // Arrange
    const onDelete = vi.fn();
    render(<FileList files={files} onDelete={onDelete} />);

    // Act
    await userEvent.click(screen.getByRole("button", { name: /delete contract.pdf/i }));
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    // Assert
    expect(onDelete).not.toHaveBeenCalled();
  });
});
