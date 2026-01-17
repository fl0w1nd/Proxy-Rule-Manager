"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { Plus, Pencil, Trash2, Loader2, Monitor } from "lucide-react";
import { toast } from "sonner";
import {
    getClients,
    addClient,
    updateClient,
    deleteClient,
    ClientConfig,
} from "@/lib/api-client";

interface ClientsManagerProps {
    onRefresh?: () => void;
}

export function ClientsManager({ onRefresh }: ClientsManagerProps) {
    const [clients, setClients] = useState<ClientConfig[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [isDialogOpen, setIsDialogOpen] = useState(false);
    const [editingClient, setEditingClient] = useState<ClientConfig | null>(null);
    const [formData, setFormData] = useState({ id: "", displayName: "", pathName: "" });
    const [isSaving, setIsSaving] = useState(false);

    const fetchClients = async () => {
        try {
            const result = await getClients();
            setClients(result.clients);
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
        setFormData({ id: "", displayName: "", pathName: "" });
        setIsDialogOpen(true);
    };

    const openEditDialog = (client: ClientConfig) => {
        setEditingClient(client);
        setFormData({ ...client });
        setIsDialogOpen(true);
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
                                        <p className="font-medium text-gray-900 dark:text-white">
                                            {client.displayName}
                                        </p>
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
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>
                            {editingClient ? "编辑客户端" : "添加客户端"}
                        </DialogTitle>
                        <DialogDescription>
                            {editingClient
                                ? "修改客户端信息。注意：修改路径名称会重命名目录并影响所有规则 URL。"
                                : "添加新的代理客户端类型。"}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
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
                    </div>
                    <DialogFooter>
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
