import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Vault",
  description: "Personal data vault",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
