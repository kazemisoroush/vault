interface Config {
  apiUrl: string;
  googleClientId: string;
}

function loadConfig(): Config {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL;
  if (!apiUrl) {
    throw new Error("NEXT_PUBLIC_API_URL is required");
  }

  const googleClientId = process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID;
  if (!googleClientId) {
    throw new Error("NEXT_PUBLIC_GOOGLE_CLIENT_ID is required");
  }

  return { apiUrl, googleClientId };
}

export const config = loadConfig();
