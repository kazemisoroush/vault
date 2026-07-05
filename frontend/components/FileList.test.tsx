import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { VaultFile } from "../lib/files/vaultFile";
import { FileList } from "./FileList";

const files: VaultFile[] = [
  { id: "1", name: "receipt.jpg", contentType: "image/jpeg", size: 1, status: "ready", createdAt: "", updatedAt: "" },
  { id: "2", name: "contract.pdf", contentType: "application/pdf", size: 2, status: "pending", createdAt: "", updatedAt: "" },
];

function renderList(overrides: Partial<Parameters<typeof FileList>[0]> = {}) {
  render(<FileList files={files} onDelete={vi.fn()} onRename={vi.fn()} {...overrides} />);
}

describe("FileList", () => {
  it("shows an empty state when there are no files", () => {
    // Arrange + Act
    render(<FileList files={[]} onDelete={vi.fn()} onRename={vi.fn()} />);

    // Assert
    expect(screen.getByText(/no files yet/i)).toBeInTheDocument();
  });

  it("renders each file with its status", () => {
    // Arrange + Act
    renderList();

    // Assert
    expect(screen.getByText("receipt.jpg")).toBeInTheDocument();
    expect(screen.getByText("ready")).toBeInTheDocument();
    expect(screen.getByText("contract.pdf")).toBeInTheDocument();
    expect(screen.getByText("pending")).toBeInTheDocument();
  });

  it("deletes a file only after confirming", async () => {
    // Arrange
    const onDelete = vi.fn();
    renderList({ onDelete });

    // Act
    await userEvent.click(screen.getByRole("button", { name: /delete receipt.jpg/i }));
    expect(onDelete).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));

    // Assert
    expect(onDelete).toHaveBeenCalledWith("1");
  });

  it("cancels a delete without removing", async () => {
    // Arrange
    const onDelete = vi.fn();
    renderList({ onDelete });

    // Act
    await userEvent.click(screen.getByRole("button", { name: /delete contract.pdf/i }));
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    // Assert
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("renames a file after editing", async () => {
    // Arrange
    const onRename = vi.fn();
    renderList({ onRename });

    // Act
    await userEvent.click(screen.getByRole("button", { name: /rename receipt.jpg/i }));
    const input = screen.getByLabelText(/new name for receipt.jpg/i);
    await userEvent.clear(input);
    await userEvent.type(input, "invoice.jpg");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    // Assert
    expect(onRename).toHaveBeenCalledWith("1", "invoice.jpg");
  });

  it("does not rename when the name is unchanged", async () => {
    // Arrange
    const onRename = vi.fn();
    renderList({ onRename });

    // Act
    await userEvent.click(screen.getByRole("button", { name: /rename contract.pdf/i }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    // Assert
    expect(onRename).not.toHaveBeenCalled();
  });
});
