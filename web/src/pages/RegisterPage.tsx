import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "@/context/AuthContext";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { UserPlus } from "lucide-react";
import { ApiError } from "@/api/client";

export default function RegisterPage() {
  const { register } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [nickname, setNickname] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await register(email, password, nickname);
      navigate("/login");
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("注册失败，请重试");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="auth-bg page-enter flex min-h-[60vh] items-center justify-center px-4">
      <Card className="w-full max-w-sm border-gray-100 shadow-xl shadow-amber-500/5">
        <CardHeader className="text-center">
          <div className="mx-auto mb-3 flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-amber-500 to-orange-400 shadow-lg shadow-amber-200/50">
            <span className="text-2xl">🫧</span>
          </div>
          <CardTitle className="text-xl">注册知涟</CardTitle>
          <p className="text-sm text-muted-foreground">创建账号，开始记录灵感</p>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <div className="space-y-2">
              <Label htmlFor="nickname">昵称</Label>
              <Input
                id="nickname"
                placeholder="你的昵称"
                value={nickname}
                onChange={(e) => setNickname(e.target.value)}
                required
                className="transition-all duration-200 focus:ring-2 focus:ring-amber-200"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">邮箱</Label>
              <Input
                id="email"
                type="email"
                placeholder="your@email.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="transition-all duration-200 focus:ring-2 focus:ring-amber-200"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">密码</Label>
              <Input
                id="password"
                type="password"
                placeholder="设置密码"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={6}
                className="transition-all duration-200 focus:ring-2 focus:ring-amber-200"
              />
            </div>
            <Button
              type="submit"
              className="btn-press w-full gap-2 bg-gradient-to-r from-amber-500 to-orange-400 shadow-md shadow-amber-200/50 transition-all duration-200 hover:shadow-lg hover:shadow-amber-200/60 hover:brightness-105"
              disabled={loading}
            >
              <UserPlus className="h-4 w-4" />
              {loading ? "注册中…" : "注册"}
            </Button>
          </form>
          <p className="mt-6 text-center text-sm text-muted-foreground">
            已有账号？{" "}
            <Link to="/login" className="text-amber-700 hover:text-amber-800 hover:underline font-medium transition-colors duration-200">
              登录
            </Link>
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
