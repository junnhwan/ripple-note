import { Link } from "react-router-dom";
import { MessageCircle } from "lucide-react";
import LikeButton from "@/components/interaction/LikeButton";
import FavoriteButton from "@/components/interaction/FavoriteButton";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import type { FeedItem } from "@/types";

interface FeedCardProps {
  item: FeedItem;
}

export default function FeedCard({ item }: FeedCardProps) {
  const coverImage = item.image_urls?.[0];
  const excerpt = item.body?.length > 60 ? item.body.slice(0, 60) + "…" : item.body;

  return (
    <Link to={`/notes/${item.id}`} className="no-underline">
      <div className="group masonry-item cursor-pointer overflow-hidden rounded-2xl border bg-white shadow-sm transition-all hover:shadow-md hover:-translate-y-0.5">
        {/* Cover image */}
        {coverImage ? (
          <div className="relative aspect-[4/3] overflow-hidden bg-muted">
            <img
              src={coverImage}
              alt={item.title}
              loading="lazy"
              className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          </div>
        ) : (
          <div className="flex aspect-[4/3] items-center justify-center bg-gradient-to-br from-cyan-50 to-cyan-100">
            <span className="text-3xl opacity-40">📝</span>
          </div>
        )}

        {/* Content */}
        <div className="p-3">
          <h3 className="line-clamp-2 text-sm font-medium leading-snug text-foreground">
            {item.title}
          </h3>
          {excerpt && (
            <p className="mt-1 text-xs text-muted-foreground line-clamp-2">{excerpt}</p>
          )}

          {/* Tags */}
          {item.tags?.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1">
              {item.tags.slice(0, 3).map((tag) => (
                <span key={tag} className="tag-chip">
                  #{tag}
                </span>
              ))}
            </div>
          )}

          {/* Footer */}
          <div className="mt-2 flex items-center justify-between">
            <div className="flex items-center gap-1.5">
              <Avatar className="h-5 w-5">
                {item.author_avatar ? (
                  <AvatarImage src={item.author_avatar} alt={item.author_nickname} />
                ) : (
                  <AvatarFallback className="text-[10px]">
                    {item.author_nickname?.charAt(0) || "U"}
                  </AvatarFallback>
                )}
              </Avatar>
              <span className="text-xs text-muted-foreground max-w-[80px] truncate">
                {item.author_nickname}
              </span>
            </div>

            <div className="flex items-center gap-3" onClick={(e) => e.stopPropagation()}>
              <LikeButton noteId={item.id} initialCount={item.likes_count} initialLiked={item.viewer_liked === true} />
              <FavoriteButton noteId={item.id} initialCount={item.favorites_count} initialFavorited={item.viewer_favorited === true} />
              <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
                <MessageCircle className="h-3.5 w-3.5" />
                {item.comments_count > 0 ? item.comments_count : ""}
              </span>
            </div>
          </div>
        </div>
      </div>
    </Link>
  );
}
