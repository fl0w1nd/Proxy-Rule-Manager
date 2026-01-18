"use client";

import { useState, useEffect } from "react";
import { ThemeProvider } from "@/components/theme-provider";
import { AuthProvider, useAuth } from "@/components/auth-provider";
import { PublicRulesPage } from "@/components/public-rules-page";
import { LoginForm } from "@/components/login-form";
import { Dashboard } from "@/components/dashboard";
import { Loader2 } from "lucide-react";

function AppContent() {
  const { isAuthenticated, isLoading, authRequired } = useAuth();
  // 初始状态设为 false，避免 hydration 不匹配
  const [showAdmin, setShowAdmin] = useState(false);
  const [mounted, setMounted] = useState(false);

  // 客户端挂载后读取 hash 并监听变化
  useEffect(() => {
    setMounted(true);
    // 初始化时读取 hash
    setShowAdmin(window.location.hash === "#admin");
    
    const handleHashChange = () => {
      setShowAdmin(window.location.hash === "#admin");
    };
    window.addEventListener("hashchange", handleHashChange);
    return () => window.removeEventListener("hashchange", handleHashChange);
  }, []);

  // 更新 hash
  const enterAdmin = () => {
    window.location.hash = "#admin";
    setShowAdmin(true);
  };

  const exitAdmin = () => {
    window.location.hash = "";
    setShowAdmin(false);
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
