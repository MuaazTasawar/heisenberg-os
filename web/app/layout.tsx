import type { Metadata, Viewport } from "next";
import type { ReactNode } from "react";
import Providers from "./providers";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "Heisenberg OS", template: "%s | Heisenberg OS" },
  description: "Cook it. Clean it. Stay ahead of the DEA.",
};

export const viewport: Viewport = {
  themeColor: "#060606",
  colorScheme: "dark",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        {/* eslint-disable-next-line @next/next/no-page-custom-font -- the root layout is the one place every page shares */}
        <link
          href="https://fonts.googleapis.com/css2?family=Bebas+Neue&family=Share+Tech+Mono&family=Barlow:wght@300;400;500;600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body suppressHydrationWarning={true}>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
