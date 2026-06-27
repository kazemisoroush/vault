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
import { clearStoredToken, getStoredToken, setStoredToken } from "./token";

interface AuthState {
  token: string | null;
  ready: boolean;
  signIn: (token: string) => void;
  signOut: () => void;
}

const AuthContext = createContext<AuthState | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    setToken(getStoredToken());
    setReady(true);
  }, []);

  const signIn = useCallback((value: string) => {
    setStoredToken(value);
    setToken(value);
  }, []);

  const signOut = useCallback(() => {
    clearStoredToken();
    window.google?.accounts.id.disableAutoSelect();
    setToken(null);
  }, []);

  const value = useMemo<AuthState>(
    () => ({ token, ready, signIn, signOut }),
    [token, ready, signIn, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}
