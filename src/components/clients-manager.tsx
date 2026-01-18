"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Plus, Pencil, Trash2, Loader2, Monitor, Settings2 } from "lucide-react";
import { toast } from "sonner";
import {
    getClients,
    addClient,
    updateClient,
    deleteClient,
    getConfig,
    ClientConfig,
} from "@/lib/api-client";
import { Transform, ScriptTransformer } from "@/lib/schema";

interface ClientsManagerProps {
    onRefresh?: () => void;
}

interface ClientFormData {
    id: string;
    displayName: string;
    pathName: string;
    transforms: Transform[];
}

export function ClientsManager({ onRefresh }: ClientsManagerProps) {
    const [clients, setClients] = useState<ClientConfig[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [isDialogOpen, setIsDialogOpen] = useState(false);
    const [editingClient, setEditingClient] = useState<ClientConfig | null>(null);
    const [formData, setFormData] = useState<ClientFormData>({ id: "", displayName: "", pathName: "", transforms: [] });
    const [isSaving, setIsSaving] = useState(false);
    const [transformers, setTransformers] = useState<Record<string, ScriptTransformer>>({});

    const fetchClients = async () => {
        try {
            const [clientsResult, configResult] = await Promise.all([
                getClients(),
                getConfig(),
            ]);
            setClients(clientsResult.clients);
            setTransformers(configResult.config.transformers || {});
        } catch (error) {
            toast.error("获取客户端列表失败: " + String(error));
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        fetchClients();
    }, []);

    const openAddDialog = () => {
        setEditingClient(null);
        setFormData({ id: "", displayName: "", pathName: "", transforms: [] });
        setIsDialogOpen(true);
    };

    const openEditDialog = (client: ClientConfig) => {
        setEditingClient(client);
        setFormData({
            id: client.id,
            displayName: client.displayName,
            pathName: client.pathName,
            transforms: client.transforms || [],
        });
        setIsDialogOpen(true);
    };

    const addTransform = (type: "use" | "replace" | "remove_lines") => {
        const newTransform: Transform = { type, target: "all" };
        if (type === "replace") {
            newTransform.pattern = "";
            newTransform.replacement = "";
        } else if (type === "remove_lines") {
            newTransform.pattern = "";
        }
        setFormData({ ...formData, transforms: [...formData.transforms, newTransform] });
    };

    const updateTransform = (index: number, updates: Partial<Transform>) => {
        setFormData({
            ...formData,
            transforms: formData.transforms.map((t, i) => (i === index ? { ...t, ...updates } : t)),
        });
    };

    const removeTransform = (index: number) => {
        setFormData({
            ...formData,
            transforms: formData.transforms.filter((_, i) => i !== index),
        });
    };

    const handleSave = async () => {
        if (!formData.id || !formData.displayName || !formData.pathName) {
            toast.error("请填写所有字段");
            return;
        }

        setIsSaving(true);
        try {
            if (editingClient) {
                const result = await updateClient(editingClient.id, formData);
                if (result.renamedPath) {
                    toast.success(`客户端已更新，目录已从 "${result.renamedPath.from}" 重命名为 "${result.renamedPath.to}"`);
                } else {
                    toast.success("客户端已更新");
                }
            } else {
                await addClient(formData);
                toast.success("客户端已添加");
            }
            setIsDialogOpen(false);
            await fetchClients();
            onRefresh?.();
        } catch (error) {
            toast.error(String(error));
        } finally {
            setIsSaving(false);
        }
    };

    const handleDelete = async (client: ClientConfig) => {
        if (!confirm(`确定要删除客户端 "${client.displayName}" 吗？这将删除所有相关规则文件！`)) {
            return;
        }

        try {
            await deleteClient(client.id);
            toast.success("客户端已删除");
            await fetchClients();
            onRefresh?.();
        } catch (error) {
            toast.error(String(error));
        }
    };

    if (isLoading) {
        return (
            <Card className="bg-white dark:bg-slate-800">
                <CardContent className="flex items-center justify-center py-8">
                    <Loader2 className="w-6 h-6 animate-spin text-blue-500" />
                </CardContent>
            </Card>
        );
    }

    return (
        <>
            <Card className="bg-white dark:bg-slate-800 border-gray-200 dark:border-slate-700">
                <CardHeader>
                    <div className="flex items-center justify-between">
                        <div>
                            <CardTitle className="text-gray-900 dark:text-white flex items-center gap-2">
                                <Monitor className="w-5 h-5 text-blue-500" />
                                客户端管理
                            </CardTitle>
                            <CardDescription className="text-gray-500 dark:text-gray-400">
                                管理代理客户端类型，添加新客户端或修改现有客户端
                            </CardDescription>
                        </div>
                        <Button onClick={openAddDialog} size="sm">
                            <Plus className="w-4 h-4 mr-2" />
                            添加客户端
                        </Button>
                    </div>
                </CardHeader>
                <CardContent>
                    <div className="space-y-3">
                        {clients.map((client) => (
                            <div
                                key={client.id}
                                className="flex items-center justify-between p-4 rounded-lg bg-gray-50 dark:bg-slate-900"
                            >
                                <div className="flex items-center gap-4">
                                    <div className="w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-900/30 flex items-center justify-center">
                                        <Monitor className="w-5 h-5 text-blue-500" />
                                    </div>
                                    <div>
                                        <div className="flex items-center gap-2">
                                            <p className="font-medium text-gray-900 dark:text-white">
                                                {client.displayName}
                                            </p>
                                            {client.transforms && client.transforms.length > 0 && (
                                                <Badge variant="secondary" className="text-xs">
                                                    <Settings2 className="w-3 h-3 mr-1" />
                                                    {client.transforms.length} 个转换器
                                                </Badge>
                                            )}
                                        </div>
                                        <p className="text-xs text-gray-500 dark:text-gray-400">
                                            ID: {client.id} | 路径: /Rules/{client.pathName}/
                                        </p>
                                    </div>
                                </div>
                                <div className="flex items-center gap-2">
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        onClick={() => openEditDialog(client)}
                                    >
                                        <Pencil className="w-4 h-4" />
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        size="icon"
                                        onClick={() => handleDelete(client)}
                                        className="text-red-500 hover:text-red-600"
                                    >
                                        <Trash2 className="w-4 h-4" />
                                    </Button>
                                </div>
                            </div>
                        ))}
                        {clients.length === 0 && (
                            <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                                暂无客户端配置
                            </div>
                        )}
                    </div>
                </CardContent>
            </Card>

            <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
                <DialogContent className="max-h-[85vh] flex flex-col p-0">
                    <DialogHeader className="shrink-0 px-6 pt-6 pb-4">
                        <DialogTitle>
                            {editingClient ? "编辑客户端" : "添加客户端"}
                        </DialogTitle>
                        <DialogDescription>
                            {editingClient
                                ? "修改客户端信息。注意：修改路径名称会重命名目录并影响所有规则 URL。"
                                : "添加新的代理客户端类型。"}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 px-6 overflow-y-auto flex-1 min-h-0">
                        <div className="space-y-2">
                            <Label htmlFor="id">客户端 ID</Label>
                            <Input
                                id="id"
                                value={formData.id}
                                onChange={(e) => setFormData({ ...formData, id: e.target.value })}
                                placeholder="例如: surge"
                            />
                            <p className="text-xs text-gray-500">
                                唯一标识符，用于内部引用（可作为规则配置中的客户端 ID）
                            </p>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="displayName">显示名称</Label>
                            <Input
                                id="displayName"
                                value={formData.displayName}
                                onChange={(e) => setFormData({ ...formData, displayName: e.target.value })}
                                placeholder="例如: Surge"
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="pathName">路径名称</Label>
                            <Input
                                id="pathName"
                                value={formData.pathName}
                                onChange={(e) => setFormData({ ...formData, pathName: e.target.value })}
                                placeholder="例如: Surge"
                            />
                            <p className="text-xs text-gray-500">
                                用于 URL 路径: /Rules/{formData.pathName || "..."}/规则名.list
                            </p>
                        </div>

                        {/* 全局转换器配置 */}
                        <div className="space-y-2 pt-2 border-t border-gray-200 dark:border-slate-700">
                            <div className="flex items-center justify-between">
                                <Label className="flex items-center gap-2">
                                    <Settings2 className="w-4 h-4" />
                                    全局转换器
                                </Label>
                                {formData.transforms.length > 0 && (
                                    <Badge variant="secondary" className="text-xs">
                                        {formData.transforms.length} 个
                                    </Badge>
                                )}
                            </div>
                            <p className="text-xs text-gray-500">
                                为该客户端配置全局转换器，规则默认会使用这些转换器
                            </p>

                            {/* 转换器列表 */}
                            <div className="space-y-2">
                                {formData.transforms.map((transform, index) => (
                                    <div
                                        key={index}
                                        className="p-3 rounded border border-gray-200 dark:border-slate-700 bg-gray-50 dark:bg-slate-800 space-y-2"
                                    >
                                        <div className="flex items-center justify-between">
                                            <span className="text-sm font-medium">转换器 {index + 1}</span>
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="icon"
                                                onClick={() => removeTransform(index)}
                                                className="w-6 h-6 text-gray-400 hover:text-red-500"
                                            >
                                                <Trash2 className="w-3 h-3" />
                                            </Button>
                                        </div>

                                        <Select
                                            value={transform.type}
                                            onValueChange={(type: "use" | "replace" | "remove_lines") => {
                                                const newTransform: Partial<Transform> = { type };
                                                if (type === "replace") {
                                                    newTransform.pattern = "";
                                                    newTransform.replacement = "";
                                                }
                                                if (type === "remove_lines") {
                                                    newTransform.pattern = "";
                                                }
                                                updateTransform(index, newTransform);
                                            }}
                                        >
                                            <SelectTrigger className="h-8">
                                                <SelectValue />
                                            </SelectTrigger>
                                            <SelectContent>
                                                {Object.keys(transformers).length > 0 && (
                                                    <SelectItem value="use">预定义转换器</SelectItem>
                                                )}
                                                <SelectItem value="replace">正则替换</SelectItem>
                                                <SelectItem value="remove_lines">正则删除</SelectItem>
                                            </SelectContent>
                                        </Select>

                                        {transform.type === "use" && (
                                            <Select
                                                value={transform.use || ""}
                                                onValueChange={(value) => updateTransform(index, { use: value })}
                                            >
                                                <SelectTrigger className="h-8">
                                                    <SelectValue placeholder="选择转换器" />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    {Object.entries(transformers).map(([name]) => (
                                                        <SelectItem key={name} value={name}>
                                                            {name}
                                                        </SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                        )}

                                        {transform.type === "replace" && (
                                            <div className="grid grid-cols-2 gap-2">
                                                <Input
                                                    value={transform.pattern || ""}
                                                    onChange={(e) => updateTransform(index, { pattern: e.target.value })}
                                                    placeholder="正则表达式"
                                                    className="h-8 text-sm"
                                                />
                                                <Input
                                                    value={transform.replacement || ""}
                                                    onChange={(e) => updateTransform(index, { replacement: e.target.value })}
                                                    placeholder="替换为"
                                                    className="h-8 text-sm"
                                                />
                                            </div>
                                        )}

                                        {transform.type === "remove_lines" && (
                                            <Input
                                                value={transform.pattern || ""}
                                                onChange={(e) => updateTransform(index, { pattern: e.target.value })}
                                                placeholder="正则表达式"
                                                className="h-8 text-sm"
                                            />
                                        )}
                                    </div>
                                ))}
                            </div>

                            {/* 添加转换器按钮 */}
                            <div className="flex flex-wrap gap-2">
                                {Object.keys(transformers).length > 0 && (
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        onClick={() => addTransform("use")}
                                    >
                                        <Plus className="w-3 h-3 mr-1" />
                                        预定义转换器
                                    </Button>
                                )}
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    onClick={() => addTransform("replace")}
                                >
                                    <Plus className="w-3 h-3 mr-1" />
                                    正则替换
                                </Button>
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    onClick={() => addTransform("remove_lines")}
                                >
                                    <Plus className="w-3 h-3 mr-1" />
                                    正则删除
                                </Button>
                            </div>
                        </div>
                    </div>
                    <DialogFooter className="shrink-0 px-6 pb-6 pt-4">
                        <Button variant="outline" onClick={() => setIsDialogOpen(false)}>
                            取消
                        </Button>
                        <Button onClick={handleSave} disabled={isSaving}>
                            {isSaving ? (
                                <>
                                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                    保存中...
                                </>
                            ) : (
                                "保存"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    );
}
