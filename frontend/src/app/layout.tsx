import type { Metadata } from "next";
import "./globals.css";
import Nav from "@/components/Nav";

export const metadata: Metadata = {
  title: "ElaMachan | ඇළමාකාන் | எளமாகான்",
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
        <Nav />
        {children}
      </body>
    </html>
  );
}
