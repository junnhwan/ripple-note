import { Link, useNavigate, useLocation } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { PenSquare, User, LogOut, Shield, Compass, Sparkles } from "lucide-react";
import { cn } from "@/lib/utils";

export default function Navbar() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const isActive = (path: string) => location.pathname === path;

  return (
    <header className="navbar-glass sticky top-0 z-40 w-full border-b border-transparent">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        {/* Logo */}
        <Link to="/" className="flex items-center gap-2 no-underline group">
          <span className="flex h-8 w-8 items-center justify-center rounded-xl bg-gradient-to-br from-amber-500 to-orange-400 text-sm text-white shadow-md shadow-amber-200 transition-transform duration-200 group-hover:scale-105 group-active:scale-95">
            🫧
          </span>
          <span className="bg-gradient-to-r from-amber-700 to-orange-600 bg-clip-text text-lg font-bold text-transparent">
            知涟
          </span>
        </Link>

        {/* Desktop nav links */}
        <nav className="hidden items-center gap-1 md:flex">
          <Link to="/">
            <Button
              variant={isActive("/") ? "secondary" : "ghost"}
              size="sm"
              className={cn(
                "gap-1.5 rounded-lg transition-all duration-200",
                isActive("/") && "bg-amber-50 text-amber-800 hover:bg-amber-100"
              )}
            >
              <Compass className="h-4 w-4" />
              发现
            </Button>
          </Link>
        </nav>

        {/* Right side */}
        <div className="flex items-center gap-2">
          {user ? (
            <>
              <Link to="/publish">
                <Button
                  size="sm"
                  className="btn-press gap-1.5 rounded-lg bg-gradient-to-r from-amber-500 to-orange-400 shadow-md shadow-amber-200/50 transition-all duration-200 hover:shadow-lg hover:shadow-amber-200/60 hover:brightness-105"
                >
                  <Sparkles className="h-4 w-4" />
                  <span className="hidden sm:inline">发布</span>
                </Button>
              </Link>

              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button className="relative h-8 w-8 rounded-full outline-none ring-offset-background transition-all duration-200 hover:ring-2 hover:ring-amber-200 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 active:scale-95">
                    <Avatar className="h-8 w-8 ring-2 ring-white shadow-sm transition-shadow duration-200 hover:ring-amber-200">
                      {user.avatar_url ? (
                        <AvatarImage src={user.avatar_url} alt={user.nickname} />
                      ) : (
                        <AvatarFallback className="bg-gradient-to-br from-amber-100 to-orange-100 text-amber-700">
                          {user.nickname?.charAt(0) || "U"}
                        </AvatarFallback>
                      )}
                    </Avatar>
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48 rounded-xl shadow-lg">
                  <div className="px-2 py-1.5">
                    <p className="text-sm font-medium">{user.nickname}</p>
                    <p className="text-xs text-muted-foreground">{user.email}</p>
                  </div>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem onClick={() => navigate("/me")} className="cursor-pointer gap-2 rounded-lg">
                    <User className="h-4 w-4" />
                    个人中心
                  </DropdownMenuItem>
                  {user.role === "admin" && (
                    <DropdownMenuItem onClick={() => navigate("/admin/review")} className="cursor-pointer gap-2 rounded-lg">
                      <Shield className="h-4 w-4" />
                      内容审核
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    onClick={() => {
                      logout();
                      navigate("/");
                    }}
                    className="cursor-pointer gap-2 rounded-lg text-destructive focus:text-destructive"
                  >
                    <LogOut className="h-4 w-4" />
                    退出登录
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          ) : (
            <div className="flex items-center gap-2">
              <Link to="/login">
                <Button variant="ghost" size="sm" className="rounded-lg transition-all duration-200">
                  登录
                </Button>
              </Link>
              <Link to="/register">
                <Button size="sm" className="btn-press rounded-lg bg-gradient-to-r from-amber-500 to-orange-400 shadow-md shadow-amber-200/50">
                  注册
                </Button>
              </Link>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
