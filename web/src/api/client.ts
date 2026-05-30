import type { ApiEnvelope } from "@/types";

const TOKEN_KEY = "ripple_note_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function removeToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

export async function apiRequest<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  const res = await fetch(path, { ...options, headers });

  if (!res.ok) {
    const body: ApiEnvelope<unknown> = await res.json().catch(() => ({
      data: null,
      error: { code: "unknown", message: res.statusText },
      request_id: "",
    }));
    const err = body.error ?? { code: "unknown", message: res.statusText };
    throw new ApiError(res.status, err.code, err.message);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const body: ApiEnvelope<T> = await res.json();
  return body.data;
}

export async function uploadFile(
  path: string,
  file: File
): Promise<{ url: string }> {
  const token = getToken();
  const formData = new FormData();
  formData.append("file", file);

  const res = await fetch(path, {
    method: "POST",
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: formData,
  });

  if (!res.ok) {
    const body: ApiEnvelope<unknown> = await res.json().catch(() => ({
      data: null,
      error: { code: "unknown", message: res.statusText },
      request_id: "",
    }));
    const err = body.error ?? { code: "unknown", message: res.statusText };
    throw new ApiError(res.status, err.code, err.message);
  }

  const body: ApiEnvelope<{ url: string }> = await res.json();
  return body.data;
}

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}
