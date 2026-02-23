"use client";

import { useState, useEffect, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
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
    Cloud,
    Save,
    Info,
} from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
} from "@/components/ui/tooltip";
import {
    getWafBans,
    getWafStats,
    getWafFailures,
    addWafBan,
    removeWafBan,
    cleanupWafBans,
    getMyIp,
    getCdnSettings,
    updateCdnSettings,
    type BanRecord,
    type WafStats,
    type FailureInfo,
    type CdnSettings,
} from "@/lib/api-client";
import { toast } from "sonner";
import { createListItemKey, createListItemKeys } from "@/lib/utils";

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

    // CDN 设置
    const [cdnSettings, setCdnSettings] = useState<CdnSettings | null>(null);
    const [savingCdn, setSavingCdn] = useState(false);
    const [newHeaderName, setNewHeaderName] = useState("");
    const [newHeaderValue, setNewHeaderValue] = useState("");
    const [customHeaderKeys, setCustomHeaderKeys] = useState<string[]>([]);

    const loadData = useCallback(async () => {
        try {
            const [bansRes, statsRes, failuresRes, ipRes, cdnRes] = await Promise.all([
                getWafBans(),
                getWafStats(),
                getWafFailures(),
                getMyIp(),
                getCdnSettings(),
            ]);
            setBans(bansRes.bans);
            setStats(statsRes);
            setFailures(failuresRes.failures);
            setMyIp(ipRes.ip);
            setCdnSettings(cdnRes.settings);
            setCustomHeaderKeys(createListItemKeys(cdnRes.settings.customHeaders.length));
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

        if (!isPermanent) {
            const durationSeconds = parseInt(duration, 10);
            if (isNaN(durationSeconds) || durationSeconds <= 0) {
                toast.error("封禁时长必须是正整数（秒）");
                return;
            }
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

    const handleSaveCdnSettings = async () => {
        if (!cdnSettings) return;
        setSavingCdn(true);
        try {
            const result = await updateCdnSettings(cdnSettings);
            setCdnSettings(result.settings);
            toast.success("CDN 设置已保存");
        } catch (error) {
            toast.error("保存失败", {
                description: error instanceof Error ? error.message : "未知错误",
            });
        } finally {
            setSavingCdn(false);
        }
    };

    const handleAddCustomHeader = () => {
        if (!newHeaderName.trim() || !newHeaderValue.trim()) {
            toast.error("请输入完整的响应头名称和值");
            return;
        }
        if (!cdnSettings) return;
        setCdnSettings((prev) => {
            if (!prev) return prev;
            return {
                ...prev,
                customHeaders: [
                    ...prev.customHeaders,
                    { name: newHeaderName.trim(), value: newHeaderValue.trim() },
                ],
            };
        });
        setCustomHeaderKeys((prev) => [...prev, createListItemKey()]);
        setNewHeaderName("");
        setNewHeaderValue("");
    };

    const handleRemoveCustomHeader = (index: number) => {
        if (!cdnSettings) return;
        setCdnSettings((prev) => {
            if (!prev) return prev;
            return {
                ...prev,
                customHeaders: prev.customHeaders.filter((_, i) => i !== index),
            };
        });
        setCustomHeaderKeys((prev) => prev.filter((_, i) => i !== index));
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
                <Card className="p-4">
                    <p className="text-xs text-muted-foreground mb-1">持久化封禁</p>
                    <div className="text-2xl font-bold flex items-center gap-2">
                        <Ban className="w-5 h-5 text-red-500" />
                        {stats?.bans.total || 0}
                    </div>
                </Card>
                <Card className="p-4">
                    <p className="text-xs text-muted-foreground mb-1">永久封禁</p>
                    <div className="text-2xl font-bold flex items-center gap-2">
                        <ShieldAlert className="w-5 h-5 text-orange-500" />
                        {stats?.bans.permanent || 0}
                    </div>
                </Card>
                <Card className="p-4">
                    <p className="text-xs text-muted-foreground mb-1">临时追踪</p>
                    <div className="text-2xl font-bold flex items-center gap-2">
                        <Clock className="w-5 h-5 text-blue-500" />
                        {stats?.temporary.totalTracked || 0}
                    </div>
                </Card>
                <Card className="p-4">
                    <p className="text-xs text-muted-foreground mb-1">当前阻塞</p>
                    <div className="text-2xl font-bold flex items-center gap-2">
                        <Shield className="w-5 h-5 text-yellow-500" />
                        {stats?.temporary.currentlyBlocked || 0}
                    </div>
                </Card>
            </div>

            {/* 当前 IP */}
            <Card>
                <div className="p-5">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                            <ShieldCheck className="w-5 h-5 text-green-500" />
                            <h3 className="text-lg font-semibold text-foreground">您的 IP 地址</h3>
                        </div>
                        <Badge variant="outline" className="font-mono">
                            {myIp}
                        </Badge>
                    </div>
                </div>
            </Card>

            {/* 添加封禁 */}
            <Card>
                <div className="px-5 pt-5 pb-3">
                    <h3 className="text-lg font-semibold text-foreground flex items-center gap-2">
                        <Plus className="w-5 h-5" />
                        手动添加封禁
                    </h3>
                </div>
                <div className="px-5 pb-5 space-y-4">
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
                </div>
            </Card>

            {/* 封禁列表 */}
            <Card>
                <div className="px-5 pt-5 pb-3">
                    <div className="flex items-center justify-between">
                        <h3 className="text-lg font-semibold text-foreground flex items-center gap-2">
                            <Ban className="w-5 h-5" />
                            封禁列表
                        </h3>
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
                </div>
                <div className="px-5 pb-5">
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
                                            aria-label={`解除封禁 ${ban.ip}`}
                                            onClick={() => handleRemoveBan(ban.ip)}
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </Button>
                                    </div>
                                ))}
                            </div>
                        </ScrollArea>
                    )}
                </div>
            </Card>

            {/* 活跃失败记录 */}
            <Card>
                <div className="px-5 pt-5 pb-3">
                    <h3 className="text-lg font-semibold text-foreground flex items-center gap-2">
                        <Clock className="w-5 h-5" />
                        活跃失败记录（内存中）
                    </h3>
                    <p className="text-sm text-muted-foreground mt-1">
                        临时追踪的登录失败记录，24小时后自动清除
                    </p>
                </div>
                <div className="px-5 pb-5">
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
                </div>
            </Card>

            {/* CDN 缓存设置 */}
            {
                cdnSettings && (
                    <Card>
                        <div className="px-5 pt-5 pb-3">
                            <div className="flex items-center justify-between">
                                <div>
                                    <h3 className="text-lg font-semibold text-foreground flex items-center gap-2">
                                        <Cloud className="w-5 h-5 text-blue-500" />
                                        CDN 缓存设置
                                    </h3>
                                    <p className="text-sm text-muted-foreground mt-1">
                                        配置规则文件、客户端配置文件和图标集的 HTTP 响应头，优化 CDN（如 Cloudflare）缓存行为
                                    </p>
                                </div>
                                <Button onClick={handleSaveCdnSettings} disabled={savingCdn}>
                                    {savingCdn ? (
                                        <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                    ) : (
                                        <Save className="w-4 h-4 mr-2" />
                                    )}
                                    保存设置
                                </Button>
                            </div>
                        </div>
                        <div className="px-5 pb-5 space-y-6">
                            {/* 启用开关 */}
                            <div className="flex items-center justify-between p-4 rounded-lg border bg-muted/30">
                                <div className="space-y-0.5">
                                    <Label className="text-base">启用自定义响应头</Label>
                                    <p className="text-sm text-muted-foreground">
                                        开启后将使用下方配置的缓存策略，关闭则使用默认的 no-cache
                                    </p>
                                </div>
                                <Switch
                                    checked={cdnSettings.enabled}
                                    onCheckedChange={(checked) =>
                                        setCdnSettings({ ...cdnSettings, enabled: checked })
                                    }
                                />
                            </div>

                            {cdnSettings.enabled && (
                                <>
                                    {/* 缓存模式 */}
                                    <div className="space-y-3">
                                        <Label>缓存模式</Label>
                                        <RadioGroup
                                            value={cdnSettings.cacheMode}
                                            onValueChange={(value: "no-cache" | "no-store" | "custom") =>
                                                setCdnSettings({ ...cdnSettings, cacheMode: value })
                                            }
                                            className="space-y-1"
                                        >
                                            <div className="flex items-center space-x-3 p-3 rounded-lg border hover:bg-accent/50 cursor-pointer">
                                                <RadioGroupItem value="no-cache" id="cache-no-cache" />
                                                <Label htmlFor="cache-no-cache" className="flex-1 cursor-pointer">
                                                    <div className="font-medium">协商缓存 + 备用缓存</div>
                                                    <div className="text-xs text-muted-foreground">
                                                        每次请求验证源站，源站不可用时用旧缓存兜底
                                                        <code className="ml-1 bg-muted px-1 rounded">no-cache, stale-if-error</code>
                                                    </div>
                                                </Label>
                                                <Badge variant="secondary" className="text-xs">推荐</Badge>
                                            </div>
                                            <div className="flex items-center space-x-3 p-3 rounded-lg border hover:bg-accent/50 cursor-pointer">
                                                <RadioGroupItem value="no-store" id="cache-no-store" />
                                                <Label htmlFor="cache-no-store" className="flex-1 cursor-pointer">
                                                    <div className="font-medium">完全不缓存</div>
                                                    <div className="text-xs text-muted-foreground">
                                                        CDN 不缓存任何内容
                                                        <code className="ml-1 bg-muted px-1 rounded">no-store</code>
                                                    </div>
                                                </Label>
                                            </div>
                                            <div className="flex items-center space-x-3 p-3 rounded-lg border hover:bg-accent/50 cursor-pointer">
                                                <RadioGroupItem value="custom" id="cache-custom" />
                                                <Label htmlFor="cache-custom" className="flex-1 cursor-pointer">
                                                    <div className="font-medium">自定义</div>
                                                    <div className="text-xs text-muted-foreground">完全自定义 Cache-Control 头</div>
                                                </Label>
                                            </div>
                                        </RadioGroup>
                                    </div>

                                    {/* stale-if-error 时长 */}
                                    {cdnSettings.cacheMode === "no-cache" && (
                                        <div className="space-y-2">
                                            <div className="flex items-center gap-2">
                                                <Label>备用缓存时长（stale-if-error）</Label>
                                                <TooltipProvider>
                                                    <Tooltip>
                                                        <TooltipTrigger>
                                                            <Info className="w-4 h-4 text-muted-foreground" />
                                                        </TooltipTrigger>
                                                        <TooltipContent className="max-w-sm">
                                                            <p>源站不可用时，CDN 继续提供旧缓存的最长时间</p>
                                                        </TooltipContent>
                                                    </Tooltip>
                                                </TooltipProvider>
                                            </div>
                                            <Select
                                                value={String(cdnSettings.staleIfErrorSeconds)}
                                                onValueChange={(value) =>
                                                    setCdnSettings({
                                                        ...cdnSettings,
                                                        staleIfErrorSeconds: parseInt(value, 10),
                                                    })
                                                }
                                            >
                                                <SelectTrigger className="w-48">
                                                    <SelectValue />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    <SelectItem value="3600">1 小时</SelectItem>
                                                    <SelectItem value="86400">1 天</SelectItem>
                                                    <SelectItem value="259200">3 天</SelectItem>
                                                    <SelectItem value="604800">7 天（推荐）</SelectItem>
                                                    <SelectItem value="1209600">14 天</SelectItem>
                                                    <SelectItem value="2592000">30 天</SelectItem>
                                                </SelectContent>
                                            </Select>
                                        </div>
                                    )}

                                    {/* 自定义 Cache-Control */}
                                    {cdnSettings.cacheMode === "custom" && (
                                        <div className="space-y-2">
                                            <Label>自定义 Cache-Control</Label>
                                            <Input
                                                placeholder="例如: public, max-age=300, stale-if-error=86400"
                                                value={cdnSettings.customCacheControl || ""}
                                                onChange={(e) =>
                                                    setCdnSettings({
                                                        ...cdnSettings,
                                                        customCacheControl: e.target.value,
                                                    })
                                                }
                                            />
                                        </div>
                                    )}

                                    {/* Cloudflare 专用头 */}
                                    <div className="space-y-3">
                                        <div className="flex items-center gap-2">
                                            <Label>Cloudflare-CDN-Cache-Control（可选）</Label>
                                            <TooltipProvider>
                                                <Tooltip>
                                                    <TooltipTrigger>
                                                        <Info className="w-4 h-4 text-muted-foreground" />
                                                    </TooltipTrigger>
                                                    <TooltipContent className="max-w-sm">
                                                        <p>Cloudflare 专用响应头，可覆盖常规 Cache-Control 的行为</p>
                                                    </TooltipContent>
                                                </Tooltip>
                                            </TooltipProvider>
                                        </div>
                                        <Input
                                            placeholder="例如: max-age=86400, stale-if-error=604800"
                                            value={cdnSettings.cloudflareCdnCacheControl || ""}
                                            onChange={(e) =>
                                                setCdnSettings({
                                                    ...cdnSettings,
                                                    cloudflareCdnCacheControl: e.target.value,
                                                })
                                            }
                                        />
                                    </div>

                                    {/* 自定义响应头 */}
                                    <div className="space-y-3">
                                        <Label>自定义响应头</Label>
                                        <div className="space-y-2">
                                            {cdnSettings.customHeaders.map((header, index) => (
                                                <div key={customHeaderKeys[index] ?? `custom-header-${index}`} className="flex items-center gap-2">
                                                    <Input
                                                        value={header.name}
                                                        onChange={(e) => {
                                                            const value = e.target.value;
                                                            setCdnSettings((prev) => {
                                                                if (!prev) return prev;
                                                                return {
                                                                    ...prev,
                                                                    customHeaders: prev.customHeaders.map((item, i) =>
                                                                        i === index ? { ...item, name: value } : item
                                                                    ),
                                                                };
                                                            });
                                                        }}
                                                        placeholder="响应头名称"
                                                        className="flex-1"
                                                    />
                                                    <Input
                                                        value={header.value}
                                                        onChange={(e) => {
                                                            const value = e.target.value;
                                                            setCdnSettings((prev) => {
                                                                if (!prev) return prev;
                                                                return {
                                                                    ...prev,
                                                                    customHeaders: prev.customHeaders.map((item, i) =>
                                                                        i === index ? { ...item, value } : item
                                                                    ),
                                                                };
                                                            });
                                                        }}
                                                        placeholder="响应头值"
                                                        className="flex-1"
                                                    />
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        aria-label="删除此响应头"
                                                        onClick={() => handleRemoveCustomHeader(index)}
                                                    >
                                                        <Trash2 className="w-4 h-4" />
                                                    </Button>
                                                </div>
                                            ))}
                                            <div className="flex items-center gap-2">
                                                <Input
                                                    value={newHeaderName}
                                                    onChange={(e) => setNewHeaderName(e.target.value)}
                                                    placeholder="新响应头名称"
                                                    className="flex-1"
                                                />
                                                <Input
                                                    value={newHeaderValue}
                                                    onChange={(e) => setNewHeaderValue(e.target.value)}
                                                    placeholder="新响应头值"
                                                    className="flex-1"
                                                />
                                                <Button
                                                    variant="outline"
                                                    size="sm"
                                                    onClick={handleAddCustomHeader}
                                                    disabled={!newHeaderName.trim() || !newHeaderValue.trim()}
                                                >
                                                    <Plus className="w-4 h-4" />
                                                </Button>
                                            </div>
                                        </div>
                                    </div>

                                    {/* 预览 */}
                                    <div className="space-y-2 p-4 rounded-lg border bg-muted/30">
                                        <Label className="text-sm text-muted-foreground">当前配置将生成的响应头预览：</Label>
                                        <Textarea
                                            readOnly
                                            className="font-mono text-xs min-h-[100px] bg-background"
                                            value={(() => {
                                                const lines: string[] = [];
                                                let cacheControl = "no-cache";
                                                if (cdnSettings.cacheMode === "no-store") {
                                                    cacheControl = "no-store";
                                                } else if (cdnSettings.cacheMode === "custom") {
                                                    cacheControl = cdnSettings.customCacheControl || "no-cache";
                                                } else if (cdnSettings.staleIfErrorSeconds > 0) {
                                                    cacheControl = `no-cache, stale-if-error=${cdnSettings.staleIfErrorSeconds}`;
                                                }
                                                lines.push(`Cache-Control: ${cacheControl}`);
                                                if (cdnSettings.cloudflareCdnCacheControl) {
                                                    lines.push(`Cloudflare-CDN-Cache-Control: ${cdnSettings.cloudflareCdnCacheControl}`);
                                                }
                                                for (const h of cdnSettings.customHeaders) {
                                                    if (h.name && h.value) {
                                                        lines.push(`${h.name}: ${h.value}`);
                                                    }
                                                }
                                                return lines.join("\n");
                                            })()}
                                        />
                                    </div>
                                </>
                            )}
                        </div>
                    </Card>
                )
            }
        </div>
    );
}
