import type { Metadata } from "next";
import { Hanken_Grotesk } from "next/font/google";

import { AuthProvider } from "../lib/auth/context";
import { MODE_STORAGE_KEY } from "../lib/mode";
import { THEME_STORAGE_KEY } from "../lib/theme";
import "./globals.css";

// One typeface carries the whole app. globals.css aliases the display and mono roles to this same
// family, so every surface reads in one consistent voice instead of mixing a serif and a monospace.
const body = Hanken_Grotesk({ subsets: ["latin"], weight: ["400", "500", "600"], variable: "--font-body" });

export const metadata: Metadata = {
  title: "Vault",
  description: "Your personal data vault",
};

// themeScript sets the theme and mode before paint, interpolating only build-time constants.
const themeScript = `(function(){try{var t=localStorage.getItem('${THEME_STORAGE_KEY}');if(!t){t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}document.documentElement.dataset.theme=t;var m=localStorage.getItem('${MODE_STORAGE_KEY}');if(m==='legal'){document.documentElement.dataset.mode=m;}}catch(e){}})();`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={body.variable}>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
