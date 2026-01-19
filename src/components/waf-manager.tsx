"use client";

import { useState, useEffect, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
    Shield,
    ShieldAlert,
    ShieldCheck,
    Plus,
    Trash2,
    RefreshCw,
    Clock,
    Ban,
    AlertTriangle,
    Loader2,
} from "lucide-react";
import {
    getWafBans,
    getWafStats,
    getWafFailures,
    addWafBan,
    removeWafBan,
    cleanupWafBans,
    getMyIp,
    type BanRecord,
    type WafStats,
    type FailureInfo,
} from "@/lib/api-client";
import { toast } from "sonner";

export function WafManager() {
    const [bans, setBans] = useState<BanRecord[]>([]);
    const [stats, setStats] = useState<WafStats | null>(null);
    const [failures, setFailures] = useState<FailureInfo[]>([]);
    const [myIp, setMyIp] = useState<string>("");
    const [loading, setLoading] = useState(true);
    const [refreshing, setRefreshing] = useState(false);

    // 添加封禁表单
    const [newIp, setNewIp] = useState("");
    const [newReason, setNewReason] = useState("");
    const [isPermanent, setIsPermanent] = useState(false);
    const [duration, setDuration] = useState("3600");
    const [adding, setAdding] = useState(false);

    const loadData = useCallback(async () => {
        try {
            const [bansRes, statsRes, failuresRes, ipRes] = await Promise.all([
                getWafBans(),
                getWafStats(),
                getWafFailures(),
                getMyIp(),
            ]);
            setBans(bansRes.bans);
            setStats(statsRes);
            setFailures(failuresRes.failures);
            setMyIp(ipRes.ip);
        } catch (error) {
            toast.error("加载数据失败", {
                description: error instanceof Error ? error.message : "未知错误",
            });
        } finally {
            setLoading(false);
            setRefreshing(false);
        }
    }, []);

    useEffect(() => {
        loadData();
    }, [loadData]);

    const handleRefresh = async () => {
        setRefreshing(true);
        await loadData();
    };

    const handleAddBan = async () => {
        if (!newIp.trim()) {
            toast.error("请输入 IP 地址");
            return;
        }

        setAdding(true);
        try {
            await addWafBan(
                newIp.trim(),
                newReason.trim() || "manual_ban",
                isPermanent,
                isPermanent ? undefined : parseInt(duration, 10)
            );
            toast.success(`已封禁 IP: ${newIp}`);
            setNewIp("");
            setNewReason("");
            await loadData();
        } catch (error) {
            toast.error("封禁失败", {
                description: error instanceof Error ? error.message : "未知错误",
            });
        } finally {
            setAdding(false);
        }
    };

    const handleRemoveBan = async (ip: string) => {
        try {
            await removeWafBan(ip);
            toast.success(`已解封 IP: ${ip}`);
            await loadData();
        } catch (error) {
            toast.error("解封失败", {
                description: error instanceof Error ? error.message : "未知错误",
            });
        }
    };

    const handleCleanup = async () => {
        try {
            const result = await cleanupWafBans();
            toast.success(result.message);
            await loadData();
        } catch (error) {
            toast.error("清理失败", {
                description: error instanceof Error ? error.message : "未知错误",
            });
        }
    };

    const formatTime = (isoString: string) => {
        return new Date(isoString).toLocaleString("zh-CN");
    };

    const getTimeRemaining = (expiresAt: string | null) => {
        if (!expiresAt) return "永久";
        const remaining = new Date(expiresAt).getTime() - Date.now();
        if (remaining <= 0) return "已过期";
        const minutes = Math.floor(remaining / 60000);
        const hours = Math.floor(minutes / 60);
        if (hours > 0) return `${hours}小时${minutes % 60}分钟`;
        return `${minutes}分钟`;
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
            </div>
        );
    }

    return (
        <div className="space-y-6">
            {/* 统计卡片 */}
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <Card>
                    <CardHeader className="pb-2">
                        <CardDescription>持久化封禁</CardDescription>
                        <CardTitle className="text-2xl flex items-center gap-2">
                            <Ban className="w-5 h-5 text-red-500" />
                            {stats?.bans.total || 0}
                        </CardTitle>
                    </CardHeader>
                </Card>
                <Card>
                    <CardHeader className="pb-2">
                        <CardDescription>永久封禁</CardDescription>
                        <CardTitle className="text-2xl flex items-center gap-2">
                            <ShieldAlert className="w-5 h-5 text-orange-500" />
                            {stats?.bans.permanent || 0}
                        </CardTitle>
                    </CardHeader>
                </Card>
                <Card>
                    <CardHeader className="pb-2">
                        <CardDescription>临时追踪</CardDescription>
                        <CardTitle className="text-2xl flex items-center gap-2">
                            <Clock className="w-5 h-5 text-blue-500" />
                            {stats?.temporary.totalTracked || 0}
                        </CardTitle>
                    </CardHeader>
                </Card>
                <Card>
                    <CardHeader className="pb-2">
                        <CardDescription>当前阻塞</CardDescription>
                        <CardTitle className="text-2xl flex items-center gap-2">
                            <Shield className="w-5 h-5 text-yellow-500" />
                            {stats?.temporary.currentlyBlocked || 0}
                        </CardTitle>
                    </CardHeader>
                </Card>
            </div>

            {/* 当前 IP */}
            <Card>
                <CardHeader className="pb-3">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <ShieldCheck className="w-5 h-5 text-green-500" />
                            <CardTitle className="text-lg">您的 IP 地址</CardTitle>
                        </div>
                        <Badge variant="outline" className="font-mono">
                            {myIp}
                        </Badge>
                    </div>
                </CardHeader>
            </Card>

            {/* 添加封禁 */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                        <Plus className="w-5 h-5" />
                        手动添加封禁
                    </CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="space-y-2">
                            <Label htmlFor="ip">IP 地址</Label>
                            <Input
                                id="ip"
                                placeholder="例如: 192.168.1.100"
                                value={newIp}
                                onChange={(e) => setNewIp(e.target.value)}
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="reason">原因</Label>
                            <Input
                                id="reason"
                                placeholder="封禁原因（可选）"
                                value={newReason}
                                onChange={(e) => setNewReason(e.target.value)}
                            />
                        </div>
                    </div>
                    <div className="flex items-center gap-6">
                        <div className="flex items-center gap-2">
                            <Switch
                                id="permanent"
                                checked={isPermanent}
                                onCheckedChange={setIsPermanent}
                            />
                            <Label htmlFor="permanent">永久封禁</Label>
                        </div>
                        {!isPermanent && (
                            <div className="flex items-center gap-2">
                                <Label htmlFor="duration">封禁时长（秒）</Label>
                                <Input
                                    id="duration"
                                    type="number"
                                    className="w-32"
                                    value={duration}
                                    onChange={(e) => setDuration(e.target.value)}
                                />
                            </div>
                        )}
                    </div>
                    <Button onClick={handleAddBan} disabled={adding || !newIp.trim()}>
                        {adding ? (
                            <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                        ) : (
                            <Plus className="w-4 h-4 mr-2" />
                        )}
                        添加封禁
                    </Button>
                </CardContent>
            </Card>

            {/* 封禁列表 */}
            <Card>
                <CardHeader>
                    <div className="flex items-center justify-between">
                        <CardTitle className="text-lg flex items-center gap-2">
                            <Ban className="w-5 h-5" />
                            封禁列表
                        </CardTitle>
                        <div className="flex gap-2">
                            <Button variant="outline" size="sm" onClick={handleCleanup}>
                                <Trash2 className="w-4 h-4 mr-2" />
                                清理过期
                            </Button>
                            <Button variant="outline" size="sm" onClick={handleRefresh} disabled={refreshing}>
                                <RefreshCw className={`w-4 h-4 mr-2 ${refreshing ? "animate-spin" : ""}`} />
                                刷新
                            </Button>
                        </div>
                    </div>
                </CardHeader>
                <CardContent>
                    {bans.length === 0 ? (
                        <div className="text-center py-8 text-muted-foreground">
                            <ShieldCheck className="w-12 h-12 mx-auto mb-2 opacity-50" />
                            <p>暂无封禁记录</p>
                        </div>
                    ) : (
                        <ScrollArea className="h-[300px]">
                            <div className="space-y-2">
                                {bans.map((ban) => (
                                    <div
                                        key={ban.ip}
                                        className="flex items-center justify-between p-3 rounded-lg border bg-card hover:bg-accent/50 transition-colors"
                                    >
                                        <div className="flex-1">
                                            <div className="flex items-center gap-2">
                                                <code className="font-mono text-sm font-semibold">{ban.ip}</code>
                                                {ban.expiresAt === null ? (
                                                    <Badge variant="destructive">永久</Badge>
                                                ) : (
                                                    <Badge variant="secondary">{getTimeRemaining(ban.expiresAt)}</Badge>
                                                )}
                                                {ban.ip === myIp && (
                                                    <Badge variant="outline" className="text-yellow-600">
                                                        <AlertTriangle className="w-3 h-3 mr-1" />
                                                        您的 IP
                                                    </Badge>
                                                )}
                                            </div>
                                            <div className="text-sm text-muted-foreground mt-1">
                                                {ban.reason} • 失败次数: {ban.failCount} • 封禁于: {formatTime(ban.bannedAt)}
                                            </div>
                                        </div>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => handleRemoveBan(ban.ip)}
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </Button>
                                    </div>
                                ))}
                            </div>
                        </ScrollArea>
                    )}
                </CardContent>
            </Card>

            {/* 活跃失败记录 */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-lg flex items-center gap-2">
                        <Clock className="w-5 h-5" />
                        活跃失败记录（内存中）
                    </CardTitle>
                    <CardDescription>
                        临时追踪的登录失败记录，24小时后自动清除
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    {failures.length === 0 ? (
                        <div className="text-center py-8 text-muted-foreground">
                            <ShieldCheck className="w-12 h-12 mx-auto mb-2 opacity-50" />
                            <p>暂无活跃失败记录</p>
                        </div>
                    ) : (
                        <ScrollArea className="h-[200px]">
                            <div className="space-y-2">
                                {failures.map((failure) => (
                                    <div
                                        key={failure.ip}
                                        className="flex items-center justify-between p-3 rounded-lg border bg-card"
                                    >
                                        <div>
                                            <div className="flex items-center gap-2">
                                                <code className="font-mono text-sm font-semibold">{failure.ip}</code>
                                                {failure.isBlocked ? (
                                                    <Badge variant="destructive">阻塞中</Badge>
                                                ) : (
                                                    <Badge variant="outline">追踪中</Badge>
                                                )}
                                            </div>
                                            <div className="text-sm text-muted-foreground mt-1">
                                                失败次数: {failure.failCount} • 阻塞时长: {failure.blockDuration}秒
                                                {failure.blockedUntil && ` • 解除时间: ${formatTime(failure.blockedUntil)}`}
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        </ScrollArea>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}
