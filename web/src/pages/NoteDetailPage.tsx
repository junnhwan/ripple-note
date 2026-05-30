import { useState, type FormEvent } from "react";
import { useParams, Link } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { getNote } from "@/api/notes";
import { getComments, createComment } from "@/api/interaction";
import { useAuth } from "@/context/AuthContext";
import LikeButton from "@/components/interaction/LikeButton";
import FavoriteButton from "@/components/interaction/FavoriteButton";
import FollowButton from "@/components/interaction/FollowButton";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Card } from "@/components/ui/card";
import { Send, MessageCircle, Calendar, ArrowLeft } from "lucide-react";
import { toast } from "sonner";
import type { Note } from "@/types";

export default function NoteDetailPage() {
  const { noteId } = useParams<{ noteId: string }>();
  const id = Number(noteId);

  const {
    data: note,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ["note", id],
    queryFn: () => getNote(id),
    enabled: !!id,
  });

  if (isLoading) return <NoteDetailSkeleton />;
  if (isError || !note) return <ErrorState message="笔记不存在或加载失败" onRetry={() => refetch()} />;

  return (
    <div className="page-enter mx-auto max-w-3xl">
      <NoteContent note={note} />
      <CommentSection noteId={id} authorId={note.author.id} />
    </div>
  );
}

function NoteContent({ note }: { note: Note }) {
  const statusMap: Record<string, { label: string; variant: "default" | "secondary" | "destructive" | "outline" }> = {
    pending_review: { label: "审核中", variant: "secondary" },
    published: { label: "已发布", variant: "default" },
    rejected: { label: "已拒绝", variant: "destructive" },
  };
  const status = statusMap[note.status] ?? { label: note.status, variant: "outline" };

  return (
    <div>
      {/* Back button */}
      <Link to="/" className="mb-5 inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-sm text-muted-foreground no-underline transition-all duration-200 hover:bg-amber-50 hover:text-amber-800">
        <ArrowLeft className="h-4 w-4" />
        返回首页
      </Link>

      {/* Images */}
      {note.images?.length > 0 && (
        <div className="mb-6 space-y-3">
          {note.images.map((img) => (
            <div key={img.id} className="overflow-hidden rounded-2xl shadow-sm">
              <img
                src={img.url}
                alt={note.title}
                loading="lazy"
                className="w-full object-cover max-h-[500px] transition-transform duration-500 hover:scale-[1.02]"
              />
            </div>
          ))}
        </div>
      )}

      {/* Title + status */}
      <div className="flex items-start gap-2">
        <h1 className="flex-1 text-xl font-bold leading-tight">{note.title}</h1>
        <Badge variant={status.variant}>{status.label}</Badge>
      </div>

      {/* Author */}
      <div className="mt-4 flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <Avatar className="h-9 w-9 ring-2 ring-amber-50 shadow-sm">
            {note.author.avatar_url ? (
              <AvatarImage src={note.author.avatar_url} alt={note.author.nickname} />
            ) : (
              <AvatarFallback className="bg-gradient-to-br from-amber-100 to-orange-100 text-amber-700">
                {note.author.nickname?.charAt(0) || "U"}
              </AvatarFallback>
            )}
          </Avatar>
          <span className="text-sm font-medium">{note.author.nickname}</span>
          <FollowButton targetUserId={note.author.id} />
        </div>
        {note.published_at && (
          <span className="flex items-center gap-1 text-xs text-muted-foreground">
            <Calendar className="h-3 w-3" />
            {new Date(note.published_at).toLocaleDateString("zh-CN")}
          </span>
        )}
      </div>

      {/* Tags */}
      {note.tags?.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-1.5">
          {note.tags.map((tag) => (
            <Link key={tag} to={`/?tag=${tag}`} className="tag-chip no-underline">
              #{tag}
            </Link>
          ))}
        </div>
      )}

      {/* Body */}
      <div className="mt-6 whitespace-pre-wrap text-sm leading-relaxed text-foreground">
        {note.body}
      </div>

      {/* Interaction bar */}
      <div className="mt-8 flex items-center gap-4 border-t border-amber-100 pt-5">
        <LikeButton noteId={note.id} initialCount={note.likes_count} />
        <FavoriteButton noteId={note.id} initialCount={note.favorites_count} />
        <span className="flex items-center gap-1 text-sm text-muted-foreground">
          <MessageCircle className="h-4 w-4" />
          {note.comments_count} 条评论
        </span>
      </div>
    </div>
  );
}

function CommentSection({ noteId, authorId }: { noteId: number; authorId: number }) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [commentBody, setCommentBody] = useState("");

  const { data: commentList, isLoading } = useQuery({
    queryKey: ["comments", noteId],
    queryFn: () => getComments(noteId),
  });

  const createMutation = useMutation({
    mutationFn: () => createComment(noteId, commentBody.trim()),
    onSuccess: () => {
      setCommentBody("");
      queryClient.invalidateQueries({ queryKey: ["comments", noteId] });
      queryClient.invalidateQueries({ queryKey: ["note", noteId] });
      toast.success("评论成功");
    },
    onError: () => {
      toast.error("评论失败");
    },
  });

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (!commentBody.trim()) return;
    createMutation.mutate();
  };

  return (
    <div className="mt-8">
      <h3 className="mb-4 flex items-center gap-2 text-base font-semibold">
        <MessageCircle className="h-5 w-5 text-amber-500" />
        评论区
      </h3>

      {/* Comment input */}
      {user ? (
        <form onSubmit={handleSubmit} className="mb-6 flex gap-2">
          <Textarea
            placeholder="写下你的评论…"
            value={commentBody}
            onChange={(e) => setCommentBody(e.target.value)}
            rows={2}
            className="flex-1 transition-all duration-200 focus:ring-2 focus:ring-amber-200"
          />
          <Button
            type="submit"
            size="icon"
            disabled={createMutation.isPending || !commentBody.trim()}
            className="btn-press self-end rounded-xl bg-gradient-to-r from-amber-500 to-orange-400 shadow-md shadow-amber-200/50 transition-all duration-200 hover:brightness-105"
          >
            <Send className="h-4 w-4" />
          </Button>
        </form>
      ) : (
        <Card className="mb-6 p-4 text-center text-sm text-muted-foreground border-gray-100 shadow-sm">
          <Link to="/login" className="text-amber-700 hover:text-amber-800 font-medium hover:underline transition-colors duration-200">登录</Link> 后可以评论
        </Card>
      )}

      {/* Comment list */}
      {isLoading && (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="flex gap-3">
              <div className="skeleton-shimmer h-8 w-8 shrink-0 rounded-full" />
              <div className="flex-1 space-y-1.5">
                <div className="skeleton-shimmer h-3 w-20 rounded-md" />
                <div className="skeleton-shimmer h-3 w-full rounded-md" />
              </div>
            </div>
          ))}
        </div>
      )}

      {commentList?.items?.length === 0 && (
        <p className="py-10 text-center text-sm text-muted-foreground">暂无评论，来做第一个吧</p>
      )}

      <div className="space-y-4">
        {commentList?.items?.map((comment) => (
          <div key={comment.id} className="flex gap-3 rounded-xl p-2 transition-colors duration-200 hover:bg-amber-50/50">
            <Avatar className="h-8 w-8 shrink-0 ring-1 ring-amber-100">
              <AvatarFallback className="bg-gradient-to-br from-amber-100 to-orange-100 text-xs text-amber-700">
                {comment.author_nickname?.charAt(0) || "U"}
              </AvatarFallback>
            </Avatar>
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">{comment.author_nickname}</span>
                <span className="text-xs text-muted-foreground">
                  {new Date(comment.created_at).toLocaleDateString("zh-CN")}
                </span>
              </div>
              <p className="mt-1 text-sm text-foreground">{comment.body}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Author info shown for context */}
      <span className="hidden">{authorId}</span>
    </div>
  );
}

function NoteDetailSkeleton() {
  return (
    <div className="mx-auto max-w-3xl space-y-4">
      <div className="skeleton-shimmer h-4 w-20 rounded-lg" />
      <div className="skeleton-shimmer h-64 w-full rounded-2xl" />
      <div className="skeleton-shimmer h-7 w-3/4 rounded-lg" />
      <div className="flex items-center gap-2">
        <div className="skeleton-shimmer h-9 w-9 rounded-full" />
        <div className="skeleton-shimmer h-4 w-24 rounded-lg" />
      </div>
      <div className="space-y-2 pt-4">
        <div className="skeleton-shimmer h-4 w-full rounded-lg" />
        <div className="skeleton-shimmer h-4 w-full rounded-lg" />
        <div className="skeleton-shimmer h-4 w-2/3 rounded-lg" />
      </div>
    </div>
  );
}
