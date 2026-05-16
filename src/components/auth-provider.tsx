"use client";

import { createContext, useContext, useState, useEffect, ReactNode } from "react";
import { verifyToken, setToken as saveToken, clearToken, checkAuthRequired } from "@/lib/api-client";

interface AuthContextType {
  isAuthenticated: boolean;
  isLoading: boolean;
  authRequired: boolean;
  login: (token: string) => Promise<boolean>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [authRequired, setAuthRequired] = useState(true);

  useEffect(() => {
    async function checkAuth() {
      try {
        const authStatus = await checkAuthRequired();
        setAuthRequired(authStatus.required);

        if (!authStatus.required) {
          setIsAuthenticated(true);
          return;
        }

        const token = localStorage.getItem("admin_token");
        if (token) {
          const valid = await verifyToken(token);
          setIsAuthenticated(valid);
          if (!valid) {
            clearToken();
          }
        }
      } catch (err) {
        // If the initial probe throws (network down, server 500, etc.) we
        // must not leave the UI stuck in a permanent loading state. Reset
        // to "auth required + unauthenticated" so the login form is shown.
        console.error("auth check failed:", err);
        setAuthRequired(true);
        setIsAuthenticated(false);
      } finally {
        setIsLoading(false);
      }
    }

    checkAuth();
  }, []);

  // React to backend-driven session loss: if any authenticated API call
  // comes back 401/403, api-client dispatches "auth-expired" and we flip
  // back to the login state. We deliberately keep authRequired untouched
  // so the open-access ("no ADMIN_TOKEN") mode is unaffected.
  useEffect(() => {
    if (typeof window === "undefined") return;
    const handler = () => {
      clearToken();
      setIsAuthenticated(false);
      setAuthRequired(true);
    };
    window.addEventListener("auth-expired", handler);
    return () => {
      window.removeEventListener("auth-expired", handler);
    };
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
    if (authRequired) {
      setIsAuthenticated(false);
    } else {
      setIsAuthenticated(true);
    }
  };

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, authRequired, login, logout }}>
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
