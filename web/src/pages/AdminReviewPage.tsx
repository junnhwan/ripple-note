import { useState, type FormEvent } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/context/AuthContext";
import { getReviewTasks, submitDecision } from "@/api/review";
import { getNote } from "@/api/notes";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import ErrorState from "@/components/common/ErrorState";
import { Shield, CheckCircle, XCircle, Clock, Eye, AlertTriangle, FileText } from "lucide-react";
import { toast } from "sonner";
import { Link } from "react-router-dom";
import type { ReviewTask } from "@/types";

export default function AdminReviewPage() {
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const {
    data: taskList,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["review-tasks"],
    queryFn: () => getReviewTasks(50),
  });

  const decisionMutation = useMutation({
    mutationFn: ({ taskId, decision, reason }: { taskId: number; decision: "approve" | "reject"; reason: string }) =>
      submitDecision(taskId, decision, reason),
    onSuccess: () => {
      toast.success("审核操作成功");
      queryClient.invalidateQueries({ queryKey: ["review-tasks"] });
    },
    onError: () => {
      toast.error("审核操作失败");
    },
  });

  if (!user || user.role !== "admin") {
    return (
      <div className="page-enter flex min-h-[40vh] items-center justify-center">
        <div className="text-center">
          <Shield className="mx-auto h-12 w-12 text-muted-foreground/30" />
          <p className="mt-2 text-muted-foreground">需要管理员权限</p>
        </div>
      </div>
    );
  }

  return (
    <div className="page-enter mx-auto max-w-4xl">
      <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold">
        <Shield className="h-5 w-5 text-primary" />
        内容审核
      </h2>

      {isLoading && (
        <div className="space-y-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24 w-full rounded-xl" />
          ))}
        </div>
      )}

      {isError && <ErrorState onRetry={() => refetch()} />}

      {taskList?.items?.length === 0 && (
        <div className="py-20 text-center text-muted-foreground">暂无待审核任务</div>
      )}

      <div className="space-y-3">
        {taskList?.items?.map((task) => (
          <TaskRow
            key={task.id}
            task={task}
            onDecision={(taskId, decision, reason) =>
              decisionMutation.mutate({ taskId, decision, reason })
            }
            deciding={decisionMutation.isPending}
          />
        ))}
      </div>
    </div>
  );
}

function TaskRow({
  task,
  onDecision,
  deciding,
}: {
  task: ReviewTask;
  onDecision: (taskId: number, decision: "approve" | "reject", reason: string) => void;
  deciding: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");

  const statusMap: Record<string, { icon: typeof Clock; label: string; color: string }> = {
    pending_agent: { icon: Clock, label: "待审核", color: "text-amber-500" },
    agent_passed: { icon: CheckCircle, label: "Agent 通过", color: "text-green-500" },
    agent_rejected: { icon: XCircle, label: "Agent 拒绝", color: "text-red-500" },
    manual_required: { icon: AlertTriangle, label: "需人工审核", color: "text-orange-500" },
    admin_approved: { icon: CheckCircle, label: "已通过", color: "text-green-500" },
    admin_rejected: { icon: XCircle, label: "已拒绝", color: "text-red-500" },
  };
  const status = statusMap[task.status] ?? { icon: FileText, label: task.status, color: "text-muted-foreground" };
  const StatusIcon = status.icon;

  const needsAction = ["pending_agent", "agent_passed", "agent_rejected", "manual_required"].includes(task.status);

  const handleDecision = (e: FormEvent, decision: "approve" | "reject") => {
    e.preventDefault();
    onDecision(task.id, decision, reason);
    setOpen(false);
    setReason("");
  };

  return (
    <div className="flex items-start gap-3 rounded-xl border bg-white p-4">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <StatusIcon className={`h-4 w-4 ${status.color}`} />
          <Badge variant="outline" className={status.color}>
            {status.label}
          </Badge>
          <Link
            to={`/notes/${task.note_id}`}
            className="text-sm text-primary hover:underline"
          >
            笔记 #{task.note_id}
          </Link>
        </div>

        {task.agent_decision && (
          <p className="mt-2 text-xs text-muted-foreground">
            Agent 决定: <span className="font-medium">{task.agent_decision}</span>
            {task.agent_risk_level && ` · 风险: ${task.agent_risk_level}`}
            {task.agent_reason && ` · ${task.agent_reason}`}
          </p>
        )}

        {task.admin_decision && (
          <p className="mt-1 text-xs text-muted-foreground">
            管理员决定: <span className="font-medium">{task.admin_decision}</span>
            {task.admin_reason && ` · ${task.admin_reason}`}
          </p>
        )}

        <p className="mt-1 text-xs text-muted-foreground">
          {new Date(task.created_at).toLocaleString("zh-CN")}
        </p>
      </div>

      {needsAction && (
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" size="sm" className="gap-1">
              <Eye className="h-3.5 w-3.5" />
              审核
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>审核笔记 #{task.note_id}</DialogTitle>
            </DialogHeader>
            <NotePreview noteId={task.note_id} />
            <form className="mt-4 space-y-3">
              <div className="space-y-2">
                <Label>审核理由</Label>
                <Textarea
                  placeholder="填写审核理由（可选）"
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  rows={3}
                />
              </div>
              <div className="flex justify-end gap-2">
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={(e) => handleDecision(e, "reject")}
                  disabled={deciding}
                  className="gap-1"
                >
                  <XCircle className="h-4 w-4" />
                  拒绝
                </Button>
                <Button
                  size="sm"
                  onClick={(e) => handleDecision(e, "approve")}
                  disabled={deciding}
                  className="gap-1"
                >
                  <CheckCircle className="h-4 w-4" />
                  通过
                </Button>
              </div>
            </form>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

function NotePreview({ noteId }: { noteId: number }) {
  const { data: note, isLoading } = useQuery({
    queryKey: ["note", noteId],
    queryFn: () => getNote(noteId),
  });

  if (isLoading) return <Skeleton className="h-24 w-full" />;
  if (!note) return <p className="text-sm text-muted-foreground">加载笔记失败</p>;

  return (
    <div className="rounded-lg border p-3">
      <h4 className="font-medium">{note.title}</h4>
      {note.images?.[0] && (
        <img src={note.images[0].url} alt="" className="mt-2 max-h-40 rounded-lg object-cover" />
      )}
      <p className="mt-2 text-sm text-muted-foreground line-clamp-2">{note.body}</p>
      <div className="mt-1 text-xs text-muted-foreground">
        作者: {note.author.nickname}
      </div>
    </div>
  );
}
