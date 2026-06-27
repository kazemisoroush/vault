export const TOKEN_STORAGE_KEY = "vault.idToken";

export function getStoredToken(): string | null {
  if (typeof localStorage === "undefined") {
    return null;
  }
  return localStorage.getItem(TOKEN_STORAGE_KEY);
}

export function setStoredToken(token: string): void {
  if (typeof localStorage === "undefined") {
    return;
  }
  localStorage.setItem(TOKEN_STORAGE_KEY, token);
}

export function clearStoredToken(): void {
  if (typeof localStorage === "undefined") {
    return;
  }
  localStorage.removeItem(TOKEN_STORAGE_KEY);
}
