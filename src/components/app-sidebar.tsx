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
            <div className={cn("h-16 flex items-center border-b border-sidebar-border", isCollapsed ? "px-3 justify-center" : "px-6")}>
                <div className={cn("w-9 h-9 bg-gradient-to-br from-primary via-primary to-blue-600 rounded-xl flex items-center justify-center shadow-sm hover-lift", !isCollapsed && "mr-3")}>
                    <Shield className="w-5 h-5 text-primary-foreground" />
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
                        className="ml-auto h-8 w-8"
                    >
                        {isCollapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
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
                                    ? "bg-sidebar-primary/15 text-sidebar-primary shadow-sm ring-1 ring-sidebar-primary/20"
                                    : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                            )}
                            title={isCollapsed ? item.title : undefined}
                        >
                            <item.icon className={cn("w-4 h-4 transition-colors flex-shrink-0", isActive ? "text-sidebar-primary" : "text-muted-foreground group-hover:text-sidebar-accent-foreground")} />
                            {!isCollapsed && item.title}
                            {!isCollapsed && isActive && (
                                <div className="ml-auto w-1.5 h-1.5 rounded-full bg-sidebar-primary animate-pulse" />
                            )}
                        </button>
                    );
                })}
            </div>

            {/* Footer / User Controls */}
            <div className={cn("border-t border-sidebar-border space-y-2 bg-sidebar/50", isCollapsed ? "p-2" : "p-4")}>
                {isCollapsed ? (
                    <div className="flex flex-col items-center gap-2">
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={toggleTheme}
                            className="h-10 w-10 text-muted-foreground hover:text-foreground"
                            title={theme === "light" ? "切换到暗色模式" : "切换到亮色模式"}
                        >
                            {theme === "light" ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
                        </Button>
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={onHome}
                            className="h-10 w-10 text-muted-foreground hover:text-primary hover:bg-primary/10"
                            title="返回首页"
                        >
                            <Home className="w-4 h-4" />
                        </Button>
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={onLogout}
                            className="h-10 w-10 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                            title="退出登录"
                        >
                            <LogOut className="w-4 h-4" />
                        </Button>
                    </div>
                ) : (
                    <>
                        <div className="flex items-center gap-2 p-2 rounded-md bg-sidebar-accent/50 border border-sidebar-border/50">
                            <div className="flex-1 min-w-0">
                                <p className="text-sm font-medium text-sidebar-foreground truncate">Administrator</p>
                                <p className="text-xs text-muted-foreground truncate">System Access</p>
                            </div>
                            <Button
                                variant="ghost"
                                size="icon"
                                onClick={toggleTheme}
                                className="h-10 w-10 text-muted-foreground hover:text-foreground"
                            >
                                {theme === "light" ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" />}
                            </Button>
                        </div>

                        <div className="flex flex-col gap-1">
                            <Button
                                variant="ghost"
                                className="w-full justify-start text-muted-foreground hover:text-primary hover:bg-primary/10 gap-3 h-11"
                                onClick={onHome}
                            >
                                <Home className="w-4 h-4" />
                                返回首页
                            </Button>

                            <Button
                                variant="ghost"
                                className="w-full justify-start text-muted-foreground hover:text-destructive hover:bg-destructive/10 gap-3 h-11"
                                onClick={onLogout}
                            >
                                <LogOut className="w-4 h-4" />
                                退出登录
                            </Button>
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
