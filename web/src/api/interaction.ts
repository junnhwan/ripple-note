import { apiRequest } from "./client";
import type { Comment, CommentList } from "@/types";

// Like
export async function likeNote(noteId: number): Promise<void> {
  return apiRequest<void>(`/api/notes/${noteId}/like`, { method: "PUT" });
}

export async function unlikeNote(noteId: number): Promise<void> {
  return apiRequest<void>(`/api/notes/${noteId}/like`, { method: "DELETE" });
}

// Favorite
export async function favoriteNote(noteId: number): Promise<void> {
  return apiRequest<void>(`/api/notes/${noteId}/favorite`, { method: "PUT" });
}

export async function unfavoriteNote(noteId: number): Promise<void> {
  return apiRequest<void>(`/api/notes/${noteId}/favorite`, { method: "DELETE" });
}

// Comments
export async function getComments(
  noteId: number,
  limit = 20
): Promise<CommentList> {
  return apiRequest<CommentList>(`/api/notes/${noteId}/comments?limit=${limit}`);
}

export async function createComment(
  noteId: number,
  body: string
): Promise<Comment> {
  return apiRequest<Comment>(`/api/notes/${noteId}/comments`, {
    method: "POST",
    body: JSON.stringify({ body }),
  });
}

// Follow
export async function followUser(targetUserId: number): Promise<void> {
  return apiRequest<void>(`/api/users/me/following/${targetUserId}`, {
    method: "PUT",
  });
}

export async function unfollowUser(targetUserId: number): Promise<void> {
  return apiRequest<void>(`/api/users/me/following/${targetUserId}`, {
    method: "DELETE",
  });
}
