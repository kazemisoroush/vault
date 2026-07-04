"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { createApiClient, type ApiClient } from "../api/client";
import { loadConfig, type AppConfig } from "../config";
import { CognitoAuth } from "./cognito";

// AuthState is the auth surface the UI consumes: readiness, session, and the typed API client.
export type AuthState = {
  ready: boolean;
  authenticated: boolean;
  signIn: (email: string, password: string) => Promise<void>;
  signOut: () => void;
  api: ApiClient | null;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [auth, setAuth] = useState<CognitoAuth | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let active = true;
    loadConfig()
      .then(async (cfg) => {
        const cognito = new CognitoAuth(cfg);
        const existing = await cognito.currentToken();
        if (!active) return;
        setConfig(cfg);
        setAuth(cognito);
        setToken(existing);
        setReady(true);
      })
      .catch(() => {
        if (active) setReady(true);
      });
    return () => {
      active = false;
    };
  }, []);

  const signIn = useCallback(
    async (email: string, password: string) => {
      if (!auth) throw new Error("auth is not ready");
      setToken(await auth.signIn(email, password));
    },
    [auth],
  );

  const signOut = useCallback(() => {
    auth?.signOut();
    setToken(null);
  }, [auth]);

  const api = useMemo(
    () => (config ? createApiClient(config, () => token, signOut) : null),
    [config, token, signOut],
  );

  const value: AuthState = { ready, authenticated: token !== null, signIn, signOut, api };
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
  return ctx;
}
