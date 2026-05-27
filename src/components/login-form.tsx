"use client";

import { useState } from "react";
import { useAuth } from "./auth-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Lock, Loader2, ArrowLeft, ShieldCheck } from "lucide-react";
import { AmbientBackground } from "./ambient-background";

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
    <div className="relative min-h-screen flex flex-col bg-background overflow-hidden">
      <AmbientBackground />

      {/* Top bar */}
      <div className="relative z-10 flex items-center px-6 py-4">
        {onBack && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onBack}
            className="text-muted-foreground"
          >
            <ArrowLeft className="w-4 h-4" />
            返回
          </Button>
        )}
      </div>

      {/* Center content */}
      <div className="relative z-10 flex-1 flex items-center justify-center px-4 -mt-8">
        <div className="w-full max-w-sm animate-slide-up">
          {/* Brand header */}
          <div className="mb-10 text-center">
            <div
              className="mx-auto mb-6 flex h-16 w-16 items-center justify-center rounded-2xl border border-border/60 bg-card shadow-[var(--shadow-sm)]"
              style={{
                backgroundImage:
                  "radial-gradient(120% 80% at 50% 0%, color-mix(in oklch, var(--primary) 16%, transparent) 0%, transparent 70%)",
              }}
            >
              <Lock className="w-6 h-6 text-foreground" strokeWidth={1.75} />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              管理后台
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">
              输入访问令牌以继续
            </p>
          </div>

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-5" noValidate>
            <div className="space-y-2">
              <Label
                htmlFor="token"
                className="text-[11px] font-semibold uppercase tracking-widest text-muted-foreground"
              >
                管理员令牌
              </Label>
              <Input
                id="token"
                type="password"
                placeholder="ADMIN_TOKEN"
                autoComplete="off"
                autoFocus
                value={token}
                onChange={(e) => setToken(e.target.value)}
                aria-invalid={!!error}
                aria-describedby={error ? "token-error" : undefined}
                className="h-11 text-base"
              />
              {error && (
                <p
                  id="token-error"
                  className="text-sm font-medium text-destructive animate-fade-in"
                >
                  {error}
                </p>
              )}
            </div>
            <Button
              type="submit"
              disabled={isLoading || !token}
              className="w-full h-11 text-sm"
            >
              {isLoading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  验证中
                </>
              ) : (
                "登录"
              )}
            </Button>
          </form>

          <div className="mt-10 flex items-center justify-center gap-1.5 text-[11px] text-muted-foreground/70">
            <ShieldCheck className="w-3 h-3" aria-hidden="true" />
            <span>本地会话，仅在此设备上有效</span>
          </div>
        </div>
      </div>
    </div>
  );
}
