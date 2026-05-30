import { useState, useCallback } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
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

const FETCHERS: Record<string, (cursor: string, limit: number) => Promise<{ items: FeedItem[]; next_cursor: string; has_more: boolean }>> = {
  latest: getLatestFeed,
  hot: getHotFeed,
  following: getFollowingFeed,
};

function getInitialTab(): FeedTab {
  const params = new URLSearchParams(window.location.search);
  const t = params.get("tab");
  if (t === "hot" || t === "following" || t === "latest") return t;
  return "latest";
}

export default function FeedPage() {
  const { user } = useAuth();
  const [tab, setTab] = useState<FeedTab>(getInitialTab);

  const handleTabChange = useCallback((v: string) => {
    const newTab = v as FeedTab;
    setTab(newTab);
    const url = new URL(window.location.href);
    url.searchParams.set("tab", newTab);
    window.history.replaceState(null, "", url.toString());
  }, []);

  return (
    <div className="page-enter">
      <Tabs value={tab} onValueChange={handleTabChange}>
        <TabsList className="mb-4">
          <TabsTrigger value="latest">最新</TabsTrigger>
          <TabsTrigger value="hot">热门</TabsTrigger>
          {user && <TabsTrigger value="following">关注</TabsTrigger>}
        </TabsList>

        <TabsContent value="latest">
          <FeedList tab="latest" />
        </TabsContent>
        <TabsContent value="hot">
          <FeedList tab="hot" />
        </TabsContent>
        {user && (
          <TabsContent value="following">
            <FeedList tab="following" />
          </TabsContent>
        )}
      </Tabs>
      <BackToTop />
    </div>
  );
}

function FeedList({ tab }: { tab: string }) {
  const fetcher = FETCHERS[tab];

  const {
    data,
    isLoading,
    isError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    refetch,
  } = useInfiniteQuery({
    queryKey: ["feed", tab],
    queryFn: ({ pageParam }) => fetcher(pageParam ?? "", 20),
    initialPageParam: "",
    getNextPageParam: (lastPage) => (lastPage.has_more ? lastPage.next_cursor : undefined),
    enabled: !!fetcher,
  });

  const items: FeedItem[] = data?.pages.flatMap((p) => p.items) ?? [];

  const loadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const sentinelRef = useInfiniteScroll(loadMore, !!hasNextPage, isLoading || isFetchingNextPage);

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
      {hasNextPage && (
        <div ref={sentinelRef} className="py-8 text-center">
          <span className="text-sm text-muted-foreground">
            {isFetchingNextPage ? "加载中…" : ""}
          </span>
        </div>
      )}
    </>
  );
}
