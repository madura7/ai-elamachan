import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const PROTECTED = ["/listings", "/dashboard"];
const TOKEN_COOKIE = "elamachan_token";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  const isProtected = PROTECTED.some(
    (p) => pathname === p || pathname.startsWith(p + "/")
  );

  if (isProtected) {
    const token = request.cookies.get(TOKEN_COOKIE)?.value;
    if (!token) {
      const url = request.nextUrl.clone();
      url.pathname = "/auth";
      return NextResponse.redirect(url);
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/listings/:path*", "/dashboard/:path*"],
};
