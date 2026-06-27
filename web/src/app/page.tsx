import { LoginGate } from "@/components/LoginGate";

export default function Home() {
  return (
    <LoginGate>
      <main className="flex min-h-screen items-center justify-center">
        <h1 className="text-4xl font-bold">Vault</h1>
      </main>
    </LoginGate>
  );
}
