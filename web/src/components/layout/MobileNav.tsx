import { Link, useLocation } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { Home, PenSquare, User, PlusCircle } from "lucide-react";
import { cn } from "@/lib/utils";

export default function MobileNav() {
  const { user } = useAuth();
  const location = useLocation();

  const links = [
    { path: "/", label: "发现", icon: Home },
    { path: "/publish", label: "发布", icon: PlusCircle, auth: true, special: true },
    { path: user ? "/me" : "/login", label: user ? "我的" : "登录", icon: User },
  ];

  return (
    <nav className="mobile-nav-glass fixed bottom-0 left-0 right-0 z-40 md:hidden">
      <div className="flex items-center justify-around py-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom))]">
        {links.map((link) => {
          if (link.auth && !user) return null;
          const active = location.pathname === link.path;
          const Icon = link.icon;

          /* Center "publish" button with special style */
          if (link.special) {
            return (
              <Link
                key={link.path}
                to={link.path}
                className="-mt-5 flex flex-col items-center no-underline"
              >
                <div className="btn-press flex h-12 w-12 items-center justify-center rounded-2xl bg-gradient-to-br from-amber-500 to-orange-400 shadow-lg shadow-amber-300/40 transition-all duration-200 active:scale-90">
                  <PenSquare className="h-5 w-5 text-white" />
                </div>
              </Link>
            );
          }

          return (
            <Link
              key={link.path}
              to={link.path}
              className={cn(
                "flex flex-col items-center gap-0.5 px-3 py-1 text-xs no-underline transition-all duration-200",
                active
                  ? "text-amber-700"
                  : "text-muted-foreground hover:text-foreground active:scale-95"
              )}
            >
              <div className={cn(
                "flex h-8 w-8 items-center justify-center rounded-xl transition-all duration-200",
                active && "bg-amber-50"
              )}>
                <Icon className={cn("h-5 w-5 transition-transform duration-200", active && "scale-110")} />
              </div>
              <span className={cn("transition-all duration-200", active && "font-medium")}>{link.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
