import { apiRequest } from "./client";
import type { Note, NoteList } from "@/types";

export async function getNote(noteId: number): Promise<Note> {
  return apiRequest<Note>(`/api/notes/${noteId}`);
}

export async function getMyNotes(
  cursor = "",
  limit = 20
): Promise<NoteList> {
  const params = new URLSearchParams();
  if (cursor) params.set("cursor", cursor);
  params.set("limit", String(limit));
  return apiRequest<NoteList>(`/api/users/me/notes?${params}`);
}

export async function publishNote(input: {
  title: string;
  body: string;
  image_urls?: string[];
  tags?: string[];
}): Promise<Note> {
  return apiRequest<Note>("/api/notes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}
