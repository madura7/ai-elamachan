"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { isAuthenticated } from "@/lib/auth";

export default function Home() {
  const router = useRouter();

  useEffect(() => {
    if (isAuthenticated()) {
      router.replace("/listings");
    } else {
      router.replace("/auth");
    }
  }, [router]);

  return (
    <div className="flex items-center justify-center min-h-[80vh]">
      <p className="text-gray-500 text-sm">Loading…</p>
    </div>
  );
}
