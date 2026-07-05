"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";

import { ThemeToggle } from "../../components/ThemeToggle";
import { Wordmark } from "../../components/Wordmark";
import { useAuth } from "../../lib/auth/context";

export default function LoginPage() {
  const { signIn } = useAuth();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await signIn(email, password);
      router.replace("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "sign in failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="login-shell">
      <div className="corner-toggle">
        <ThemeToggle />
      </div>
      <div className="login">
        <Wordmark />
        <p className="lede">Sign in to your vault.</p>
        <form onSubmit={onSubmit}>
          <label>
            Email
            <input
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
            />
          </label>
          <label>
            Password
            <input
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </label>
          <button className="btn" type="submit" disabled={busy}>
            {busy ? "Signing in…" : "Sign in"}
          </button>
        </form>
        {error && <p role="alert">{error}</p>}
      </div>
    </main>
  );
}
