import { Link, useLocation } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { Home, Flame, PenSquare, User } from "lucide-react";
import { cn } from "@/lib/utils";

export default function MobileNav() {
  const { user } = useAuth();
  const location = useLocation();

  const links = [
    { path: "/", label: "发现", icon: Home },
    { path: "/hot", label: "热门", icon: Flame },
    { path: "/publish", label: "发布", icon: PenSquare, auth: true },
    { path: user ? "/me" : "/login", label: user ? "我的" : "登录", icon: User },
  ];

  return (
    <nav className="fixed bottom-0 left-0 right-0 z-40 border-t bg-white md:hidden">
      <div className="flex items-center justify-around py-2">
        {links.map((link) => {
          if (link.auth && !user) return null;
          const active = location.pathname === link.path;
          return (
            <Link
              key={link.path}
              to={link.path}
              className={cn(
                "flex flex-col items-center gap-0.5 text-xs no-underline transition-colors",
                active ? "text-primary" : "text-muted-foreground hover:text-foreground"
              )}
            >
              <link.icon className="h-5 w-5" />
              <span>{link.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
