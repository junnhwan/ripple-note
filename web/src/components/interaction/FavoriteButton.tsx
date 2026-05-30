import { useState } from "react";
import { Star } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/context/AuthContext";
import { favoriteNote, unfavoriteNote } from "@/api/interaction";
import { toast } from "sonner";

interface FavoriteButtonProps {
  noteId: number;
  initialCount: number;
  initialFavorited?: boolean;
}

export default function FavoriteButton({ noteId, initialCount, initialFavorited = false }: FavoriteButtonProps) {
  const { user } = useAuth();
  const [favorited, setFavorited] = useState(initialFavorited);
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
      if (favorited) {
        await unfavoriteNote(noteId);
        setFavorited(false);
        setCount((c) => Math.max(0, c - 1));
      } else {
        await favoriteNote(noteId);
        setFavorited(true);
        setCount((c) => c + 1);
        setAnimating(true);
        setTimeout(() => setAnimating(false), 350);
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
        favorited ? "text-amber-500" : "text-muted-foreground hover:text-amber-400"
      )}
    >
      <Star
        className={cn(
          "h-4 w-4 transition-transform",
          favorited && "fill-current",
          animating && "star-flash"
        )}
      />
      <span>{count > 0 ? count : ""}</span>
    </button>
  );
}
