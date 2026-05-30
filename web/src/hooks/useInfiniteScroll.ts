import { useEffect, useRef, useCallback } from "react";

export function useInfiniteScroll(
  onIntersect: () => void,
  hasMore: boolean,
  loading: boolean
) {
  const observerRef = useRef<IntersectionObserver | null>(null);

  const sentinelRef = useCallback(
    (node: HTMLDivElement | null) => {
      if (observerRef.current) observerRef.current.disconnect();
      if (!node || !hasMore || loading) return;

      observerRef.current = new IntersectionObserver(
        (entries) => {
          if (entries[0].isIntersecting && hasMore && !loading) {
            onIntersect();
          }
        },
        { rootMargin: "200px" }
      );

      observerRef.current.observe(node);
    },
    [onIntersect, hasMore, loading]
  );

  useEffect(() => {
    return () => observerRef.current?.disconnect();
  }, []);

  return sentinelRef;
}
