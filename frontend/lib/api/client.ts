import createClient, { type Client, type Middleware } from "openapi-fetch";

import type { AppConfig } from "../config";
import type { paths } from "./schema";

// ApiClient is the typed Vault API client generated from openapi.yaml.
export type ApiClient = Client<paths>;

// createApiClient builds a typed client that attaches the current bearer token to every request
// and calls onUnauthorized when the API rejects a token with 401, so the app can send the user
// back to sign in.
export function createApiClient(
  config: AppConfig,
  getToken: () => string | null,
  onUnauthorized: () => void,
): ApiClient {
  const client = createClient<paths>({ baseUrl: config.apiUrl });

  const authMiddleware: Middleware = {
    onRequest({ request }) {
      const token = getToken();
      if (token) {
        request.headers.set("Authorization", `Bearer ${token}`);
      }
      return request;
    },
    onResponse({ response }) {
      if (response.status === 401) {
        onUnauthorized();
      }
      return response;
    },
  };

  client.use(authMiddleware);
  return client;
}
