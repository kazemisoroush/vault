import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { VaultFile } from "../lib/files/vaultFile";
import { FileList } from "./FileList";

describe("FileList", () => {
  it("shows an empty state when there are no files", () => {
    // Arrange + Act
    render(<FileList files={[]} />);

    // Assert
    expect(screen.getByText(/no files yet/i)).toBeInTheDocument();
  });

  it("renders each file with its status", () => {
    // Arrange
    const files: VaultFile[] = [
      { id: "1", name: "receipt.jpg", contentType: "image/jpeg", size: 1, status: "ready", createdAt: "", updatedAt: "" },
      { id: "2", name: "contract.pdf", contentType: "application/pdf", size: 2, status: "pending", createdAt: "", updatedAt: "" },
    ];

    // Act
    render(<FileList files={files} />);

    // Assert
    expect(screen.getByText("receipt.jpg")).toBeInTheDocument();
    expect(screen.getByText("ready")).toBeInTheDocument();
    expect(screen.getByText("contract.pdf")).toBeInTheDocument();
    expect(screen.getByText("pending")).toBeInTheDocument();
  });
});
