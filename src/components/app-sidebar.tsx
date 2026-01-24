"use client";

import {
    Code2,
    FileText,
    History,
    LayoutDashboard,
    LogOut,
    Monitor,
    Settings,
    Shield,
    Sun,
    Moon,
    Home
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { useTheme } from "./theme-provider";

export interface SidebarNavProps {
    activeTab: string;
    onTabChange: (tab: string) => void;
    className?: string;
    onLogout?: () => void;
    onHome?: () => void;
    version?: string;
}

const NAV_ITEMS = [
    {
        title: "概览",
        value: "overview",
        icon: LayoutDashboard,
    },
    {
        title: "规则管理",
        value: "rules",
        icon: FileText,
    },
    {
        title: "活动记录",
        value: "activity",
        icon: History,
    },
    {
        title: "转换器",
        value: "transformers",
        icon: Code2,
    },
    {
        title: "客户端",
        value: "clients",
        icon: Monitor,
    },
    {
        title: "安全防护",
        value: "security",
        icon: Shield,
    },
    {
        title: "系统配置",
        value: "config",
        icon: Settings,
    },
];

export function AppSidebar({ activeTab, onTabChange, className, onLogout, onHome, version }: SidebarNavProps) {
    const { theme, toggleTheme } = useTheme();

    return (
        <div className={cn("flex flex-col h-full bg-sidebar border-r border-sidebar-border w-64 transition-all duration-300", className)}>
            {/* Header */}
            <div className="h-16 flex items-center px-6 border-b border-sidebar-border">
                <div className="w-8 h-8 bg-gradient-to-br from-primary to-blue-600 rounded-lg flex items-center justify-center mr-3 shadow-sm hover-lift">
                    <Shield className="w-5 h-5 text-primary-foreground" />
                </div>
                <div>
                    <h1 className="font-bold text-sidebar-foreground tracking-tight">Rule Manager</h1>
                    <div className="flex items-center gap-2">
                        <p className="text-[10px] text-muted-foreground uppercase tracking-wider font-medium">Proxy Control</p>
                        {version && (
                            <span className="text-[11px] px-1.5 py-0.5 rounded bg-sidebar-primary/10 text-sidebar-primary border border-sidebar-primary/20 font-mono leading-none">
                                v{version}
                            </span>
                        )}
                    </div>
                </div>
            </div>

            {/* Navigation */}
            <div className="flex-1 py-6 px-3 space-y-1 overflow-y-auto">
                <div className="mb-2 px-3 text-xs font-semibold text-muted-foreground/70 uppercase tracking-wider">
                    Platform
                </div>
                {NAV_ITEMS.map((item) => {
                    const isActive = activeTab === item.value;
                    return (
                        <button
                            key={item.value}
                            onClick={() => onTabChange(item.value)}
                            className={cn(
                                "w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-md transition-all duration-200 group",
                                isActive
                                    ? "bg-sidebar-primary/10 text-sidebar-primary shadow-sm"
                                    : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                            )}
                        >
                            <item.icon className={cn("w-4 h-4 transition-colors", isActive ? "text-sidebar-primary" : "text-muted-foreground group-hover:text-sidebar-accent-foreground")} />
                            {item.title}
                            {isActive && (
                                <div className="ml-auto w-1.5 h-1.5 rounded-full bg-sidebar-primary animate-pulse" />
                            )}
                        </button>
                    );
                })}
            </div>

            {/* Footer / User Controls */}
            <div className="p-4 border-t border-sidebar-border space-y-2 bg-sidebar/50">
                <div className="flex items-center gap-2 p-2 rounded-md bg-sidebar-accent/50 border border-sidebar-border/50">
                    <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-sidebar-foreground truncate">Administrator</p>
                        <p className="text-xs text-muted-foreground truncate">System Access</p>
                    </div>
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={toggleTheme}
                        className="h-8 w-8 text-muted-foreground hover:text-foreground"
                    >
                        {theme === "light" ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
                    </Button>
                </div>

                <div className="flex flex-col gap-1">
                    <Button
                        variant="ghost"
                        className="w-full justify-start text-muted-foreground hover:text-primary hover:bg-primary/10 gap-3"
                        onClick={onHome}
                    >
                        <Home className="w-4 h-4" />
                        返回首页
                    </Button>

                    <Button
                        variant="ghost"
                        className="w-full justify-start text-muted-foreground hover:text-destructive hover:bg-destructive/10 gap-3"
                        onClick={onLogout}
                    >
                        <LogOut className="w-4 h-4" />
                        退出登录
                    </Button>
                </div>
            </div>
        </div>
    );
}
