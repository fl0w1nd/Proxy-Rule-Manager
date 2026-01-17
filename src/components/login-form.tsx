"use client";

import { useState } from "react";
import { useAuth } from "./auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Lock, Loader2, ArrowLeft } from "lucide-react";

interface LoginFormProps {
  onBack?: () => void;
}

export function LoginForm({ onBack }: LoginFormProps) {
  const { login } = useAuth();
  const [token, setToken] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    const success = await login(token);
    
    if (!success) {
      setError("无效的令牌");
    }
    
    setIsLoading(false);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-50 to-gray-100 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900">
      <Card className="w-full max-w-md mx-4 bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700 shadow-xl">
        <CardHeader className="text-center">
          {onBack && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onBack}
              className="absolute left-4 top-4 text-gray-500"
            >
              <ArrowLeft className="w-4 h-4 mr-1" />
              返回
            </Button>
          )}
          <div className="mx-auto w-12 h-12 bg-gradient-to-br from-blue-500 to-cyan-500 rounded-xl flex items-center justify-center mb-4">
            <Lock className="w-6 h-6 text-white" />
          </div>
          <CardTitle className="text-2xl font-bold text-gray-900 dark:text-white">
            管理后台
          </CardTitle>
          <CardDescription className="text-gray-500 dark:text-gray-400">
            请输入管理员令牌以继续
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="token" className="text-gray-700 dark:text-gray-300">
                管理员令牌
              </Label>
              <Input
                id="token"
                type="password"
                placeholder="输入 ADMIN_TOKEN"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="bg-gray-50 dark:bg-slate-900 border-gray-200 dark:border-slate-600"
              />
              {error && (
                <p className="text-sm text-red-500">{error}</p>
              )}
            </div>
            <Button
              type="submit"
              className="w-full bg-gradient-to-r from-blue-500 to-cyan-500 hover:from-blue-600 hover:to-cyan-600 text-white"
              disabled={isLoading || !token}
            >
              {isLoading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  验证中...
                </>
              ) : (
                "登录"
              )}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
