"use client";

import { useEffect, useRef } from "react";
import { config } from "@/lib/config";
import { useAuth } from "@/lib/auth";

// SignInButton renders the Google Identity Services button and reports the ID token.
export function SignInButton() {
  const { signIn } = useAuth();
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;

    const render = (): boolean => {
      if (cancelled || !window.google || !ref.current) {
        return false;
      }
      window.google.accounts.id.initialize({
        client_id: config.googleClientId,
        callback: (response) => signIn(response.credential),
      });
      window.google.accounts.id.renderButton(ref.current, {
        theme: "outline",
        size: "large",
      });
      return true;
    };

    if (render()) {
      return;
    }

    const interval = setInterval(() => {
      if (render()) {
        clearInterval(interval);
      }
    }, 200);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [signIn]);

  return <div ref={ref} />;
}
