import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const signIn = vi.fn();
const replace = vi.fn();

vi.mock("../../lib/auth/context", () => ({ useAuth: () => ({ signIn }) }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    signIn.mockReset();
    replace.mockReset();
  });

  it("submits the credentials and navigates home", async () => {
    signIn.mockResolvedValue(undefined);
    render(<LoginPage />);

    await userEvent.type(screen.getByLabelText(/email/i), "me@example.com");
    await userEvent.type(screen.getByLabelText(/password/i), "secret");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));

    expect(signIn).toHaveBeenCalledWith("me@example.com", "secret");
    expect(replace).toHaveBeenCalledWith("/");
  });

  it("shows an error when sign in fails", async () => {
    signIn.mockRejectedValue(new Error("nope"));
    render(<LoginPage />);

    await userEvent.type(screen.getByLabelText(/email/i), "me@example.com");
    await userEvent.type(screen.getByLabelText(/password/i), "secret");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent("nope");
    expect(replace).not.toHaveBeenCalled();
  });
});
