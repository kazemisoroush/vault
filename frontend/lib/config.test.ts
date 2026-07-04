import { describe, expect, it } from "vitest";

import { parseConfig } from "./config";

describe("parseConfig", () => {
  const full = {
    apiUrl: "https://api.example.com",
    cognitoRegion: "us-east-1",
    cognitoUserPoolId: "us-east-1_pool",
    cognitoClientId: "client-id",
  };

  it("returns the config when every key is present", () => {
    expect(parseConfig(full)).toEqual(full);
  });

  it("throws naming the missing key", () => {
    expect(() => parseConfig({ apiUrl: "https://api.example.com" })).toThrow(/cognitoRegion/);
  });
});
