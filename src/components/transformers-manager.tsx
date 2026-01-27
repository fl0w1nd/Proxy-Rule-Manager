"use client";

import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
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
} from "lucide-react";
import { getConfig, saveConfig } from "@/lib/api-client";
import { RulesConfig, ScriptTransformer } from "@/lib/schema";
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
  const [isLoading, setIsLoading] = useState(true);
  const [editingTransformer, setEditingTransformer] = useState<{
    name: string;
    data: ScriptTransformer;
    isNew: boolean;
  } | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [deletingTransformer, setDeletingTransformer] = useState<string | null>(null);
  const [showHelp, setShowHelp] = useState(false);
  const [testInput, setTestInput] = useState("");
  const [testOutput, setTestOutput] = useState("");
  const [testError, setTestError] = useState("");

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async () => {
    try {
      const { config } = await getConfig();
      setConfig(config);
    } catch (error) {
      console.error("Failed to fetch config:", error);
      toast.error("获取配置失败");
    } finally {
      setIsLoading(false);
    }
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

    if (!data.script.trim()) {
      toast.error("脚本内容不能为空");
      return;
    }

    // 验证脚本语法
    try {
      new Function("content", data.script + "\nreturn transform(content);");
    } catch (e) {
      toast.error("脚本语法错误: " + String(e));
      return;
    }

    setIsSaving(true);
    try {
      const newTransformers = { ...config.transformers };

      // 如果名称变更，删除旧的
      if (!isNew && name !== data.name) {
        delete newTransformers[name];
      }

      newTransformers[data.name] = {
        ...data,
        updatedAt: new Date().toISOString(),
      };

      await saveConfig({
        ...config,
        transformers: newTransformers,
      });

      toast.success("转换器保存成功");
      setEditingTransformer(null);
      await fetchConfig();
      onRefresh?.();
    } catch (error) {
      toast.error("保存失败: " + String(error));
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async (name: string) => {
    if (!config) return;

    try {
      const newTransformers = { ...config.transformers };
      delete newTransformers[name];

      await saveConfig({
        ...config,
        transformers: newTransformers,
      });

      toast.success("转换器已删除");
      setDeletingTransformer(null);
      await fetchConfig();
      onRefresh?.();
    } catch (error) {
      toast.error("删除失败: " + String(error));
    }
  };

  const handleTest = () => {
    if (!editingTransformer) return;

    setTestError("");
    setTestOutput("");

    try {
      const fn = new Function("content", editingTransformer.data.script + "\nreturn transform(content);");
      const result = fn(testInput);
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
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 头部操作 */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
            <Code2 className="w-5 h-5 text-blue-500" />
            预定义转换器
          </h2>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
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

      {/* 转换器列表 */}
      {transformerList.length === 0 ? (
        <Card className="shadow-minimal border-none bg-card">
          <CardContent className="text-center py-16">
            <div className="w-20 h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-muted/50 to-muted flex items-center justify-center">
              <Code2 className="w-10 h-10 text-muted-foreground/40" />
            </div>
            <p className="text-lg font-medium text-foreground">暂无预定义转换器</p>
            <p className="text-sm text-muted-foreground mt-2 max-w-sm mx-auto">
              转换器可用于处理规则内容，如去重、排序、正则替换等
            </p>
            <Button onClick={handleCreate} className="mt-6">
              <Plus className="w-4 h-4 mr-2" />
              创建转换器
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {transformerList.map(([name, transformer], index) => (
            <Card
              key={name}
              className="shadow-minimal border-none bg-card hover-lift group relative overflow-hidden animate-in fade-in slide-in-from-bottom-4"
              style={{ animationDelay: `${index * 50}ms`, animationFillMode: 'backwards' }}
            >
              <CardHeader className="pb-3 z-10 relative">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-md bg-purple-500/10 flex items-center justify-center text-purple-600 dark:text-purple-400">
                      <Code2 className="w-5 h-5" />
                    </div>
                    <div>
                      <CardTitle className="text-base font-semibold">{name}</CardTitle>
                      <p className="text-xs text-muted-foreground mt-1 line-clamp-1" title={transformer.description}>
                        {transformer.description || "无描述"}
                      </p>
                    </div>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="z-10 relative">
                <div className="flex items-center justify-between border-t border-border/50 pt-3 mt-1">
                  <span className="text-xs text-muted-foreground font-mono">
                    {transformer.updatedAt
                      ? new Date(transformer.updatedAt).toLocaleDateString("zh-CN")
                      : "未更新"}
                  </span>
                  <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity absolute right-4 bottom-4 bg-card shadow-sm border rounded-md p-0.5">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => handleEdit(name)}
                      className="w-7 h-7 hover:bg-muted"
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
              </CardContent>
            </Card>
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
              <Code2 className="w-5 h-5 text-blue-500" />
              {editingTransformer?.isNew ? "新建转换器" : "编辑转换器"}
            </DialogTitle>
            <DialogDescription>
              编辑转换器名称、描述与脚本内容
            </DialogDescription>
          </DialogHeader>

          {editingTransformer && (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
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
                        <HelpCircle className="w-4 h-4 text-gray-400" />
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
              <div className="border border-gray-200 dark:border-slate-700 rounded-lg p-4 space-y-4">
                <div className="flex items-center justify-between">
                  <Label className="flex items-center gap-2">
                    <Play className="w-4 h-4" />
                    测试脚本
                  </Label>
                  <Button variant="outline" size="sm" onClick={handleTest}>
                    运行测试
                  </Button>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label className="text-sm text-gray-500">输入内容</Label>
                    <Textarea
                      value={testInput}
                      onChange={(e) => setTestInput(e.target.value)}
                      placeholder="输入测试内容..."
                      className="font-mono text-sm h-32"
                    />
                  </div>
                  <div className="space-y-2">
                    <Label className="text-sm text-gray-500">输出结果</Label>
                    {testError ? (
                      <div className="p-3 h-32 overflow-auto rounded bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-600 dark:text-red-400 text-sm font-mono">
                        {testError}
                      </div>
                    ) : (
                      <Textarea
                        value={testOutput}
                        readOnly
                        placeholder="运行测试查看输出..."
                        className="font-mono text-sm h-32 bg-gray-50 dark:bg-slate-800"
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
              <AlertTriangle className="w-5 h-5 text-red-500" />
              确认删除
            </DialogTitle>
            <DialogDescription>
              确定要删除转换器 <strong>{deletingTransformer}</strong> 吗？
              <br />
              <span className="text-red-500">此操作无法恢复，使用此转换器的规则将无法正常工作。</span>
            </DialogDescription>
          </DialogHeader>
          <div className="flex justify-end gap-3 mt-4">
            <Button variant="outline" onClick={() => setDeletingTransformer(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={() => deletingTransformer && handleDelete(deletingTransformer)}
            >
              <Trash2 className="w-4 h-4 mr-1" />
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
              <BookOpen className="w-5 h-5 text-blue-500" />
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
        <pre key={key++} className="bg-gray-100 dark:bg-slate-800 p-3 rounded-lg overflow-x-auto text-xs">
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
            <code key={idx} className="bg-gray-100 dark:bg-slate-800 px-1 py-0.5 rounded text-xs">
              {part.slice(1, -1)}
            </code>
          );
        }
        return part;
      });
    };

    // 标题
    if (line.startsWith('####')) {
      elements.push(<h4 key={key++} className="text-sm font-semibold text-gray-800 dark:text-gray-200 mt-4 mb-2">{processInlineCode(line.slice(4).trim())}</h4>);
    } else if (line.startsWith('###')) {
      elements.push(<h3 key={key++} className="text-base font-semibold text-gray-900 dark:text-white mt-4 mb-2">{processInlineCode(line.slice(3).trim())}</h3>);
    } else if (line.startsWith('##')) {
      elements.push(<h2 key={key++} className="text-lg font-bold text-gray-900 dark:text-white mt-6 mb-3">{processInlineCode(line.slice(2).trim())}</h2>);
    } else if (line.trim() === '') {
      // 空行跳过
    } else {
      // 普通段落
      elements.push(<p key={key++} className="text-gray-700 dark:text-gray-300 leading-relaxed">{processInlineCode(line)}</p>);
    }
    i++;
  }

  return <>{elements}</>;
}
