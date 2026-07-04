// AppConfig is the runtime configuration the SPA needs, served as /config.json.
// The values are written into the bucket at deploy time from the CDK stack outputs,
// so the static build carries no environment-specific values.
export type AppConfig = {
  apiUrl: string;
  cognitoUserPoolId: string;
  cognitoClientId: string;
};

const requiredKeys: (keyof AppConfig)[] = ["apiUrl", "cognitoUserPoolId", "cognitoClientId"];

// parseConfig validates a raw config.json payload, failing loudly on missing keys.
export function parseConfig(raw: Partial<AppConfig>): AppConfig {
  for (const key of requiredKeys) {
    if (!raw[key]) {
      throw new Error(`config.json is missing ${key}`);
    }
  }
  return raw as AppConfig;
}

let cached: AppConfig | null = null;

// loadConfig fetches and memoizes /config.json.
export async function loadConfig(): Promise<AppConfig> {
  if (cached) {
    return cached;
  }
  const response = await fetch("/config.json", { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`load config.json: ${response.status}`);
  }
  cached = parseConfig((await response.json()) as Partial<AppConfig>);
  return cached;
}
