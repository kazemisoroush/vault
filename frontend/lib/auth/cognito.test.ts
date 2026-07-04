import { beforeEach, describe, expect, it, vi } from "vitest";

const authenticateUser = vi.fn();
const getCurrentUser = vi.fn();

vi.mock("amazon-cognito-identity-js", () => ({
  CognitoUserPool: vi.fn().mockImplementation(() => ({ getCurrentUser })),
  CognitoUser: vi.fn().mockImplementation(() => ({ authenticateUser })),
  AuthenticationDetails: vi.fn(),
}));

import { CognitoAuth } from "./cognito";

const config = {
  apiUrl: "https://api.example.com",
  cognitoRegion: "us-east-1",
  cognitoUserPoolId: "us-east-1_pool",
  cognitoClientId: "client-id",
};

describe("CognitoAuth", () => {
  beforeEach(() => {
    authenticateUser.mockReset();
    getCurrentUser.mockReset();
  });

  it("resolves the access token on a successful sign in", async () => {
    authenticateUser.mockImplementation((_details, callbacks) => {
      callbacks.onSuccess({ getAccessToken: () => ({ getJwtToken: () => "jwt-token" }) });
    });

    const auth = new CognitoAuth(config);

    await expect(auth.signIn("me@example.com", "pw")).resolves.toBe("jwt-token");
  });

  it("rejects when sign in fails", async () => {
    authenticateUser.mockImplementation((_details, callbacks) => {
      callbacks.onFailure(new Error("bad credentials"));
    });

    const auth = new CognitoAuth(config);

    await expect(auth.signIn("me@example.com", "pw")).rejects.toThrow("bad credentials");
  });

  it("returns null token when there is no stored user", async () => {
    getCurrentUser.mockReturnValue(null);

    const auth = new CognitoAuth(config);

    await expect(auth.currentToken()).resolves.toBeNull();
  });
});
