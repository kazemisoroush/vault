import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DropZone } from "./DropZone";

describe("DropZone", () => {
  it("hands every picked file to onFiles", async () => {
    // Arrange
    const onFiles = vi.fn();
    render(<DropZone onFiles={onFiles} busy={false} />);
    const a = new File(["a"], "a.txt", { type: "text/plain" });
    const b = new File(["b"], "b.txt", { type: "text/plain" });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;

    // Act
    await userEvent.upload(input, [a, b]);

    // Assert
    expect(onFiles).toHaveBeenCalledWith([a, b]);
  });

  it("shows the uploading count and a spinner while busy", () => {
    // Arrange + Act
    render(<DropZone onFiles={() => {}} busy={true} pending={3} />);

    // Assert
    expect(screen.getByText(/uploading 3/i)).toBeInTheDocument();
    expect(document.querySelector(".spinner")).toBeInTheDocument();
  });
});
