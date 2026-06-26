interface Config {
  apiUrl: string;
}

function loadConfig(): Config {
  const apiUrl = process.env.NEXT_PUBLIC_API_URL;
  if (!apiUrl) {
    throw new Error("NEXT_PUBLIC_API_URL is required");
  }

  return { apiUrl };
}

export const config = loadConfig();
