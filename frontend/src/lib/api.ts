const BASE = "/api/v1";

export interface OTPRequestResponse {
  challenge_id: string;
  expires_at: string;
}

export interface OTPVerifyResponse {
  token: string;
  expires_at: string;
  user: {
    id: string;
    phone: string;
    display_name: string | null;
    preferred_language: string;
  };
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const err = await res.json();
      message = err.message ?? err.error ?? message;
    } catch {
      // ignore parse error
    }
    throw new Error(message);
  }

  return res.json() as Promise<T>;
}

export function requestOTP(phone: string): Promise<OTPRequestResponse> {
  return post<OTPRequestResponse>("/auth/otp/request", {
    phone,
    purpose: "login",
  });
}

export function verifyOTP(
  challenge_id: string,
  code: string
): Promise<OTPVerifyResponse> {
  return post<OTPVerifyResponse>("/auth/otp/verify", { challenge_id, code });
}
