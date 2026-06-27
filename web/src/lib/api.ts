import { config } from "./config";
import { getStoredToken } from "./token";

// apiFetch calls the Vault API, attaching the signed-in user's bearer token.
export async function apiFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const token = getStoredToken();
  const headers = new Headers(init.headers);
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return fetch(`${config.apiUrl}${path}`, { ...init, headers });
}
