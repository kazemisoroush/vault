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

    // Act
    const { config } = require("./config");

    // Assert
    expect(config.apiUrl).toBe("https://api.example.com");
  });

  it("should throw when NEXT_PUBLIC_API_URL is missing", () => {
    // Arrange
    delete process.env.NEXT_PUBLIC_API_URL;

    // Act & Assert
    expect(() => require("./config")).toThrow("NEXT_PUBLIC_API_URL is required");
  });
});
