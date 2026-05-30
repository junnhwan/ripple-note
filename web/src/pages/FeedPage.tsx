import { useState, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { getLatestFeed, getHotFeed, getFollowingFeed } from "@/api/feed";
import FeedCard from "@/components/feed/FeedCard";
import FeedSkeleton from "@/components/feed/FeedSkeleton";
import EmptyState from "@/components/common/EmptyState";
import ErrorState from "@/components/common/ErrorState";
import BackToTop from "@/components/layout/BackToTop";
import { useInfiniteScroll } from "@/hooks/useInfiniteScroll";
import { useAuth } from "@/context/AuthContext";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import type { FeedItem } from "@/types";

type FeedTab = "latest" | "hot" | "following";

export default function FeedPage() {
  const { user } = useAuth();
  const [tab, setTab] = useState<FeedTab>("latest");

  return (
    <div className="page-enter">
      <Tabs value={tab} onValueChange={(v) => setTab(v as FeedTab)}>
        <TabsList className="mb-4">
          <TabsTrigger value="latest">最新</TabsTrigger>
          <TabsTrigger value="hot">热门</TabsTrigger>
          {user && <TabsTrigger value="following">关注</TabsTrigger>}
        </TabsList>

        <TabsContent value="latest">
          <FeedList fetcher={getLatestFeed} />
        </TabsContent>
        <TabsContent value="hot">
          <FeedList fetcher={getHotFeed} />
        </TabsContent>
        {user && (
          <TabsContent value="following">
            <FeedList fetcher={getFollowingFeed} />
          </TabsContent>
        )}
      </Tabs>
      <BackToTop />
    </div>
  );
}

function FeedList({
  fetcher,
}: {
  fetcher: (cursor: string, limit: number) => Promise<{ items: FeedItem[]; next_cursor: string; has_more: boolean }>;
}) {
  const [items, setItems] = useState<FeedItem[]>([]);
  const [cursor, setCursor] = useState("");
  const [hasMore, setHasMore] = useState(true);

  const { isLoading, isError, refetch } = useQuery({
    queryKey: ["feed", fetcher, cursor],
    queryFn: async () => {
      const result = await fetcher(cursor, 20);
      setItems((prev) => (cursor === "" ? result.items : [...prev, ...result.items]));
      setCursor(result.next_cursor || "");
      setHasMore(result.has_more);
      return result;
    },
    enabled: true,
  });

  const loadMore = useCallback(() => {
    if (hasMore && !isLoading) {
      refetch();
    }
  }, [hasMore, isLoading, refetch]);

  const sentinelRef = useInfiniteScroll(loadMore, hasMore, isLoading);

  if (isLoading && items.length === 0) return <FeedSkeleton />;

  if (isError) return <ErrorState onRetry={() => refetch()} />;

  if (items.length === 0) return <EmptyState />;

  return (
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
  );
}
