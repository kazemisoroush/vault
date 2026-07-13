import type { Metadata } from "next";
import { Fraunces, Hanken_Grotesk, IBM_Plex_Mono } from "next/font/google";

import { AuthProvider } from "../lib/auth/context";
import { MODE_STORAGE_KEY } from "../lib/mode";
import { THEME_STORAGE_KEY } from "../lib/theme";
import "./globals.css";

const display = Fraunces({ subsets: ["latin"], weight: ["400", "500"], variable: "--font-display" });
const body = Hanken_Grotesk({ subsets: ["latin"], variable: "--font-body" });
const mono = IBM_Plex_Mono({ subsets: ["latin"], weight: ["400", "500"], variable: "--font-mono" });

export const metadata: Metadata = {
  title: "Vault",
  description: "Your personal data vault",
};

// themeScript sets the theme and mode before paint, interpolating only build-time constants.
const themeScript = `(function(){try{var t=localStorage.getItem('${THEME_STORAGE_KEY}');if(!t){t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}document.documentElement.dataset.theme=t;var m=localStorage.getItem('${MODE_STORAGE_KEY}');if(m==='legal'){document.documentElement.dataset.mode=m;}}catch(e){}})();`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${display.variable} ${body.variable} ${mono.variable}`}>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
