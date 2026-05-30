import { useState, useEffect } from "react";
import { ArrowUp } from "lucide-react";
import { cn } from "@/lib/utils";

export default function BackToTop() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const onScroll = () => setVisible(window.scrollY > 400);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <button
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      className={cn(
        "scroll-top-btn fixed bottom-24 right-4 z-50 flex h-10 w-10 items-center justify-center rounded-full border bg-white shadow-md transition-all md:bottom-8",
        visible ? "opacity-100" : "pointer-events-none opacity-0 translate-y-4"
      )}
    >
      <ArrowUp className="h-4 w-4" />
    </button>
  );
}
