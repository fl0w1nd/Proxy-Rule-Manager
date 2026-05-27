"use client";

import { useState, useEffect, startTransition } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import {
  Plus,
  Trash2,
  Edit3,
  Code2,
  HelpCircle,
  Loader2,
  Play,
  AlertTriangle,
  BookOpen,
  Lock,
} from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";
import { getConfig, saveConfig } from "@/lib/api-client";
import { RulesConfig, ScriptTransformer, BuiltinTransformer } from "@/lib/schema";
import { toast } from "sonner";

interface TransformersManagerProps {
  onRefresh?: () => void;
}

// 帮助文档内容
const HELP_DOC = `
## 预定义转换器脚本说明

预定义转换器使用 JavaScript 脚本来处理规则内容。脚本必须包含一个 \`transform\` 函数。

### 函数签名

\`\`\`javascript
function transform(content) {
  // content: string - 输入的规则内容
  // 返回值: string - 处理后的规则内容
  return processedContent;
}
\`\`\`

### 常用操作示例

#### 1. 删除注释行
\`\`\`javascript
function transform(content) {
  return content
    .split('\\n')
    .filter(line => !line.trim().startsWith('#'))
    .join('\\n');
}
\`\`\`

#### 2. 删除空行
\`\`\`javascript
function transform(content) {
  return content
    .split('\\n')
    .filter(line => line.trim())
    .join('\\n');
}
\`\`\`

#### 3. 去重
\`\`\`javascript
function transform(content) {
  const lines = content.split('\\n');
  return [...new Set(lines)].join('\\n');
}
\`\`\`

#### 4. 排序
\`\`\`javascript
function transform(content) {
  return content
    .split('\\n')
    .sort()
    .join('\\n');
}
\`\`\`

#### 5. 正则替换
\`\`\`javascript
function transform(content) {
  return content.replace(/DOMAIN,/g, 'DOMAIN-SUFFIX,');
}
\`\`\`

#### 6. 添加/删除前缀
\`\`\`javascript
// 添加前缀
function transform(content) {
  return content
    .split('\\n')
    .map(line => line.trim() ? 'DOMAIN-SUFFIX,' + line : '')
    .filter(Boolean)
    .join('\\n');
}
\`\`\`

#### 7. 组合多个操作
\`\`\`javascript
function transform(content) {
  return content
    .split('\\n')
    .map(line => line.trim())         // 去除首尾空白
    .filter(line => line && !line.startsWith('#'))  // 过滤空行和注释
    .map(line => line.toLowerCase())  // 转小写
    .sort()                           // 排序
    .filter((line, i, arr) => arr.indexOf(line) === i)  // 去重
    .join('\\n');
}
\`\`\`

### 注意事项

1. 脚本在服务端沙箱环境中执行，无法访问文件系统或网络
2. 脚本执行有超时限制（5秒）
3. 如果脚本出错，将返回原始内容并记录错误
4. 建议在保存前先测试脚本功能
`;

export function TransformersManager({ onRefresh }: TransformersManagerProps) {
  const [config, setConfig] = useState<RulesConfig | null>(null);
  const [builtins, setBuiltins] = useState<BuiltinTransformer[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [editingTransformer, setEditingTransformer] = useState<{
    name: string;
    data: ScriptTransformer;
    isNew: boolean;
  } | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [deletingTransformer, setDeletingTransformer] = useState<string | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [showHelp, setShowHelp] = useState(false);
  const [testInput, setTestInput] = useState("");
  const [testOutput, setTestOutput] = useState("");
  const [testError, setTestError] = useState("");

  const fetchConfig = async () => {
    try {
      const { config, builtinTransformers } = await getConfig();
      setConfig(config);
      setBuiltins(builtinTransformers ?? []);
    } catch (error) {
      console.error("Failed to fetch config:", error);
      toast.error("获取配置失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    startTransition(() => { fetchConfig(); });
  }, []);

  const formatSaveError = (error: unknown) => {
    const err = error as Error & { code?: string };
    if (err.code === "CONFIG_CONFLICT") {
      return "配置已被其他操作更新，请重新加载后再保存";
    }
    return String(error);
  };

  const handleCreate = () => {
    setEditingTransformer({
      name: "",
      data: {
        name: "",
        description: "",
        script: `function transform(content) {\n  // 在这里编写转换逻辑\n  return content;\n}`,
        createdAt: new Date().toISOString(),
      },
      isNew: true,
    });
  };

  const handleEdit = (name: string) => {
    const transformer = config?.transformers?.[name];
    if (transformer) {
      setEditingTransformer({
        name,
        data: { ...transformer },
        isNew: false,
      });
    }
  };

  const handleSave = async () => {
    if (!editingTransformer || !config) return;

    const { name, data, isNew } = editingTransformer;

    if (!data.name.trim()) {
      toast.error("转换器名称不能为空");
      return;
    }

    if (data.name.trim().startsWith("builtin:")) {
      toast.error("名称不能以 \"builtin:\" 开头，该前缀保留给内置转换器");
      return;
    }

    if (!data.script.trim()) {
      toast.error("脚本内容不能为空");
      return;
    }

    // Syntax-only validation: the function is constructed but never called here.
    try {
      new Function("content", data.script + "\nreturn transform(content);");
    } catch (e) {
      toast.error("脚本语法错误: " + String(e));
      return;
    }

    if (isSaving) return;
    setIsSaving(true);
    try {
      const { config: latestConfig, rev } = await getConfig();
      const newTransformers = { ...(latestConfig.transformers || {}) };

      // 如果名称变更，删除旧的
      if (!isNew && name !== data.name) {
        delete newTransformers[name];
      }

      newTransformers[data.name] = {
        ...data,
        updatedAt: new Date().toISOString(),
      };

      await saveConfig({
        ...latestConfig,
        transformers: newTransformers,
      }, rev);

      toast.success("转换器保存成功");
      setEditingTransformer(null);
      await fetchConfig();
      onRefresh?.();
    } catch (error) {
      toast.error("保存失败: " + formatSaveError(error));
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async (name: string) => {
    if (!config) return;

    setIsDeleting(true);
    try {
      const { config: latestConfig, rev } = await getConfig();
      const newTransformers = { ...(latestConfig.transformers || {}) };
      delete newTransformers[name];

      await saveConfig({
        ...latestConfig,
        transformers: newTransformers,
      }, rev);

      toast.success("转换器已删除");
      setDeletingTransformer(null);
      await fetchConfig();
      onRefresh?.();
    } catch (error) {
      toast.error("删除失败: " + formatSaveError(error));
    } finally {
      setIsDeleting(false);
    }
  };

  const handleTest = () => {
    if (!editingTransformer) return;

    setTestError("");
    setTestOutput("");

    try {
      // Shadow common browser globals to limit accidental side-effects in the
      // test sandbox. This is admin-only code, but defence-in-depth helps.
      const fn = new Function(
        "content",
        "fetch", "XMLHttpRequest", "WebSocket",
        "localStorage", "sessionStorage", "indexedDB",
        "document", "window",
        editingTransformer.data.script + "\nreturn transform(content);"
      );
      const result = fn.call(
        null,
        testInput,
        undefined, undefined, undefined,
        undefined, undefined, undefined,
        undefined, undefined
      );
      setTestOutput(String(result));
    } catch (e) {
      setTestError(String(e));
    }
  };

  const transformers = config?.transformers || {};
  const transformerList = Object.entries(transformers);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin text-primary" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 头部操作 */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-foreground flex items-center gap-2">
            <Code2 className="w-5 h-5 text-primary" />
            预定义转换器
          </h2>
          <p className="text-sm text-muted-foreground mt-1">
            使用 JavaScript 脚本创建可复用的规则转换器
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => setShowHelp(true)}>
            <BookOpen className="w-4 h-4 mr-1" />
            帮助文档
          </Button>
          <Button onClick={handleCreate}>
            <Plus className="w-4 h-4 mr-1" />
            新建转换器
          </Button>
        </div>
      </div>

      {/* 内置转换器（只读） */}
      {builtins.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-baseline justify-between">
            <h3 className="text-sm font-semibold text-foreground/80 flex items-center gap-1.5">
              <Lock className="w-3.5 h-3.5 text-muted-foreground" />
              内置转换器
            </h3>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {builtins.map((b) => (
              <div
                key={b.name}
                className="group relative overflow-hidden rounded-2xl border border-dashed border-border bg-surface-subtle/40 shadow-[var(--shadow-xs)]"
              >
                <div className="px-5 py-5 z-10 relative">
                  <div className="flex items-start gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-muted/40 bg-muted/30 text-muted-foreground">
                      <Lock className="w-4 h-4" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <h3 className="text-[15px] font-semibold text-foreground font-mono truncate" title={b.name}>
                        {b.name}
                      </h3>
                      <p className="text-xs text-muted-foreground mt-1 leading-relaxed line-clamp-3" title={b.description}>
                        {b.description || "无描述"}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 转换器列表 */}
      {transformerList.length === 0 ? (
        <Card>
          <EmptyState
            icon={Code2}
            title="暂无预定义转换器"
            description="转换器可用于处理规则内容，如去重、排序、正则替换等"
            action={<Button onClick={handleCreate}><Plus className="w-4 h-4 mr-2" />创建转换器</Button>}
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {transformerList.map(([name, transformer], index) => (
            <div
              key={name}
              className="group relative overflow-hidden rounded-2xl border border-border bg-card shadow-[var(--shadow-sm)] animate-in fade-in slide-in-from-bottom-4"
              style={{ animationDelay: `${index * 50}ms`, animationFillMode: 'backwards' }}
            >
              <div className="px-5 pt-5 pb-3 z-10 relative">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-primary/12 bg-primary-soft text-primary shadow-[var(--shadow-xs)]">
                      <Code2 className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="text-[15px] font-semibold text-foreground">{name}</h3>
                      <p className="text-xs text-muted-foreground mt-1 line-clamp-1" title={transformer.description}>
                        {transformer.description || "无描述"}
                      </p>
                    </div>
                  </div>
                </div>
              </div>
              <div className="px-5 pb-5 z-10 relative">
                <div className="flex items-center justify-between border-t border-border/50 pt-3 mt-1">
                  <span className="text-xs text-muted-foreground font-mono">
                    {transformer.updatedAt
                      ? new Date(transformer.updatedAt).toLocaleDateString("zh-CN")
                      : "未更新"}
                  </span>
                  <div className="absolute right-4 bottom-4 flex gap-1 rounded-lg border border-border/50 bg-card/95 p-0.5 opacity-100 shadow-[var(--shadow-xs)] transition-opacity md:opacity-0 md:group-hover:opacity-100 md:focus-within:opacity-100">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => handleEdit(name)}
                      className="w-7 h-7 hover:bg-accent"
                      title="编辑"
                    >
                      <Edit3 className="w-3.5 h-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => setDeletingTransformer(name)}
                      className="w-7 h-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                      title="删除"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 编辑对话框 */}
      <Dialog
        open={!!editingTransformer}
        onOpenChange={(open) => !open && setEditingTransformer(null)}
      >
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Code2 className="w-5 h-5 text-primary" />
              {editingTransformer?.isNew ? "新建转换器" : "编辑转换器"}
            </DialogTitle>
            <DialogDescription>
              编辑转换器名称、描述与脚本内容
            </DialogDescription>
          </DialogHeader>

          {editingTransformer && (
            <div className="space-y-4">
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <Label>转换器名称 *</Label>
                  <Input
                    value={editingTransformer.data.name}
                    onChange={(e) =>
                      setEditingTransformer({
                        ...editingTransformer,
                        data: { ...editingTransformer.data, name: e.target.value },
                      })
                    }
                    placeholder="例如: clean_comments"
                  />
                </div>
                <div className="space-y-2">
                  <Label>描述</Label>
                  <Input
                    value={editingTransformer.data.description || ""}
                    onChange={(e) =>
                      setEditingTransformer({
                        ...editingTransformer,
                        data: { ...editingTransformer.data, description: e.target.value },
                      })
                    }
                    placeholder="转换器功能描述"
                  />
                </div>
              </div>

              <div className="space-y-2">
                <Label className="flex items-center gap-2">
                  脚本内容 *
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger>
                        <HelpCircle className="w-4 h-4 text-muted-foreground" />
                      </TooltipTrigger>
                      <TooltipContent side="right" className="max-w-sm">
                        <p>编写 transform(content) 函数处理规则内容</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </Label>
                <Textarea
                  value={editingTransformer.data.script}
                  onChange={(e) =>
                    setEditingTransformer({
                      ...editingTransformer,
                      data: { ...editingTransformer.data, script: e.target.value },
                    })
                  }
                  placeholder="function transform(content) { ... }"
                  className="font-mono text-sm h-64"
                />
              </div>

              {/* 测试区域 */}
              <div className="space-y-4 rounded-2xl border border-border bg-surface-subtle/35 p-4 shadow-[var(--shadow-xs)]">
                <div className="flex items-center justify-between">
                  <Label className="flex items-center gap-2">
                    <Play className="w-4 h-4" />
                    测试脚本
                  </Label>
                  <Button variant="outline" size="sm" onClick={handleTest}>
                    运行测试
                  </Button>
                </div>
                <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                  <div className="space-y-2">
                    <Label className="text-sm text-muted-foreground">输入内容</Label>
                    <Textarea
                      value={testInput}
                      onChange={(e) => setTestInput(e.target.value)}
                      placeholder="输入测试内容..."
                      className="font-mono text-sm h-32"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-sm text-muted-foreground">输出结果</Label>
                    {testError ? (
                      <div className="h-32 overflow-auto rounded-xl border border-destructive/20 bg-destructive/6 p-3 text-sm font-mono text-destructive">
                        {testError}
                      </div>
                    ) : (
                      <Textarea
                        value={testOutput}
                        readOnly
                        placeholder="运行测试查看输出..."
                        className="h-32 bg-surface-subtle/60 font-mono text-sm"
                      />
                    )}
                  </div>
                </div>
              </div>

              <div className="flex justify-end gap-3">
                <Button variant="outline" onClick={() => setEditingTransformer(null)}>
                  取消
                </Button>
                <Button onClick={handleSave} disabled={isSaving}>
                  {isSaving ? (
                    <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                  ) : null}
                  保存
                </Button>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* 删除确认对话框 */}
      <Dialog open={!!deletingTransformer} onOpenChange={(open) => !open && setDeletingTransformer(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle className="w-5 h-5 text-destructive" />
              确认删除
            </DialogTitle>
            <DialogDescription>
              确定要删除转换器 <strong>{deletingTransformer}</strong> 吗？
              <br />
              <span className="text-destructive">此操作无法恢复，使用此转换器的规则将无法正常工作。</span>
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-3 mt-4">
            <Button variant="outline" onClick={() => setDeletingTransformer(null)} disabled={isDeleting}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={isDeleting}
              onClick={() => deletingTransformer && handleDelete(deletingTransformer)}
            >
              {isDeleting ? <Loader2 className="w-4 h-4 mr-1 animate-spin" /> : <Trash2 className="w-4 h-4 mr-1" />}
              删除
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* 帮助文档对话框 */}
      <Dialog open={showHelp} onOpenChange={setShowHelp}>
        <DialogContent className="max-w-5xl w-[95vw] max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <BookOpen className="w-5 h-5 text-primary" />
              预定义转换器帮助文档
            </DialogTitle>
            <DialogDescription>
              了解脚本转换器的使用方式与示例
            </DialogDescription>
          </DialogHeader>
          <div className="prose dark:prose-invert max-w-none text-sm space-y-4">
            <SimpleMarkdown content={HELP_DOC} />
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

// 简单的 Markdown 渲染组件（无需外部依赖）
function SimpleMarkdown({ content }: { content: string }) {
  const lines = content.split('\n');
  const elements: React.ReactNode[] = [];
  let i = 0;
  let key = 0;

  while (i < lines.length) {
    const line = lines[i];

    // 代码块
    if (line.startsWith('```')) {
      const lang = line.slice(3).trim();
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith('```')) {
        codeLines.push(lines[i]);
        i++;
      }
      i++; // 跳过结束的 ```
      elements.push(
        <pre key={key++} className="bg-muted p-3 rounded-lg overflow-x-auto text-xs">
          <code className={`language-${lang}`}>{codeLines.join('\n')}</code>
        </pre>
      );
      continue;
    }

    // 行内代码
    const processInlineCode = (text: string) => {
      const parts = text.split(/(`[^`]+`)/g);
      return parts.map((part, idx) => {
        if (part.startsWith('`') && part.endsWith('`')) {
          return (
            <code key={idx} className="bg-muted px-1 py-0.5 rounded text-xs">
              {part.slice(1, -1)}
            </code>
          );
        }
        return part;
      });
    };

    // 标题
    if (line.startsWith('####')) {
      elements.push(<h4 key={key++} className="text-sm font-semibold text-foreground/80 mt-4 mb-2">{processInlineCode(line.slice(4).trim())}</h4>);
    } else if (line.startsWith('###')) {
      elements.push(<h3 key={key++} className="text-base font-semibold text-foreground mt-4 mb-2">{processInlineCode(line.slice(3).trim())}</h3>);
    } else if (line.startsWith('##')) {
      elements.push(<h2 key={key++} className="text-lg font-bold text-foreground mt-6 mb-3">{processInlineCode(line.slice(2).trim())}</h2>);
    } else if (line.trim() === '') {
      // 空行跳过
    } else {
      // 普通段落
      elements.push(<p key={key++} className="text-foreground/80 leading-relaxed">{processInlineCode(line)}</p>);
    }
    i++;
  }

  return <>{elements}</>;
}
