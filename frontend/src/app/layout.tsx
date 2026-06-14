import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "ElaMachan | ඇළමාකාන් | எளமாகான்",
  description: "Sri Lanka's trusted marketplace",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-orange-50">
        {children}
      </body>
    </html>
  );
}
