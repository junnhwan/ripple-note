import { useState } from "react";
import { Heart } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/context/AuthContext";
import { likeNote, unlikeNote } from "@/api/interaction";
import { toast } from "sonner";

interface LikeButtonProps {
  noteId: number;
  initialCount: number;
  initialLiked?: boolean;
}

export default function LikeButton({ noteId, initialCount, initialLiked = false }: LikeButtonProps) {
  const { user } = useAuth();
  const [liked, setLiked] = useState(initialLiked);
  const [count, setCount] = useState(initialCount);
  const [animating, setAnimating] = useState(false);

  const handleToggle = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (!user) {
      toast.info("请先登录");
      return;
    }
    try {
      if (liked) {
        await unlikeNote(noteId);
        setLiked(false);
        setCount((c) => Math.max(0, c - 1));
      } else {
        await likeNote(noteId);
        setLiked(true);
        setCount((c) => c + 1);
        setAnimating(true);
        setTimeout(() => setAnimating(false), 300);
      }
    } catch {
      toast.error("操作失败，请重试");
    }
  };

  return (
    <button
      onClick={handleToggle}
      className={cn(
        "flex items-center gap-1 text-sm transition-colors outline-none",
        liked ? "text-red-500" : "text-muted-foreground hover:text-red-400"
      )}
    >
      <Heart
        className={cn(
          "h-4 w-4 transition-transform",
          liked && "fill-current",
          animating && "heart-pop"
        )}
      />
      <span>{count > 0 ? count : ""}</span>
    </button>
  );
}
