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
import { Clock, Flame, Users } from "lucide-react";
import { cn } from "@/lib/utils";
import type { FeedItem } from "@/types";

type FeedTab = "latest" | "hot" | "following";

const TABS = [
  { key: "latest" as FeedTab, label: "最新", icon: Clock },
  { key: "hot" as FeedTab, label: "热门", icon: Flame },
];

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

  const allTabs = user
    ? [...TABS, { key: "following" as FeedTab, label: "关注", icon: Users }]
    : TABS;

  const handleTabChange = useCallback((newTab: FeedTab) => {
    setTab(newTab);
    const url = new URL(window.location.href);
    url.searchParams.set("tab", newTab);
    window.history.replaceState(null, "", url.toString());
  }, []);

  return (
    <div className="page-enter">
      {/* Custom tab bar */}
      <div className="mb-5 flex items-center gap-1 rounded-xl bg-amber-50/80 p-1">
        {allTabs.map(({ key, label, icon: Icon }) => (
          <button
            key={key}
            onClick={() => handleTabChange(key)}
            className={cn(
              "flex flex-1 items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium transition-all duration-300",
              tab === key
                ? "bg-white text-amber-800 shadow-sm"
                : "text-muted-foreground hover:text-foreground hover:bg-white/50"
            )}
          >
            <Icon className={cn("h-4 w-4 transition-transform duration-200", tab === key && "scale-110")} />
            {label}
          </button>
        ))}
      </div>

      {/* Tab content with crossfade */}
      <div className="tab-transition">
        <FeedList tab={tab} />
      </div>
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
