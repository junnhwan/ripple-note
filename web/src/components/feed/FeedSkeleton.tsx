export default function FeedSkeleton({ count = 6 }: { count?: number }) {
  return (
    <div className="masonry-grid">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="masonry-item">
          <div className="overflow-hidden rounded-2xl border border-gray-100 bg-white">
            {/* Image placeholder */}
            <div className="skeleton-shimmer aspect-[4/3] w-full rounded-none" />
            {/* Text placeholders */}
            <div className="space-y-2.5 p-3.5">
              <div className="skeleton-shimmer h-4 w-4/5 rounded-md" />
              <div className="skeleton-shimmer h-3 w-3/5 rounded-md" />
              <div className="flex items-center justify-between pt-1">
                <div className="flex items-center gap-1.5">
                  <div className="skeleton-shimmer h-5 w-5 rounded-full" />
                  <div className="skeleton-shimmer h-3 w-12 rounded-md" />
                </div>
                <div className="skeleton-shimmer h-4 w-16 rounded-md" />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
