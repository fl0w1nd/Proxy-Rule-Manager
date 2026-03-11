"use client";

import { lazy, Suspense, useState, useEffect, useSyncExternalStore, useCallback } from "react";
import { ThemeProvider } from "@/components/theme-provider";
import { AuthProvider, useAuth } from "@/components/auth-provider";
import { Loader2 } from "lucide-react";

const PublicRulesPage = lazy(() => import("@/components/home").then(m => ({ default: m.PublicRulesPage })));
const LoginForm = lazy(() => import("@/components/login-form").then(m => ({ default: m.LoginForm })));
const Dashboard = lazy(() => import("@/components/dashboard").then(m => ({ default: m.Dashboard })));

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

  const fallback = (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <Loader2 className="w-8 h-8 animate-spin text-primary" />
    </div>
  );

  // 客户端挂载前或检查认证状态时显示加载
  if (!mounted || (showAdmin && isLoading)) {
    return fallback;
  }

  // 如果要进入管理后台
  if (showAdmin) {
    if (authRequired && !isAuthenticated) {
      return <Suspense fallback={fallback}><LoginForm onBack={exitAdmin} /></Suspense>;
    }
    return <Suspense fallback={fallback}><Dashboard onBack={exitAdmin} /></Suspense>;
  }

  // 默认显示公开规则展示页
  return <Suspense fallback={fallback}><PublicRulesPage onAdminClick={enterAdmin} /></Suspense>;
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
