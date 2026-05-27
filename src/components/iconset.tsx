"use client";

import { useState, useEffect, useRef, startTransition } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import NextImage from "next/image";

import { Trash2, Loader2, Image as ImageIcon, Upload, Copy, Pencil, Check, X } from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";
import { toast } from "sonner";
import { listIcons, uploadIcons, renameIcon, deleteIcon, IconMeta } from "@/lib/api-client";

function formatFileSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

export function IconSetManager() {
    const [icons, setIcons] = useState<IconMeta[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [isUploadDialogOpen, setIsUploadDialogOpen] = useState(false);
    const [isUploading, setIsUploading] = useState(false);
    const [editingIconId, setEditingIconId] = useState<string | null>(null);
    const [editingName, setEditingName] = useState("");
    const [isSavingRename, setIsSavingRename] = useState(false);
    const [deleteConfirmIcon, setDeleteConfirmIcon] = useState<IconMeta | null>(null);
    const [isDeleting, setIsDeleting] = useState(false);
    const [isDragging, setIsDragging] = useState(false);
    const fileInputRef = useRef<HTMLInputElement>(null);
    const editInputRef = useRef<HTMLInputElement>(null);

    const fetchIcons = async () => {
        try {
            const result = await listIcons();
            setIcons(result.icons);
        } catch (error) {
            toast.error("获取图标列表失败: " + String(error));
        } finally {
            setIsLoading(false);
        }
    };

    useEffect(() => {
        startTransition(() => { fetchIcons(); });
    }, []);

    useEffect(() => {
        if (editingIconId && editInputRef.current) {
            editInputRef.current.focus();
            editInputRef.current.select();
        }
    }, [editingIconId]);

    const openUploadDialog = () => {
        setIsUploadDialogOpen(true);
    };

    const handleFileSelect = async (files: FileList | null) => {
        if (!files || files.length === 0) return;

        setIsUploading(true);
        try {
            const result = await uploadIcons(files);
            if (result.renamed.length > 0) {
                const renamedList = result.renamed.map((r) => `${r.original} → ${r.renamed}`).join(", ");
                toast.warning(`部分文件已重命名: ${renamedList}`);
            }
            toast.success(`成功上传 ${result.uploaded.length} 个图标`);
            setIsUploadDialogOpen(false);
            if (fileInputRef.current) {
                fileInputRef.current.value = "";
            }
            await fetchIcons();
        } catch (error) {
            toast.error("上传失败: " + String(error));
        } finally {
            setIsUploading(false);
        }
    };

    const startRename = (icon: IconMeta) => {
        setEditingIconId(icon.id);
        setEditingName(icon.name);
    };

    const cancelRename = () => {
        setEditingIconId(null);
        setEditingName("");
    };

    const saveRename = async (icon: IconMeta) => {
        if (!editingName.trim()) {
            toast.error("名称不能为空");
            return;
        }

        const ext = icon.id.match(/\.[^/.]+$/)?.[0] || "";
        const newFullName = editingName.trim() + ext;

        if (newFullName === icon.id) {
            cancelRename();
            return;
        }

        setIsSavingRename(true);
        try {
            await renameIcon(icon.id, newFullName);
            toast.success("重命名成功");
            cancelRename();
            await fetchIcons();
        } catch (error) {
            toast.error("重命名失败: " + String(error));
        } finally {
            setIsSavingRename(false);
        }
    };

    const handleDelete = async () => {
        if (!deleteConfirmIcon) return;

        setIsDeleting(true);
        try {
            await deleteIcon(deleteConfirmIcon.id);
            toast.success("图标已删除");
            setDeleteConfirmIcon(null);
            await fetchIcons();
        } catch (error) {
            toast.error("删除失败: " + String(error));
        } finally {
            setIsDeleting(false);
        }
    };

    const copyUrl = async (url: string) => {
        try {
            // Use URL constructor to safely resolve both relative and absolute URLs
            const fullUrl = new URL(url, window.location.origin).href;
            await navigator.clipboard.writeText(fullUrl);
            toast.success("链接已复制");
        } catch {
            toast.error("复制失败");
        }
    };

    return (
        <>
            <Card>
                <div className="px-5 pt-5 pb-3">
                    <div className="flex items-center justify-between">
                        <div>
                            <h3 className="text-base font-medium text-foreground flex items-center gap-2">
                                <ImageIcon className="w-5 h-5" />
                                图标集
                            </h3>
                            <p className="text-sm text-muted-foreground">上传和管理图标文件</p>
                        </div>
                        <Button onClick={openUploadDialog}>
                            <Upload className="w-4 h-4 mr-2" />
                            上传图标
                        </Button>
                    </div>
                </div>
                <div className="px-5 pb-5">
                    {isLoading ? (
                        <div className="flex items-center justify-center py-12">
                            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
                        </div>
                    ) : icons.length === 0 ? (
                    <EmptyState
                      icon={ImageIcon}
                      title="暂无图标"
                      description="点击上方按钮上传图标"
                    />
                    ) : (
                        <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-4">
                            {icons.map((icon) => (
                                <div
                                    key={icon.id}
                                    className="group relative flex flex-col rounded-2xl border border-border/50 bg-card p-3 shadow-[var(--shadow-xs)] transition-all duration-200 hover:border-border hover:shadow-[var(--shadow-sm)]"
                                >
                                    <div className="relative mb-3 flex aspect-square w-full items-center justify-center overflow-hidden rounded-xl border border-border/50 bg-surface-subtle">
                                        <NextImage
                                            src={icon.url}
                                            alt={icon.name}
                                            fill
                                            sizes="(max-width: 640px) 40vw, (max-width: 1024px) 20vw, 10vw"
                                            unoptimized
                                            className="object-contain"
                                        />
                                    </div>

                                    {editingIconId === icon.id ? (
                                        <div className="flex items-center gap-1">
                                            <Input
                                                ref={editInputRef}
                                                value={editingName}
                                                onChange={(e) => setEditingName(e.target.value)}
                                                onKeyDown={(e) => {
                                                    if (e.key === "Enter") {
                                                        saveRename(icon);
                                                    } else if (e.key === "Escape") {
                                                        cancelRename();
                                                    }
                                                }}
                                                className="h-7 text-xs"
                                                disabled={isSavingRename}
                                            />
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="h-7 w-7 shrink-0"
                                                onClick={() => saveRename(icon)}
                                                disabled={isSavingRename}
                                                aria-label="确认重命名"
                                            >
                                                {isSavingRename ? (
                                                    <Loader2 className="w-3 h-3 animate-spin" />
                                                ) : (
                                                    <Check className="w-3 h-3" />
                                                )}
                                            </Button>
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="h-7 w-7 shrink-0"
                                                onClick={cancelRename}
                                                disabled={isSavingRename}
                                                aria-label="取消重命名"
                                            >
                                                <X className="w-3 h-3" />
                                            </Button>
                                        </div>
                                    ) : (
                                        <button
                                            onClick={() => startRename(icon)}
                                            className="flex items-center gap-1 text-left transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-primary/15 group/name"
                                        >
                                            <span className="text-xs font-medium truncate flex-1" title={icon.name}>
                                                {icon.name}
                                            </span>
                                            <Pencil className="w-3 h-3 opacity-0 group-hover/name:opacity-100 shrink-0" />
                                        </button>
                                    )}

                                    <p className="text-[11px] text-muted-foreground mt-1">
                                        {formatFileSize(icon.size)}
                                    </p>

                                    <div className="flex items-center gap-1 mt-2">
                                        <div className="flex-1 min-w-0">
                                            <p
                                                className="text-[11px] text-muted-foreground truncate"
                                                title={icon.url}
                                            >
                                                {icon.url}
                                            </p>
                                        </div>
                                        <Button
                                            variant="ghost"
                                            size="icon"
                                            className="h-6 w-6 shrink-0"
                                            onClick={() => copyUrl(icon.url)}
                                            title="复制链接"
                                        >
                                            <Copy className="w-3 h-3" />
                                        </Button>
                                    </div>

                                        <Button
                                            variant="ghost"
                                            size="icon"
                                            className="absolute right-2 top-2 h-6 w-6 bg-card/90 opacity-0 shadow-[var(--shadow-xs)] backdrop-blur transition-opacity group-hover:opacity-100 text-destructive hover:text-destructive hover:bg-destructive/10"
                                            onClick={() => setDeleteConfirmIcon(icon)}
                                            title="删除"
                                        >
                                        <Trash2 className="w-3 h-3" />
                                    </Button>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </Card>

            <Dialog open={isUploadDialogOpen} onOpenChange={setIsUploadDialogOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>上传图标</DialogTitle>
                        <DialogDescription>选择图片文件上传到图标集</DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4">
                        <div
                            className={`relative rounded-2xl border-2 border-dashed p-8 text-center transition-all ${
                                isDragging
                                    ? "border-primary/50 bg-primary-soft"
                                    : "border-border/70 bg-surface-subtle/60 hover:border-primary/30 hover:bg-accent/20"
                            } ${isUploading ? "pointer-events-none opacity-60" : "cursor-pointer"}`}
                            onDragOver={(e) => {
                                e.preventDefault();
                                setIsDragging(true);
                            }}
                            onDragLeave={(e) => {
                                e.preventDefault();
                                // Only clear when leaving the drop zone itself, not a child element
                                if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                                    setIsDragging(false);
                                }
                            }}
                            onDrop={(e) => {
                                e.preventDefault();
                                setIsDragging(false);
                                if (e.dataTransfer.files.length > 0) {
                                    handleFileSelect(e.dataTransfer.files);
                                }
                            }}
                            onClick={() => fileInputRef.current?.click()}
                        >
                            <input
                                ref={fileInputRef}
                                type="file"
                                accept="image/*"
                                multiple
                                className="hidden"
                                onChange={(e) => handleFileSelect(e.target.files)}
                                disabled={isUploading}
                            />
                            <div className="flex flex-col items-center gap-2">
                                {isUploading ? (
                                    <Loader2 className="w-10 h-10 text-primary animate-spin" />
                                ) : (
                                    <Upload className="w-10 h-10 text-muted-foreground" />
                                )}
                                <div className="space-y-1">
                                    <p className="text-sm font-medium">
                                        {isUploading ? "上传中..." : "拖拽图片到此处，或点击选择"}
                                    </p>
                                    <p className="text-xs text-muted-foreground">
                                        支持 PNG, JPG, GIF, SVG, WebP 等格式，可批量上传
                                    </p>
                                </div>
                            </div>
                        </div>
                        <p className="text-xs text-muted-foreground text-center">
                            如果文件名重复将自动重命名为副本
                        </p>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setIsUploadDialogOpen(false)} disabled={isUploading}>
                            取消
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            <Dialog open={!!deleteConfirmIcon} onOpenChange={(open) => !open && setDeleteConfirmIcon(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>确认删除</DialogTitle>
                        <DialogDescription>
                            确定要删除图标 &quot;{deleteConfirmIcon?.name}&quot; 吗？此操作无法撤销。
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setDeleteConfirmIcon(null)} disabled={isDeleting}>
                            取消
                        </Button>
                        <Button
                            variant="destructive"
                            onClick={handleDelete}
                            disabled={isDeleting}
                        >
                            {isDeleting ? (
                                <>
                                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                                    删除中...
                                </>
                            ) : (
                                "删除"
                            )}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    );
}
