import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { getMyNotes } from "@/api/notes";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import EmptyState from "@/components/common/EmptyState";
import { FileText, Clock, XCircle, CheckCircle, Sparkles } from "lucide-react";
import type { Note } from "@/types";

export default function ProfilePage() {
  const { user } = useAuth();

  const { data: noteList, isLoading } = useQuery({
    queryKey: ["my-notes"],
    queryFn: () => getMyNotes(),
  });

  if (!user) {
    return (
      <div className="page-enter flex min-h-[40vh] items-center justify-center">
        <p className="text-muted-foreground">
          请先 <Link to="/login" className="text-primary hover:underline">登录</Link>
        </p>
      </div>
    );
  }

  return (
    <div className="page-enter mx-auto max-w-3xl">
      {/* Profile header */}
      <div className="relative overflow-hidden rounded-2xl border border-gray-100 bg-white p-6 shadow-sm">
        {/* Decorative gradient */}
        <div className="absolute -top-10 -right-10 h-32 w-32 rounded-full bg-gradient-to-br from-amber-100/60 to-orange-100/40" />

        <div className="relative flex items-center gap-4">
          <Avatar className="h-16 w-16 ring-4 ring-amber-50 shadow-md">
            {user.avatar_url ? (
              <AvatarImage src={user.avatar_url} alt={user.nickname} />
            ) : (
              <AvatarFallback className="bg-gradient-to-br from-amber-100 to-orange-100 text-xl text-amber-700">
                {user.nickname?.charAt(0) || "U"}
              </AvatarFallback>
            )}
          </Avatar>
          <div className="flex-1">
            <h2 className="text-lg font-bold">{user.nickname}</h2>
            <p className="text-sm text-muted-foreground">{user.email}</p>
            <div className="mt-1 flex items-center gap-2">
              {user.role === "admin" && (
                <Badge variant="secondary" className="bg-amber-50 text-amber-700">管理员</Badge>
              )}
              <span className="text-xs text-muted-foreground">
                加入于 {new Date(user.created_at).toLocaleDateString("zh-CN")}
              </span>
            </div>
          </div>
          <Link to="/publish">
            <Button size="sm" className="btn-press gap-1.5 rounded-lg bg-gradient-to-r from-amber-500 to-orange-400 shadow-md shadow-amber-200/50">
              <Sparkles className="h-4 w-4" />
              发布笔记
            </Button>
          </Link>
        </div>
      </div>

      {/* Notes list */}
      <h3 className="mb-3 mt-6 text-base font-semibold">我的笔记</h3>

      {isLoading && (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton-shimmer h-20 w-full rounded-xl" />
          ))}
        </div>
      )}

      {noteList?.items?.length === 0 && (
        <EmptyState title="还没有笔记" description="发布你的第一篇笔记吧" />
      )}

      <div className="space-y-3">
        {noteList?.items?.map((note: Note) => (
          <NoteRow key={note.id} note={note} />
        ))}
      </div>
    </div>
  );
}

function NoteRow({ note }: { note: Note }) {
  const statusConfig: Record<string, { icon: typeof FileText; label: string; color: string }> = {
    pending_review: { icon: Clock, label: "审核中", color: "text-amber-500" },
    published: { icon: CheckCircle, label: "已发布", color: "text-green-500" },
    rejected: { icon: XCircle, label: "已拒绝", color: "text-red-500" },
  };
  const status = statusConfig[note.status] ?? { icon: FileText, label: note.status, color: "text-muted-foreground" };
  const StatusIcon = status.icon;

  return (
    <Link to={`/notes/${note.id}`} className="no-underline">
      <div className="flex items-start gap-3 rounded-xl border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:border-amber-200/60 hover:shadow-md hover:-translate-y-0.5">
        {note.images?.[0] && (
          <img
            src={note.images[0].url}
            alt=""
            loading="lazy"
            className="h-16 w-16 shrink-0 rounded-lg object-cover shadow-sm"
          />
        )}
        <div className="flex-1 min-w-0">
          <h4 className="truncate text-sm font-medium">{note.title}</h4>
          <p className="mt-1 truncate text-xs text-muted-foreground">
            {note.body?.slice(0, 80) || "无正文"}
          </p>
          <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
            <span className={`flex items-center gap-1 ${status.color}`}>
              <StatusIcon className="h-3 w-3" />
              {status.label}
            </span>
            <span>{new Date(note.created_at).toLocaleDateString("zh-CN")}</span>
            {note.tags?.length > 0 && (
              <span className="truncate">
                {note.tags.map((t) => `#${t}`).join(" ")}
              </span>
            )}
          </div>
        </div>
      </div>
    </Link>
  );
}
