"use client";

import { createContext, useContext, useState, useEffect, ReactNode } from "react";
import { verifyToken, setToken as saveToken, clearToken, checkAuthRequired } from "@/lib/api-client";

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (token: string) => Promise<boolean>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    async function checkAuth() {
      // 首先检查后端是否需要认证
      const authStatus = await checkAuthRequired();

      if (!authStatus.required) {
        // 后端未设置 ADMIN_TOKEN，无需认证
        setIsAuthenticated(true);
        setIsLoading(false);
        return;
      }

      // 需要认证，检查本地存储的 token
      const token = localStorage.getItem("admin_token");
      if (token) {
        const valid = await verifyToken(token);
        setIsAuthenticated(valid);
        if (!valid) {
          clearToken();
        }
      }
      setIsLoading(false);
    }

    checkAuth();
  }, []);

  const login = async (token: string): Promise<boolean> => {
    const valid = await verifyToken(token);
    if (valid) {
      saveToken(token);
      setIsAuthenticated(true);
    }
    return valid;
  };

  const logout = () => {
    clearToken();
    setIsAuthenticated(false);
  };

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
