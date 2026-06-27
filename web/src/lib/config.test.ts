describe("config", () => {
  const originalEnv = process.env;

  beforeEach(() => {
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it("should load config from environment variables", () => {
    // Arrange
    process.env.NEXT_PUBLIC_API_URL = "https://api.example.com";
    process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID = "client-id";

    // Act
    const { config } = require("./config");

    // Assert
    expect(config.apiUrl).toBe("https://api.example.com");
    expect(config.googleClientId).toBe("client-id");
  });

  it("should throw when NEXT_PUBLIC_API_URL is missing", () => {
    // Arrange
    delete process.env.NEXT_PUBLIC_API_URL;
    process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID = "client-id";

    // Act & Assert
    expect(() => require("./config")).toThrow("NEXT_PUBLIC_API_URL is required");
  });

  it("should throw when NEXT_PUBLIC_GOOGLE_CLIENT_ID is missing", () => {
    // Arrange
    process.env.NEXT_PUBLIC_API_URL = "https://api.example.com";
    delete process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID;

    // Act & Assert
    expect(() => require("./config")).toThrow("NEXT_PUBLIC_GOOGLE_CLIENT_ID is required");
  });
});
