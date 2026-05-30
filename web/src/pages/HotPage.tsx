import { useState, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { getHotFeed } from "@/api/feed";
import FeedCard from "@/components/feed/FeedCard";
import FeedSkeleton from "@/components/feed/FeedSkeleton";
import EmptyState from "@/components/common/EmptyState";
import ErrorState from "@/components/common/ErrorState";
import BackToTop from "@/components/layout/BackToTop";
import { useInfiniteScroll } from "@/hooks/useInfiniteScroll";
import type { FeedItem } from "@/types";

export default function HotPage() {
  const [items, setItems] = useState<FeedItem[]>([]);
  const [cursor, setCursor] = useState("");
  const [hasMore, setHasMore] = useState(true);

  const { isLoading, isError, refetch } = useQuery({
    queryKey: ["hot-feed", cursor],
    queryFn: async () => {
      const result = await getHotFeed(cursor, 20);
      setItems((prev) => (cursor === "" ? result.items : [...prev, ...result.items]));
      setCursor(result.next_cursor || "");
      setHasMore(result.has_more);
      return result;
    },
  });

  const loadMore = useCallback(() => {
    if (hasMore && !isLoading) refetch();
  }, [hasMore, isLoading, refetch]);

  const sentinelRef = useInfiniteScroll(loadMore, hasMore, isLoading);

  return (
    <div className="page-enter">
      <h2 className="mb-4 text-lg font-semibold">🔥 热门笔记</h2>

      {isLoading && items.length === 0 && <FeedSkeleton />}
      {isError && <ErrorState onRetry={() => refetch()} />}
      {!isLoading && !isError && items.length === 0 && <EmptyState />}

      {items.length > 0 && (
        <>
          <div className="masonry-grid">
            {items.map((item) => (
              <FeedCard key={item.id} item={item} />
            ))}
          </div>
          {hasMore && (
            <div ref={sentinelRef} className="py-8 text-center">
              <span className="text-sm text-muted-foreground">加载中…</span>
            </div>
          )}
        </>
      )}
      <BackToTop />
    </div>
  );
}
