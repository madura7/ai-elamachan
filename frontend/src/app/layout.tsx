import type { Metadata } from "next";
import "./globals.css";
import Nav from "@/components/Nav";
import Footer from "@/components/Footer";

export const metadata: Metadata = {
  title: "ElaMachan | ඇළමාකාන් | எளமாகான்",
  description: "Sri Lanka's trusted marketplace",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-background text-ink">
        <Nav />
        {children}
        <Footer />
      </body>
    </html>
  );
}
