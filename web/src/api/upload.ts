import { uploadFile } from "./client";

export async function uploadImage(file: File): Promise<{ url: string }> {
  return uploadFile("/api/uploads/images", file);
}
