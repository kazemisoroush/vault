import createClient, { type Client, type Middleware } from "openapi-fetch";

import type { AppConfig } from "../config";
import type { paths } from "./schema";

// ApiClient is the typed Vault API client generated from openapi.yaml.
export type ApiClient = Client<paths>;

// createApiClient builds a typed client that attaches the current bearer token to every request.
export function createApiClient(config: AppConfig, getToken: () => string | null): ApiClient {
  const client = createClient<paths>({ baseUrl: config.apiUrl });

  const authMiddleware: Middleware = {
    onRequest({ request }) {
      const token = getToken();
      if (token) {
        request.headers.set("Authorization", `Bearer ${token}`);
      }
      return request;
    },
  };

  client.use(authMiddleware);
  return client;
}
