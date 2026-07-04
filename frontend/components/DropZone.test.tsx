import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DropZone } from "./DropZone";

describe("DropZone", () => {
  it("hands a picked file to onFile", async () => {
    // Arrange
    const onFile = vi.fn();
    render(<DropZone onFile={onFile} busy={false} />);
    const file = new File(["hi"], "a.txt", { type: "text/plain" });
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;

    // Act
    await userEvent.upload(input, file);

    // Assert
    expect(onFile).toHaveBeenCalledWith(file);
  });

  it("shows the uploading state while busy", () => {
    // Arrange + Act
    render(<DropZone onFile={() => {}} busy={true} />);

    // Assert
    expect(screen.getByText(/uploading/i)).toBeInTheDocument();
  });
});
