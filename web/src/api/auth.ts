import { apiRequest, setToken, removeToken } from "./client";
import type { Session, User } from "@/types";

export async function register(
  email: string,
  password: string,
  nickname: string
): Promise<User> {
  const user = await apiRequest<User>("/api/users", {
    method: "POST",
    body: JSON.stringify({ email, password, nickname }),
  });
  return user;
}

export async function login(
  email: string,
  password: string
): Promise<Session> {
  const session = await apiRequest<Session>("/api/sessions", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  setToken(session.token);
  return session;
}

export async function getMe(): Promise<User> {
  return apiRequest<User>("/api/users/me");
}

export function logout(): void {
  removeToken();
}
