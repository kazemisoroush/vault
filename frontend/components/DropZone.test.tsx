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
    const input = document.querySelector('input[type="file"]:not([webkitdirectory])') as HTMLInputElement;

    // Act
    await userEvent.upload(input, [a, b]);

    // Assert
    expect(onFiles).toHaveBeenCalledWith([a, b]);
  });

  it("hands a picked folder's files to onFiles, skipping system junk", async () => {
    // Arrange: the folder picker is the input carrying the webkitdirectory attribute.
    const onFiles = vi.fn();
    render(<DropZone onFiles={onFiles} busy={false} />);
    const folderInput = document.querySelector("input[webkitdirectory]") as HTMLInputElement;
    expect(folderInput).not.toBeNull();
    const good = new File(["x"], "photo.jpg");
    const junk = new File(["x"], ".DS_Store");

    // Act
    await userEvent.upload(folderInput, [good, junk]);

    // Assert: only the real file is handed on.
    expect(onFiles).toHaveBeenCalledWith([good]);
  });

  it("shows the uploading count and a spinner while busy", () => {
    // Arrange + Act
    render(<DropZone onFiles={() => {}} busy={true} pending={3} />);

    // Assert
    expect(screen.getByText(/uploading 3/i)).toBeInTheDocument();
    expect(document.querySelector(".spinner")).toBeInTheDocument();
  });

  it("opens the folder picker from the add-a-folder control", async () => {
    // Arrange
    render(<DropZone onFiles={() => {}} busy={false} />);
    const folderInput = document.querySelector("input[webkitdirectory]") as HTMLInputElement;
    const clicked = vi.spyOn(folderInput, "click").mockImplementation(() => {});

    // Act
    await userEvent.click(screen.getByRole("button", { name: /add a folder/i }));

    // Assert
    expect(clicked).toHaveBeenCalled();
  });
});
