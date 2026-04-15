"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Icon } from "@iconify/react";
import { X, Loader2, Search } from "lucide-react";

interface IconPickerProps {
  value?: string;
  onChange: (icon: string | undefined) => void;
}

interface SearchResult {
  icons: string[];
  total: number;
}

export function IconPicker({ value, onChange }: IconPickerProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [results, setResults] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const searchIcons = useCallback(async (query: string) => {
    if (!query.trim()) {
      setResults([]);
      setSearched(false);
      return;
    }

    setLoading(true);
    setSearched(true);
    try {
      const res = await fetch(
        `https://api.iconify.design/search?query=${encodeURIComponent(query)}&limit=100`
      );
      const data: SearchResult = await res.json();
      setResults(data.icons || []);
    } catch {
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (debounceRef.current) {
      clearTimeout(debounceRef.current);
    }
    if (!search.trim()) {
      setResults([]);
      setSearched(false);
      return;
    }
    debounceRef.current = setTimeout(() => {
      searchIcons(search);
    }, 400);
    return () => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current);
      }
    };
  }, [search, searchIcons]);

  const selectIcon = (icon: string) => {
    onChange(icon);
    setOpen(false);
  };

  const clearIcon = () => {
    onChange(undefined);
  };

  return (
    <div className="flex items-center gap-2">
      <Button
        type="button"
        variant="outline"
        className="h-10 w-10 p-0 flex items-center justify-center"
        onClick={() => setOpen(true)}
      >
        {value ? (
          <Icon icon={value} className="w-5 h-5" />
        ) : (
          <Search className="w-4 h-4 text-muted-foreground" />
        )}
      </Button>
      {value && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-8 px-2 text-muted-foreground hover:text-destructive"
          onClick={clearIcon}
        >
          <X className="w-4 h-4" />
        </Button>
      )}
      <span className="text-xs text-muted-foreground truncate max-w-[200px]">
        {value || "未选择图标"}
      </span>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh]">
          <DialogHeader>
            <DialogTitle>选择图标</DialogTitle>
          </DialogHeader>

          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <Input
              placeholder="输入关键词搜索，如 youtube, telegram, cloud..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
              autoFocus
            />
            {loading && (
              <Loader2 className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 animate-spin text-muted-foreground" />
            )}
          </div>

          <ScrollArea className="h-[400px] mt-4">
            {loading ? (
              <div className="flex items-center justify-center h-32">
                <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
              </div>
            ) : results.length > 0 ? (
              <div className="grid grid-cols-8 gap-2">
                {results.map((icon) => (
                  <button
                    key={icon}
                    type="button"
                    onClick={() => selectIcon(icon)}
                    className={`p-3 rounded-lg border hover:bg-accent hover:border-primary transition-colors flex items-center justify-center ${
                      value === icon ? "bg-accent border-primary" : "border-transparent"
                    }`}
                    title={icon}
                  >
                    <Icon icon={icon} className="w-6 h-6" />
                  </button>
                ))}
              </div>
            ) : searched ? (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                未找到图标
              </div>
            ) : (
              <div className="flex items-center justify-center h-32 text-muted-foreground">
                输入关键词搜索图标
              </div>
            )}
          </ScrollArea>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export function RuleIcon({ icon, className = "w-5 h-5" }: { icon?: string; className?: string }) {
  if (!icon) return null;
  return <Icon icon={icon} className={className} />;
}
