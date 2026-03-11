"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
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
import { Plus, Pencil, Trash2, Loader2, Monitor, Settings2, FileText, Globe, Maximize2, X, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { type Monaco } from "@monaco-editor/react";
import type { editor } from "monaco-editor";
import { LazyMonacoEditor } from "./lazy-monaco";
import { useTheme } from "./theme-provider";
import { useEditorValidation } from "@/hooks/use-editor-validation";
import {
    getClients,
    addClient,
    updateClient,
    deleteClient,
    listClientFiles,
    createClientFile,
    updateClientFile,
    deleteClientFile,
    getClientFile,
    getConfig,
    ClientConfig,
} from "@/lib/api-client";
import { Transform, ScriptTransformer, ClientFileMeta } from "@/lib/schema";
import { createTransformByType, getTransformTypeUpdates } from "@/lib/transform-utils";
import { createListItemKey, createListItemKeys } from "@/lib/utils";

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
    const [transformKeys, setTransformKeys] = useState<string[]>([]);
    const [isSaving, setIsSaving] = useState(false);
    const [transformers, setTransformers] = useState<Record<string, ScriptTransformer>>({});
    const [selectedClientId, setSelectedClientId] = useState<string | null>(null);
    const [clientFiles, setClientFiles] = useState<ClientFileMeta[]>([]);
    const [isFilesLoading, setIsFilesLoading] = useState(false);
    const [isFileDialogOpen, setIsFileDialogOpen] = useState(false);
    const [editingFile, setEditingFile] = useState<ClientFileMeta | null>(null);
    const [fileForm, setFileForm] = useState({ configId: "", displayName: "", description: "", ext: "", isPublic: false });
    const [fileContent, setFileContent] = useState("");
    const [isFileSaving, setIsFileSaving] = useState(false);
    const [isFileLoading, setIsFileLoading] = useState(false);
    const [isFullscreenFileEditor, setIsFullscreenFileEditor] = useState(false);
    const [deletingFile, setDeletingFile] = useState<ClientFileMeta | null>(null);
    const [isDeletingFile, setIsDeletingFile] = useState(false);
    const [deletingClient, setDeletingClient] = useState<ClientConfig | null>(null);
    const [isDeletingClient, setIsDeletingClient] = useState(false);
    const { theme } = useTheme();
    const editorRef = useRef<editor.IStandaloneCodeEditor | null>(null);
    const monacoRef = useRef<Monaco | null>(null);

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

    const fetchClientFiles = async (clientId: string) => {
        setIsFilesLoading(true);
        try {
            const result = await listClientFiles(clientId);
            setClientFiles(result.files);
        } catch (error) {
            toast.error("获取配置文件失败: " + String(error));
        } finally {
            setIsFilesLoading(false);
        }
    };

    useEffect(() => {
        fetchClients();
    }, []);

    useEffect(() => {
        if (clients.length === 0) {
            setSelectedClientId(null);
            return;
        }
        if (!selectedClientId || !clients.some((c) => c.id === selectedClientId)) {
            setSelectedClientId(clients[0].id);
        }
    }, [clients, selectedClientId]);

    useEffect(() => {
        if (selectedClientId) {
            fetchClientFiles(selectedClientId);
        } else {
            setClientFiles([]);
        }
    }, [selectedClientId]);

    // ESC key exits fullscreen
    useEffect(() => {
        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.key === "Escape" && isFullscreenFileEditor) {
                setIsFullscreenFileEditor(false);
            }
        };
        window.addEventListener("keydown", handleKeyDown);
        return () => window.removeEventListener("keydown", handleKeyDown);
    }, [isFullscreenFileEditor]);

    const openAddDialog = () => {
        setEditingClient(null);
        setFormData({ id: "", displayName: "", pathName: "", transforms: [] });
        setTransformKeys([]);
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
        setTransformKeys(createListItemKeys(client.transforms?.length || 0));
        setIsDialogOpen(true);
    };

    const openCreateFileDialog = () => {
        if (!selectedClientId) return;
        setEditingFile(null);
        setFileForm({ configId: "", displayName: "", description: "", ext: "", isPublic: false });
        setFileContent("");
        setIsFileLoading(false);
        setIsFullscreenFileEditor(false);
        setIsFileDialogOpen(true);
    };

    const openEditFileDialog = async (file: ClientFileMeta) => {
        if (!selectedClientId) return;
        setEditingFile(file);
        setFileForm({
            configId: file.configId,
            displayName: file.displayName,
            description: file.description || "",
            ext: file.ext,
            isPublic: file.isPublic
        });
        setFileContent("");
        setIsFullscreenFileEditor(false);
        setIsFileDialogOpen(true);
        setIsFileLoading(true);
        try {
            const result = await getClientFile(selectedClientId, file.id);
            setFileContent(result.content || "");
        } catch (error) {
            toast.error("读取配置文件失败: " + String(error));
        } finally {
            setIsFileLoading(false);
        }
    };

    const addTransform = (type: "use" | "replace" | "remove_lines") => {
        setFormData((prev) => ({
            ...prev,
            transforms: [...prev.transforms, createTransformByType(type)],
        }));
        setTransformKeys((prev) => [...prev, createListItemKey()]);
    };

    const updateTransform = (index: number, updates: Partial<Transform>) => {
        setFormData((prev) => ({
            ...prev,
            transforms: prev.transforms.map((t, i) => (i === index ? { ...t, ...updates } : t)),
        }));
    };

    const removeTransform = (index: number) => {
        setFormData((prev) => ({
            ...prev,
            transforms: prev.transforms.filter((_, i) => i !== index),
        }));
        setTransformKeys((prev) => prev.filter((_, i) => i !== index));
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
                if (editingClient.id !== formData.id) {
                    setSelectedClientId(formData.id);
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

    const handleSaveFile = async () => {
        if (!selectedClientId) return;
        if (!fileForm.configId || !fileForm.displayName || !fileForm.ext) {
            toast.error("请填写配置 ID、显示名称和后缀");
            return;
        }

        if (!/^[a-zA-Z0-9_-]+$/.test(fileForm.configId)) {
            toast.error("配置 ID 只能包含字母、数字、连字符和下划线");
            return;
        }

        if (validation.hasErrors) {
            toast.error(`内容存在 ${validation.errors.length} 个语法错误，请修正后再保存`);
            return;
        }

        setIsFileSaving(true);
        try {
            if (editingFile) {
                await updateClientFile(selectedClientId, editingFile.id, {
                    configId: fileForm.configId,
                    displayName: fileForm.displayName,
                    description: fileForm.description,
                    ext: fileForm.ext,
                    isPublic: fileForm.isPublic,
                    content: fileContent,
                });
                toast.success("配置文件已更新");
            } else {
                await createClientFile(selectedClientId, {
                    configId: fileForm.configId,
                    displayName: fileForm.displayName,
                    description: fileForm.description,
                    ext: fileForm.ext,
                    isPublic: fileForm.isPublic,
                    content: fileContent,
                });
                toast.success("配置文件已创建");
            }
            setIsFileDialogOpen(false);
            await fetchClientFiles(selectedClientId);
        } catch (error) {
            toast.error(String(error));
        } finally {
            setIsFileSaving(false);
        }
    };

    const handleDeleteFile = async (file: ClientFileMeta) => {
        if (!selectedClientId) return;
        setDeletingFile(file);
    };

    const confirmDeleteFile = async () => {
        if (!selectedClientId || !deletingFile) return;
        setIsDeletingFile(true);
        try {
            await deleteClientFile(selectedClientId, deletingFile.id);
            toast.success("配置文件已删除");
            setDeletingFile(null);
            await fetchClientFiles(selectedClientId);
        } catch (error) {
            toast.error(String(error));
        } finally {
            setIsDeletingFile(false);
        }
    };

    const handleDelete = async (client: ClientConfig) => {
        setDeletingClient(client);
    };

    const confirmDeleteClient = async () => {
        if (!deletingClient) return;
        setIsDeletingClient(true);
        try {
            await deleteClient(deletingClient.id);
            toast.success("客户端已删除");
            if (selectedClientId === deletingClient.id) {
                setSelectedClientId(null);
            }
            setDeletingClient(null);
            await fetchClients();
            onRefresh?.();
        } catch (error) {
            toast.error(String(error));
        } finally {
            setIsDeletingClient(false);
        }
    };

    // 根据文件扩展名映射 Monaco Editor 语言
    const getEditorLanguage = (ext: string): string => {
        const extLower = ext.toLowerCase();
        const languageMap: Record<string, string> = {
            yaml: "yaml",
            yml: "yaml",
            json: "json",
            toml: "toml",
            conf: "ini",
            ini: "ini",
            txt: "plaintext",
        };
        return languageMap[extLower] || "plaintext";
    };

    const selectedClient = clients.find((client) => client.id === selectedClientId) || null;
    const editorTheme = theme === "dark" ? "vs-dark" : "light";
    const editorLanguage = getEditorLanguage(fileForm.ext);
    const validation = useEditorValidation(fileContent, editorLanguage);

    // 将语法错误标记到 Monaco Editor
    useEffect(() => {
        const model = editorRef.current?.getModel();
        const monaco = monacoRef.current;
        if (!model || !monaco) return;

        const markers: editor.IMarkerData[] = validation.errors.map((err) => ({
            severity: monaco.MarkerSeverity.Error,
            message: err.message,
            startLineNumber: err.line,
            startColumn: err.column,
            endLineNumber: err.line,
            endColumn: err.column + 1,
        }));
        monaco.editor.setModelMarkers(model, "syntax-validation", markers);

        return () => {
            if (model && !model.isDisposed()) {
                monaco.editor.setModelMarkers(model, "syntax-validation", []);
            }
        };
    }, [validation.errors]);

    const handleEditorMount = useCallback((ed: editor.IStandaloneCodeEditor, monaco: Monaco) => {
        editorRef.current = ed;
        monacoRef.current = monaco;
    }, []);

    if (isLoading) {
        return (
            <Card>
                <div className="flex items-center justify-center py-8">
                    <Loader2 className="w-6 h-6 animate-spin text-primary" />
                </div>
            </Card>
        );
    }

    // Fullscreen file editor mode
    if (isFullscreenFileEditor) {
        return (
            <div className="fixed inset-0 z-50 bg-background flex flex-col">
                <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-muted/50">
                    <div className="flex items-center gap-3">
                        <FileText className="w-5 h-5 text-primary" />
                        <span className="font-semibold text-foreground">
                            {editingFile ? "编辑配置文件" : "新建配置文件"}
                        </span>
                        {fileForm.configId && fileForm.ext && (
                            <Badge variant="outline" className="border-muted-foreground/30 text-muted-foreground font-mono">
                                {fileForm.configId}.{fileForm.ext}
                            </Badge>
                        )}
                    </div>
                    <div className="flex items-center gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setIsFullscreenFileEditor(false)}
                        >
                            取消
                        </Button>
                        <Button
                            size="sm"
                            onClick={handleSaveFile}
                            disabled={isFileLoading || isFileSaving}
                        >
                            {isFileSaving ? (
                                <>
                                    <Loader2 className="w-4 h-4 mr-1 animate-spin" />
                                    保存中
                                </>
                            ) : (
                                "保存"
                            )}
                        </Button>
                        <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => setIsFullscreenFileEditor(false)}
                            title="关闭全屏"
                        >
                            <X className="w-5 h-5" />
                        </Button>
                    </div>
                </div>

                <div className="flex-1 flex flex-col min-h-0">
                    <div className="flex-1 min-h-0">
                        <LazyMonacoEditor
                            height="100%"
                            language={editorLanguage}
                            value={fileContent}
                            onChange={(value) => setFileContent(value || "")}
                            theme={editorTheme}
                            onMount={handleEditorMount}
                            options={{
                                readOnly: isFileLoading || isFileSaving,
                                minimap: { enabled: true },
                                fontSize: 14,
                                lineNumbers: "on",
                                scrollBeyondLastLine: false,
                                automaticLayout: true,
                                tabSize: 2,
                                wordWrap: "off",
                                padding: { top: 16 },
                                scrollbar: {
                                    horizontal: "visible",
                                    vertical: "visible",
                                    horizontalScrollbarSize: 12,
                                    verticalScrollbarSize: 12,
                                },
                            }}
                        />
                    </div>
                    {validation.hasErrors && (
                        <div className="shrink-0 border-t border-border bg-destructive/5 px-4 py-2 flex items-start gap-2 max-h-28 overflow-y-auto">
                            <AlertTriangle className="w-4 h-4 text-destructive shrink-0 mt-0.5" />
                            <div className="text-sm space-y-0.5">
                                {validation.errors.map((err, i) => (
                                    <p key={i} className="text-destructive">
                                        <span className="font-mono text-xs opacity-70">行 {err.line}:{err.column}</span>{" "}
                                        {err.message}
                                    </p>
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            </div>
        );
    }

    return (
        <>
            <Card>
                <div className="px-5 pt-5 pb-3">
                    <div className="flex items-center justify-between">
                        <div>
                            <h3 className="text-base font-medium text-foreground flex items-center gap-2">
                                <Monitor className="w-5 h-5 text-primary" />
                                客户端管理
                            </h3>
                            <p className="text-sm text-muted-foreground">
                                管理代理客户端类型，添加新客户端或修改现有客户端
                            </p>
                        </div>
                        <Button onClick={openAddDialog} size="sm" className="shadow-sm">
                            <Plus className="w-4 h-4 mr-2" />
                            添加客户端
                        </Button>
                    </div>
                </div>
                <div className="px-5 pb-5">
                    {clients.length === 0 ? (
                        <div className="text-center py-16">
                            <div className="w-20 h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-muted/50 to-muted flex items-center justify-center">
                                <Monitor className="w-10 h-10 text-muted-foreground/40" />
                            </div>
                            <p className="text-lg font-medium text-foreground">暂无客户端配置</p>
                            <p className="text-sm text-muted-foreground mt-2 max-w-sm mx-auto">
                                客户端用于定义不同代理软件的规则输出格式和转换规则
                            </p>
                            <Button onClick={openAddDialog} className="mt-6">
                                <Plus className="w-4 h-4 mr-2" />
                                添加客户端
                            </Button>
                        </div>
                    ) : (
                        <div className="grid grid-cols-1 lg:grid-cols-[280px_1fr] gap-4">
                            <div className="space-y-2">
                                {clients.map((client, index) => {
                                    const isActive = selectedClientId === client.id;
                                    return (
                                        <button
                                            key={client.id}
                                            type="button"
                                            onClick={() => setSelectedClientId(client.id)}
                                            className={`w-full text-left p-3 rounded-lg border transition-all animate-in fade-in slide-in-from-left-4 ${isActive
                                                ? "border-primary/50 bg-primary/5 shadow-sm"
                                                : "border-transparent bg-muted/30 hover:bg-muted/50"
                                                }`}
                                            style={{ animationDelay: `${index * 50}ms`, animationFillMode: 'backwards' }}
                                        >
                                            <div className="flex items-start justify-between">
                                                <div>
                                                    <p className="font-medium text-foreground">{client.displayName}</p>
                                                    <p className="text-xs text-muted-foreground font-mono mt-0.5">
                                                        ID: {client.id}
                                                    </p>
                                                </div>
                                                {client.transforms && client.transforms.length > 0 && (
                                                    <Badge variant="secondary" className="text-[10px] h-5 px-1.5 font-normal">
                                                        <Settings2 className="w-3 h-3 mr-1" />
                                                        {client.transforms.length}
                                                    </Badge>
                                                )}
                                            </div>
                                            <p className="text-xs text-muted-foreground font-mono mt-2 truncate" title={`/Rules/${client.pathName}/`}>
                                                /Rules/{client.pathName}/
                                            </p>
                                        </button>
                                    );
                                })}
                            </div>
                            <div className="space-y-4">
                                {selectedClient ? (
                                    <>
                                        <div className="rounded-lg border bg-muted/20 p-4 flex flex-col gap-4">
                                            <div className="flex items-start justify-between gap-4">
                                                <div>
                                                    <div className="flex items-center gap-2">
                                                        <div className="w-10 h-10 rounded-md bg-primary/10 flex items-center justify-center">
                                                            <Monitor className="w-5 h-5 text-primary" />
                                                        </div>
                                                        <div>
                                                            <p className="text-lg font-semibold text-foreground">{selectedClient.displayName}</p>
                                                            <p className="text-xs text-muted-foreground font-mono">ID: {selectedClient.id}</p>
                                                        </div>
                                                    </div>
                                                    <div className="mt-3 space-y-1 text-xs text-muted-foreground font-mono">
                                                        <p title={`/Rules/${selectedClient.pathName}/`}>规则目录：/Rules/{selectedClient.pathName}/</p>
                                                        <p title="/client/">配置文件：/client/</p>
                                                    </div>
                                                </div>
                                                <div className="flex items-center gap-2">
                                                    <Button variant="outline" size="sm" onClick={() => openEditDialog(selectedClient)}>
                                                        <Pencil className="w-4 h-4 mr-1" />
                                                        编辑
                                                    </Button>
                                                    <Button
                                                        variant="outline"
                                                        size="sm"
                                                        className="text-destructive hover:text-destructive"
                                                        onClick={() => handleDelete(selectedClient)}
                                                    >
                                                        <Trash2 className="w-4 h-4 mr-1" />
                                                        删除
                                                    </Button>
                                                </div>
                                            </div>
                                        </div>

                                        <div className="rounded-lg border bg-card p-4">
                                            <div className="flex items-center justify-between">
                                                <div>
                                                    <p className="font-medium text-foreground flex items-center gap-2">
                                                        <FileText className="w-4 h-4 text-primary" />
                                                        配置文件
                                                    </p>
                                                    <p className="text-xs text-muted-foreground mt-1">
                                                        为该客户端创建可编辑的配置文件，可选择是否公开
                                                    </p>
                                                </div>
                                                <Button size="sm" onClick={openCreateFileDialog}>
                                                    <Plus className="w-4 h-4 mr-1" />
                                                    新建配置文件
                                                </Button>
                                            </div>

                                            <div className="mt-4 space-y-2">
                                                {isFilesLoading ? (
                                                    <div className="flex items-center justify-center py-6">
                                                        <Loader2 className="w-5 h-5 animate-spin text-primary" />
                                                    </div>
                                                ) : clientFiles.length === 0 ? (
                                                    <div className="text-center py-12">
                                                        <div className="w-16 h-16 mx-auto mb-4 rounded-xl bg-gradient-to-br from-muted/50 to-muted flex items-center justify-center">
                                                            <FileText className="w-8 h-8 text-muted-foreground/40" />
                                                        </div>
                                                        <p className="text-sm font-medium text-foreground">暂无配置文件</p>
                                                        <p className="text-xs text-muted-foreground mt-1">点击「新建配置文件」添加配置文件</p>
                                                    </div>
                                                ) : (
                                                    clientFiles.map((file, index) => (
                                                        <div
                                                            key={file.id}
                                                            className="flex items-center justify-between p-3 rounded-lg border border-border/60 bg-muted/20 animate-in fade-in slide-in-from-bottom-2"
                                                            style={{ animationDelay: `${index * 50}ms`, animationFillMode: 'backwards' }}
                                                        >
                                                            <div className="min-w-0">
                                                                <div className="flex items-center gap-2">
                                                                    <p className="font-medium text-foreground truncate">
                                                                        {file.displayName}
                                                                    </p>
                                                                    <p className="text-xs text-muted-foreground font-mono truncate">
                                                                        {file.configId}.{file.ext}
                                                                    </p>
                                                                    {file.isPublic && (
                                                                        <Badge variant="secondary" className="text-[10px] h-5 px-1.5 font-normal">
                                                                            <Globe className="w-3 h-3 mr-1" />
                                                                            公开
                                                                        </Badge>
                                                                    )}
                                                                </div>
                                                                {file.description && (
                                                                    <p className="text-xs text-muted-foreground mt-1 whitespace-pre-wrap break-words">
                                                                        {file.description}
                                                                    </p>
                                                                )}
                                                                <p className="text-xs text-muted-foreground font-mono mt-0.5 truncate">
                                                                    /client/{file.configId}.{file.ext}
                                                                </p>
                                                                <p className="text-[11px] text-muted-foreground mt-1">
                                                                    更新于 {new Date(file.updatedAt).toLocaleString("zh-CN")}
                                                                </p>
                                                            </div>
                                                            <div className="flex items-center gap-2">
                                                                <Button variant="ghost" size="icon" onClick={() => openEditFileDialog(file)} aria-label="编辑配置文件">
                                                                    <Pencil className="w-4 h-4" />
                                                                </Button>
                                                                <Button
                                                                    variant="ghost"
                                                                    size="icon"
                                                                    className="text-destructive hover:text-destructive"
                                                                    onClick={() => handleDeleteFile(file)}
                                                                    aria-label="删除配置文件"
                                                                >
                                                                    <Trash2 className="w-4 h-4" />
                                                                </Button>
                                                            </div>
                                                        </div>
                                                    ))
                                                )}
                                            </div>
                                        </div>
                                    </>
                                ) : (
                                    <div className="text-center py-12 text-muted-foreground bg-muted/10 rounded-lg border border-dashed border-border/50">
                                        <Monitor className="w-12 h-12 mx-auto text-muted-foreground/30 mb-3" />
                                        <p>请选择一个客户端</p>
                                    </div>
                                )}
                            </div>
                        </div>
                    )}
                </div>
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
                                onChange={(e) => setFormData((prev) => ({ ...prev, id: e.target.value }))}
                                placeholder="例如: surge"
                            />
                            <p className="text-xs text-muted-foreground">
                                唯一标识符，用于内部引用（可作为规则配置中的客户端 ID）
                            </p>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="displayName">显示名称</Label>
                            <Input
                                id="displayName"
                                value={formData.displayName}
                                onChange={(e) => setFormData((prev) => ({ ...prev, displayName: e.target.value }))}
                                placeholder="例如: Surge"
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="pathName">路径名称</Label>
                            <Input
                                id="pathName"
                                value={formData.pathName}
                                onChange={(e) => setFormData((prev) => ({ ...prev, pathName: e.target.value }))}
                                placeholder="例如: Surge"
                            />
                            <p className="text-xs text-muted-foreground">
                                用于 URL 路径: /Rules/{formData.pathName || "..."}/规则名.list
                            </p>
                        </div>

                        {/* 全局转换器配置 */}
                        <div className="space-y-2 pt-2 border-t border-border">
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
                            <p className="text-xs text-muted-foreground">
                                为该客户端配置全局转换器，规则默认会使用这些转换器
                            </p>

                            {/* 转换器列表 */}
                            <div className="space-y-2">
                                {formData.transforms.map((transform, index) => (
                                    <div
                                        key={transformKeys[index] ?? `transform-${index}`}
                                        className="p-3 rounded border border-border bg-muted/50 space-y-2"
                                    >
                                        <div className="flex items-center justify-between">
                                            <span className="text-sm font-medium">转换器 {index + 1}</span>
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="icon"
                                                onClick={() => removeTransform(index)}
                                                className="w-6 h-6 text-muted-foreground hover:text-destructive"
                                                aria-label="删除转换器"
                                            >
                                                <Trash2 className="w-3 h-3" />
                                            </Button>
                                        </div>

                                        <Select
                                            value={transform.type}
                                            onValueChange={(type: "use" | "replace" | "remove_lines") => {
                                                updateTransform(index, getTransformTypeUpdates(type));
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

            <Dialog open={isFileDialogOpen && !isFullscreenFileEditor} onOpenChange={setIsFileDialogOpen}>
                <DialogContent className="max-h-[85vh] flex flex-col p-0">
                    <DialogHeader className="shrink-0 px-6 pt-6 pb-4">
                        <DialogTitle>
                            {editingFile ? "编辑配置文件" : "新建配置文件"}
                        </DialogTitle>
                        <DialogDescription>
                            {selectedClient ? `客户端：${selectedClient.displayName}` : "请选择客户端"}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 px-6 overflow-y-auto flex-1 min-h-0">
                        <div className="grid grid-cols-2 gap-3">
                            <div className="space-y-2">
                                <Label htmlFor="file-config-id">配置 ID</Label>
                                <Input
                                    id="file-config-id"
                                    value={fileForm.configId}
                                    onChange={(e) => setFileForm({ ...fileForm, configId: e.target.value })}
                                    placeholder="例如: proxy"
                                    disabled={isFileLoading || isFileSaving}
                                />
                                <p className="text-[10px] text-muted-foreground">决定访问路径</p>
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="file-ext">文件后缀</Label>
                                <Input
                                    id="file-ext"
                                    value={fileForm.ext}
                                    onChange={(e) => setFileForm({ ...fileForm, ext: e.target.value })}
                                    placeholder="例如: yaml"
                                    disabled={isFileLoading || isFileSaving}
                                />
                            </div>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="file-display-name">显示名称</Label>
                            <Input
                                id="file-display-name"
                                value={fileForm.displayName}
                                onChange={(e) => setFileForm({ ...fileForm, displayName: e.target.value })}
                                placeholder="例如: 我的代理配置"
                                disabled={isFileLoading || isFileSaving}
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="file-description">配置文件描述</Label>
                            <Textarea
                                id="file-description"
                                value={fileForm.description}
                                onChange={(e) => setFileForm({ ...fileForm, description: e.target.value })}
                                placeholder="输入配置文件描述..."
                                className="min-h-[60px] text-sm"
                                disabled={isFileLoading || isFileSaving}
                            />
                        </div>
                        <div className="flex items-center justify-between rounded-lg border border-border/60 bg-muted/20 px-3 py-2">
                            <div className="space-y-0.5">
                                <p className="text-sm font-medium text-foreground">公开访问</p>
                                <p className="text-xs text-muted-foreground">开启后可通过公开 URL 访问</p>
                            </div>
                            <Switch
                                checked={fileForm.isPublic}
                                onCheckedChange={(checked) => setFileForm({ ...fileForm, isPublic: checked })}
                                disabled={isFileLoading || isFileSaving}
                            />
                        </div>
                        <div className="space-y-2">
                            <div className="flex items-center justify-between">
                                <Label htmlFor="file-content">内容</Label>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    onClick={() => setIsFullscreenFileEditor(true)}
                                    className="h-7 px-2"
                                    title="全屏编辑 (ESC 退出)"
                                >
                                    <Maximize2 className="w-3.5 h-3.5 mr-1" />
                                    全屏
                                </Button>
                            </div>
                            <div className="relative border border-border rounded-lg overflow-hidden">
                                <LazyMonacoEditor
                                    height="300px"
                                    language={editorLanguage}
                                    value={fileContent}
                                    onChange={(value) => setFileContent(value || "")}
                                    theme={editorTheme}
                                    onMount={handleEditorMount}
                                    options={{
                                        readOnly: isFileLoading || isFileSaving,
                                        minimap: { enabled: false },
                                        fontSize: 13,
                                        lineNumbers: "on",
                                        scrollBeyondLastLine: false,
                                        automaticLayout: true,
                                        tabSize: 2,
                                        wordWrap: "off",
                                        padding: { top: 12, bottom: 12 },
                                        scrollbar: {
                                            horizontal: "visible",
                                            vertical: "visible",
                                            horizontalScrollbarSize: 10,
                                            verticalScrollbarSize: 10,
                                        },
                                    }}
                                />
                            </div>
                            {validation.hasErrors && (
                                <div className="mt-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 flex items-start gap-2 max-h-24 overflow-y-auto">
                                    <AlertTriangle className="w-4 h-4 text-destructive shrink-0 mt-0.5" />
                                    <div className="text-sm space-y-0.5">
                                        {validation.errors.map((err, i) => (
                                            <p key={i} className="text-destructive">
                                                <span className="font-mono text-xs opacity-70">行 {err.line}:{err.column}</span>{" "}
                                                {err.message}
                                            </p>
                                        ))}
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                    <DialogFooter className="shrink-0 px-6 pb-6 pt-4">
                        <Button variant="outline" onClick={() => setIsFileDialogOpen(false)}>
                            取消
                        </Button>
                        <Button onClick={handleSaveFile} disabled={isFileSaving}>
                            {isFileSaving ? (
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

            {/* Delete File Confirm Dialog */}
            <Dialog open={!!deletingFile} onOpenChange={(open) => !open && setDeletingFile(null)}>
                <DialogContent className="max-w-md">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <AlertTriangle className="w-5 h-5 text-destructive" />
                            确认删除配置文件
                        </DialogTitle>
                        <DialogDescription>
                            确定要删除配置文件{" "}
                            <strong className="text-foreground">{deletingFile?.displayName}</strong> 吗？
                            <span className="block mt-1 text-destructive text-xs">此操作不可恢复。</span>
                        </DialogDescription>
                    </DialogHeader>
                    <div className="flex justify-end gap-3 mt-4">
                        <Button variant="outline" onClick={() => setDeletingFile(null)} disabled={isDeletingFile}>
                            取消
                        </Button>
                        <Button variant="destructive" onClick={confirmDeleteFile} disabled={isDeletingFile}>
                            {isDeletingFile ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <Trash2 className="w-4 h-4 mr-2" />}
                            确认删除
                        </Button>
                    </div>
                </DialogContent>
            </Dialog>

            {/* Delete Client Confirm Dialog */}
            <Dialog open={!!deletingClient} onOpenChange={(open) => !open && setDeletingClient(null)}>
                <DialogContent className="max-w-md">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <AlertTriangle className="w-5 h-5 text-destructive" />
                            确认删除客户端
                        </DialogTitle>
                        <DialogDescription>
                            确定要删除客户端{" "}
                            <strong className="text-foreground">{deletingClient?.displayName}</strong> 吗？
                            <span className="block mt-1 text-destructive text-xs">
                                这将删除所有相关规则文件，且无法恢复。
                            </span>
                        </DialogDescription>
                    </DialogHeader>
                    <div className="flex justify-end gap-3 mt-4">
                        <Button variant="outline" onClick={() => setDeletingClient(null)} disabled={isDeletingClient}>
                            取消
                        </Button>
                        <Button variant="destructive" onClick={confirmDeleteClient} disabled={isDeletingClient}>
                            {isDeletingClient ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <Trash2 className="w-4 h-4 mr-2" />}
                            确认删除
                        </Button>
                    </div>
                </DialogContent>
            </Dialog>
        </>
    );
}
