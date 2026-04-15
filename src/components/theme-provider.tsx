"use client";

import { createContext, useContext, useEffect, useState, useCallback, ReactNode } from "react";

type Mode = "light" | "dark";
type Brand = "wise" | "linear" | "vercel" | "stripe" | "supabase";

export type { Brand };

export const BRAND_LIST: { id: Brand; label: string; accent: string }[] = [
  { id: "wise", label: "Wise", accent: "#9fe870" },
  { id: "linear", label: "Linear", accent: "#5e6ad2" },
  { id: "vercel", label: "Vercel", accent: "#171717" },
  { id: "stripe", label: "Stripe", accent: "#533afd" },
  { id: "supabase", label: "Supabase", accent: "#3ecf8e" },
];

interface ThemeContextType {
  mode: Mode;
  brand: Brand;
  toggleMode: () => void;
  setMode: (mode: Mode) => void;
  setBrand: (brand: Brand) => void;
}

const ThemeContext = createContext<ThemeContextType>({
  mode: "light",
  brand: "wise",
  toggleMode: () => {},
  setMode: () => {},
  setBrand: () => {},
});

function applyTheme(mode: Mode, brand: Brand) {
  const root = document.documentElement;
  root.classList.toggle("dark", mode === "dark");
  root.setAttribute("data-theme", brand);
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<Mode>(() => {
    if (typeof window === "undefined") return "light";
    return (localStorage.getItem("theme-mode") as Mode) || "light";
  });

  const [brand, setBrandState] = useState<Brand>(() => {
    if (typeof window === "undefined") return "wise";
    return (localStorage.getItem("theme-brand") as Brand) || "wise";
  });

  useEffect(() => {
    applyTheme(mode, brand);
  }, [mode, brand]);

  const setMode = useCallback((m: Mode) => {
    setModeState(m);
    localStorage.setItem("theme-mode", m);
  }, []);

  const toggleMode = useCallback(() => {
    setModeState((prev) => {
      const next = prev === "light" ? "dark" : "light";
      localStorage.setItem("theme-mode", next);
      return next;
    });
  }, []);

  const setBrand = useCallback((b: Brand) => {
    setBrandState(b);
    localStorage.setItem("theme-brand", b);
  }, []);

  const value: ThemeContextType = { mode, brand, toggleMode, setMode, setBrand };

  return (
    <ThemeContext.Provider value={value}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  return useContext(ThemeContext);
}
