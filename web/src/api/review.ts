import { apiRequest } from "./client";
import type { ReviewTask, ReviewTaskList } from "@/types";

export async function getReviewTasks(
  limit = 20
): Promise<ReviewTaskList> {
  return apiRequest<ReviewTaskList>(`/api/admin/review/tasks?limit=${limit}`);
}

export async function getReviewTask(taskId: number): Promise<ReviewTask> {
  return apiRequest<ReviewTask>(`/api/admin/review/tasks/${taskId}`);
}

export async function submitDecision(
  taskId: number,
  decision: "approve" | "reject",
  reason: string
): Promise<ReviewTask> {
  return apiRequest<ReviewTask>(
    `/api/admin/review/tasks/${taskId}/decision`,
    {
      method: "PUT",
      body: JSON.stringify({ decision, reason }),
    }
  );
}
