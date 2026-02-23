"use client";

import { useState, useEffect, useSyncExternalStore, useCallback } from "react";
import { ThemeProvider } from "@/components/theme-provider";
import { AuthProvider, useAuth } from "@/components/auth-provider";
import { PublicRulesPage } from "@/components/home";
import { LoginForm } from "@/components/login-form";
import { Dashboard } from "@/components/dashboard";
import { Loader2 } from "lucide-react";

// 使用 useSyncExternalStore 订阅 hash 变化
function useHash() {
  const subscribe = useCallback((callback: () => void) => {
    window.addEventListener("hashchange", callback);
    return () => window.removeEventListener("hashchange", callback);
  }, []);

  const getSnapshot = useCallback(() => {
    return typeof window !== "undefined" ? window.location.hash : "";
  }, []);

  const getServerSnapshot = useCallback(() => "", []);

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

function AppContent() {
  const { isAuthenticated, isLoading, authRequired } = useAuth();
  const hash = useHash();
  const showAdmin = hash === "#admin";
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMounted(true);
  }, []);

  // 更新 hash
  const enterAdmin = () => {
    window.location.hash = "#admin";
  };

  const exitAdmin = () => {
    window.location.hash = "";
  };

  // 客户端挂载前或检查认证状态时显示加载
  if (!mounted || (showAdmin && isLoading)) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-slate-900">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  // 如果要进入管理后台
  if (showAdmin) {
    if (authRequired && !isAuthenticated) {
      return <LoginForm onBack={exitAdmin} />;
    }
    return <Dashboard onBack={exitAdmin} />;
  }

  // 默认显示公开规则展示页
  return <PublicRulesPage onAdminClick={enterAdmin} />;
}

export default function Home() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <AppContent />
      </AuthProvider>
    </ThemeProvider>
  );
}
