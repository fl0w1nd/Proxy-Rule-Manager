"use client";

import {
    ChevronLeft,
    ChevronRight,
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
    Home,
    Image
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
    isCollapsed?: boolean;
    onToggle?: () => void;
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
        title: "图标集",
        value: "iconset",
        icon: Image,
    },
    {
        title: "系统配置",
        value: "config",
        icon: Settings,
    },
];

export function AppSidebar({ activeTab, onTabChange, className, onLogout, onHome, version, isCollapsed, onToggle }: SidebarNavProps) {
    const { theme, toggleTheme } = useTheme();

    return (
        <div className={cn(
            "flex flex-col h-full bg-sidebar border-r border-sidebar-border transition-all duration-300",
            isCollapsed ? "w-16" : "w-64",
            className
        )}>
            {/* Header */}
            <div className={cn("h-16 flex items-center border-b border-sidebar-border relative", isCollapsed ? "justify-center" : "px-6 gap-3")}>
                <div className={cn("w-9 h-9 flex-shrink-0 flex items-center justify-center transition-transform hover:scale-105 duration-300")}>
                    <img src="/logo.svg" alt="Logo" className="w-full h-full object-contain" />
                </div>
                {!isCollapsed && (
                    <div className="flex-1 min-w-0">
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
                )}
                {onToggle && (
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={onToggle}
                        className={cn(
                            "h-8 w-8 transition-all",
                            isCollapsed ? "absolute -right-3 top-1/2 -translate-y-1/2 bg-sidebar border border-sidebar-border shadow-sm rounded-full z-50 hover:bg-sidebar-accent" : "ml-auto"
                        )}
                    >
                        {isCollapsed ? <ChevronRight className="w-3 h-3" /> : <ChevronLeft className="w-3 h-3" />}
                    </Button>
                )}
            </div>

            {/* Navigation */}
            <div className="flex-1 py-6 px-3 space-y-1 overflow-y-auto">
                {!isCollapsed && (
                    <div className="mb-2 px-3 text-xs font-semibold text-muted-foreground/70 uppercase tracking-wider">
                        Platform
                    </div>
                )}
                {NAV_ITEMS.map((item, index) => {
                    const isActive = activeTab === item.value;
                    return (
                        <button
                            key={item.value}
                            onClick={() => onTabChange(item.value)}
                            style={{ animationDelay: `${index * 30}ms` }}
                            className={cn(
                                "w-full flex items-center gap-3 px-3 py-2 text-sm font-medium rounded-md transition-all duration-200 group animate-fade-in opacity-0",
                                isCollapsed && "justify-center",
                                isActive
                                    ? "neu-pill-active !rounded-lg"
                                    : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                            )}
                            title={isCollapsed ? item.title : undefined}
                        >
                            <item.icon className={cn("w-4 h-4 transition-colors flex-shrink-0", isActive ? "text-blue-600/80 dark:text-blue-300/80" : "text-muted-foreground group-hover:text-sidebar-accent-foreground")} />
                            {!isCollapsed && item.title}
                            {!isCollapsed && isActive && (
                                <div className="ml-auto w-1.5 h-1.5 rounded-full bg-blue-500/60 dark:bg-blue-400/60 animate-pulse" />
                            )}
                        </button>
                    );
                })}
            </div>

            {/* Footer / User Controls */}
            <div className={cn("border-t border-sidebar-border space-y-2 bg-sidebar/50", isCollapsed ? "p-2" : "p-4")}>
                {isCollapsed ? (
                    <div className="flex flex-col items-center gap-2">
                        <button
                            onClick={toggleTheme}
                            className="neu-btn w-10 h-10 flex items-center justify-center text-muted-foreground !rounded-xl"
                            title={theme === "light" ? "切换到暗色模式" : "切换到亮色模式"}
                        >
                            {theme === "light" ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
                        </button>
                        <button
                            onClick={onHome}
                            className="neu-btn w-10 h-10 flex items-center justify-center text-muted-foreground !rounded-xl"
                            title="返回首页"
                        >
                            <Home className="w-4 h-4" />
                        </button>
                        {onLogout && (
                            <button
                                onClick={onLogout}
                                className="neu-btn w-10 h-10 flex items-center justify-center text-muted-foreground !rounded-xl"
                                title="退出登录"
                            >
                                <LogOut className="w-4 h-4" />
                            </button>
                        )}
                    </div>
                ) : (
                    <>
                        <div className="flex items-center gap-2 p-2 rounded-md bg-sidebar-accent/50 border border-sidebar-border/50">
                            <div className="flex-1 min-w-0">
                                <p className="text-sm font-medium text-sidebar-foreground truncate">Administrator</p>
                                <p className="text-xs text-muted-foreground truncate">System Access</p>
                            </div>
                            <button
                                onClick={toggleTheme}
                                className="neu-btn w-9 h-9 flex items-center justify-center text-muted-foreground !rounded-lg flex-shrink-0"
                            >
                                {theme === "light" ? <Moon className="w-3.5 h-3.5" /> : <Sun className="w-3.5 h-3.5" />}
                            </button>
                        </div>

                        <div className="flex gap-2">
                            <button
                                onClick={onHome}
                                className="neu-btn flex-1 h-10 flex items-center justify-center gap-2 text-sm text-muted-foreground !rounded-xl"
                            >
                                <Home className="w-4 h-4" />
                                返回首页
                            </button>

                            {onLogout && (
                                <button
                                    onClick={onLogout}
                                    className="neu-btn flex-1 h-10 flex items-center justify-center gap-2 text-sm text-muted-foreground !rounded-xl"
                                >
                                    <LogOut className="w-4 h-4" />
                                    退出登录
                                </button>
                            )}
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
