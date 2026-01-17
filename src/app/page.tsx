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
  // 从 URL hash 读取初始状态
  const [showAdmin, setShowAdmin] = useState(() => {
    if (typeof window !== "undefined") {
      return window.location.hash === "#admin";
    }
    return false;
  });

  // 监听 hash 变化
  useEffect(() => {
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

  // 如果正在检查认证状态，且用户要进入管理后台
  if (showAdmin && isLoading) {
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
