"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import { useAuth } from "../lib/auth/context";

export default function Home() {
  const { ready, authenticated, signOut } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (ready && !authenticated) {
      router.replace("/login");
    }
  }, [ready, authenticated, router]);

  if (!ready) {
    return (
      <main>
        <p>Loading…</p>
      </main>
    );
  }
  if (!authenticated) {
    return null;
  }

  return (
    <main>
      <h1>You are in.</h1>
      <p>The vault shell is live. The ask box lands next.</p>
      <button onClick={signOut}>Sign out</button>
    </main>
  );
}
