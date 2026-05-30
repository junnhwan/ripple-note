import { apiRequest } from "./client";
import type { FeedResult } from "@/types";

export async function getLatestFeed(
  cursor = "",
  limit = 20
): Promise<FeedResult> {
  const params = new URLSearchParams();
  if (cursor) params.set("cursor", cursor);
  params.set("limit", String(limit));
  return apiRequest<FeedResult>(`/api/feed/latest?${params}`);
}

export async function getHotFeed(
  cursor = "",
  limit = 20
): Promise<FeedResult> {
  const params = new URLSearchParams();
  if (cursor) params.set("cursor", cursor);
  params.set("limit", String(limit));
  return apiRequest<FeedResult>(`/api/feed/hot?${params}`);
}

export async function getFollowingFeed(
  cursor = "",
  limit = 20
): Promise<FeedResult> {
  const params = new URLSearchParams();
  if (cursor) params.set("cursor", cursor);
  params.set("limit", String(limit));
  return apiRequest<FeedResult>(`/api/feed/following?${params}`);
}

export async function getTagFeed(
  tag: string,
  cursor = "",
  limit = 20
): Promise<FeedResult> {
  const params = new URLSearchParams();
  if (cursor) params.set("cursor", cursor);
  params.set("limit", String(limit));
  return apiRequest<FeedResult>(`/api/tags/${encodeURIComponent(tag)}/feed?${params}`);
}
