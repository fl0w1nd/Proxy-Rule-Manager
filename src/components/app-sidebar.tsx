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
    Home,
    Image as ImageIcon
} from "lucide-react";
import NextImage from "next/image";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ThemeSwitcher } from "./theme-switcher";

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
        icon: ImageIcon,
    },
    {
        title: "系统配置",
        value: "config",
        icon: Settings,
    },
];

export function AppSidebar({ activeTab, onTabChange, className, onLogout, onHome, version, isCollapsed, onToggle }: SidebarNavProps) {

    return (
        <div className={cn(
            "flex flex-col h-full bg-sidebar border-r border-sidebar-border transition-all duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
            isCollapsed ? "w-16" : "w-60",
            className
        )}>
            {/* Header */}
            <div className={cn(
                "flex items-center border-b border-sidebar-border h-16",
                isCollapsed ? "justify-center px-2" : "px-4 gap-3"
            )}>
                {isCollapsed ? (
                    onToggle ? (
                        <Button
                            variant="ghost"
                            size="icon"
                            className="h-9 w-9 rounded-full"
                            onClick={onToggle}
                            title="展开侧栏"
                        >
                            <ChevronRight className="w-4 h-4" />
                        </Button>
                    ) : (
                        <div className="w-8 h-8 flex items-center justify-center">
                            <NextImage src="/logo.svg" alt="Logo" width={32} height={32} className="w-full h-full object-contain" />
                        </div>
                    )
                ) : (
                    <>
                        <div className="w-9 h-9 flex-shrink-0 flex items-center justify-center">
                            <NextImage src="/logo.svg" alt="Logo" width={36} height={36} className="w-full h-full object-contain" />
                        </div>
                        <div className="flex-1 min-w-0 overflow-hidden">
                            <div className="flex items-baseline gap-2">
                                <h1 className="font-bold text-sidebar-foreground tracking-tight leading-tight truncate">后台管理</h1>
                                {version && (
                                    <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-primary/10 text-primary-foreground font-mono leading-none shrink-0">
                                        {version}
                                    </span>
                                )}
                            </div>
                        </div>
                        {onToggle && (
                            <Button
                                variant="ghost"
                                size="icon"
                                onClick={onToggle}
                                className="ml-auto h-9 w-9 flex-shrink-0 rounded-full"
                                title="收起侧栏"
                            >
                                <ChevronLeft className="w-4 h-4" />
                            </Button>
                        )}
                    </>
                )}
            </div>

            {/* Navigation */}
            <div className="flex-1 py-4 px-3 space-y-0.5 overflow-y-auto">
                {!isCollapsed && (
                    <div className="mb-2 px-3 text-[11px] font-semibold text-muted-foreground uppercase tracking-widest">
                        功能
                    </div>
                )}
                {NAV_ITEMS.map((item, index) => {
                    const isActive = activeTab === item.value;
                    return (
                        <Button
                            key={item.value}
                            onClick={() => onTabChange(item.value)}
                            style={{ animationDelay: `${index * 30}ms` }}
                            variant="ghost"
                            className={cn(
                                "w-full justify-start gap-3 px-3 py-2 text-sm font-medium transition-all duration-150 group animate-fade-in opacity-0",
                                isCollapsed && "h-10 justify-center px-0",
                                isActive
                                    ? "rounded-xl bg-sidebar-accent text-sidebar-foreground font-semibold"
                                    : "text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                            )}
                            title={isCollapsed ? item.title : undefined}
                        >
                            <item.icon className={cn("w-4 h-4 transition-colors flex-shrink-0", isActive ? "text-foreground" : "text-muted-foreground group-hover:text-sidebar-accent-foreground")} />
                            {!isCollapsed && (
                                <span className="truncate transition-opacity duration-200">{item.title}</span>
                            )}
                        </Button>
                    );
                })}
            </div>

            {/* Footer */}
            <div className={cn("border-t border-sidebar-border", isCollapsed ? "p-2" : "p-3")}>
                {isCollapsed ? (
                    <div className="flex flex-col items-center gap-1">
                        <ThemeSwitcher compact />
                        <Button
                            onClick={onHome}
                            variant="ghost"
                            size="icon"
                            className="h-9 w-9 rounded-full"
                            title="返回首页"
                        >
                            <Home className="w-4 h-4" />
                        </Button>
                        {onLogout && (
                            <Button
                                onClick={onLogout}
                                variant="ghost"
                                size="icon"
                                className="h-9 w-9 rounded-full"
                                title="退出登录"
                            >
                                <LogOut className="w-4 h-4" />
                            </Button>
                        )}
                    </div>
                ) : (
                    <div className="space-y-2">
                        <div className="flex items-center gap-2 px-2 py-1.5">
                            <div className="flex-1 min-w-0">
                                <p className="text-sm font-semibold text-sidebar-foreground truncate">管理员</p>
                                <p className="text-xs text-muted-foreground truncate">系统访问</p>
                            </div>
                        </div>

                        <ThemeSwitcher />

                        <div className="flex gap-1.5">
                            <Button
                                onClick={onHome}
                                variant="ghost"
                                className="flex-1 h-9 text-sm"
                            >
                                <Home className="w-4 h-4" />
                                返回首页
                            </Button>

                            {onLogout && (
                                <Button
                                    onClick={onLogout}
                                    variant="ghost"
                                    className="flex-1 h-9 text-sm"
                                >
                                    <LogOut className="w-4 h-4" />
                                    退出登录
                                </Button>
                            )}
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}
