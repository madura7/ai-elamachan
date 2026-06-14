import { createNavigation } from "next-intl/navigation";
import { routing } from "./routing";

// Locale-aware navigation helpers (Link, redirect, router, pathname).
export const { Link, redirect, usePathname, useRouter, getPathname } =
  createNavigation(routing);
