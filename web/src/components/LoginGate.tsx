"use client";

import type { ReactNode } from "react";
import { useAuth } from "@/lib/auth";
import { SignInButton } from "@/components/SignInButton";

// LoginGate shows the Google sign-in screen until the owner is authenticated.
export function LoginGate({ children }: { children: ReactNode }) {
  const { token, ready } = useAuth();

  if (!ready) {
    return null;
  }

  if (!token) {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center gap-6">
        <h1 className="text-4xl font-bold">Vault</h1>
        <p className="text-sm text-gray-500">Sign in with Google to continue.</p>
        <SignInButton />
      </main>
    );
  }

  return <>{children}</>;
}
