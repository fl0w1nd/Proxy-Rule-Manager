import { RuleConfig, ClientType, TransformersConfig, ClientConfig } from "../schema";
import { applyNewTransforms, mergeContents } from "../transformer";
import { fetchSource } from "./fetcher";
import { readLocalSourceContent } from "../local-source-store";

interface RuleProcessResult {
  ruleName: string;
  contents: Map<ClientType, string>;
  errors: string[];
}

export async function processRule(
  rule: RuleConfig,
  transformersConfig: TransformersConfig,
  ruleContentsCache: Map<string, Map<ClientType, string>>,
  clientsConfig?: ClientConfig[]
): Promise<RuleProcessResult> {
  const result: RuleProcessResult = {
    ruleName: rule.name,
    contents: new Map(),
    errors: [],
  };

  const staticContents = new Map<number, string>();

  if (rule.sources) {
    for (let i = 0; i < rule.sources.length; i++) {
      const source = rule.sources[i];
      const sourceType = source.type || "url";

      if (sourceType === "url" && source.url) {
        const { content, error } = await fetchSource(source.url);
        if (error) {
          result.errors.push(`Source ${source.url}: ${error}`);
        } else {
          staticContents.set(i, content);
        }
      } else if (sourceType === "local") {
        if (source.content) {
          staticContents.set(i, source.content);
        } else if (source.contentRef) {
          const content = await readLocalSourceContent(source.contentRef);
          if (content !== null) {
            staticContents.set(i, content);
          }
        }
      }
    }
  }

  for (const client of rule.output.clients) {
    let baseContent = "";

    if (rule.sources && rule.sources.length > 0) {
      const sourceContents: string[] = [];

      for (let i = 0; i < rule.sources.length; i++) {
        const source = rule.sources[i];
        const sourceType = source.type || "url";

        if (sourceType === "ref" && source.ref) {
          const refContents = ruleContentsCache.get(source.ref);
          if (refContents) {
            const refContent = refContents.get(client) || refContents.values().next().value;
            if (refContent) {
              sourceContents.push(refContent);
            } else {
              result.errors.push(`Ref "${source.ref}" has no content for client ${client}`);
            }
          } else {
            result.errors.push(`Ref rule "${source.ref}" not found in cache`);
          }
        } else if (staticContents.has(i)) {
          sourceContents.push(staticContents.get(i)!);
        }
      }

      if (sourceContents.length === 0) {
        result.errors.push(`No sources fetched successfully for client ${client}`);
        continue;
      }

      let processedContents = sourceContents;
      if (rule.transforms && rule.transforms.length > 0) {
        processedContents = applyNewTransforms(sourceContents, rule.transforms, transformersConfig);
      }

      const strategy = rule.merge?.strategy || "concat";
      const dedupe = rule.merge?.dedupe || false;
      baseContent = mergeContents(processedContents, strategy, dedupe);
    } else {
      result.errors.push("Rule has no sources");
      return result;
    }

    // 应用客户端差异化配置的转换器
    const clientOverride = rule.output.client_overrides?.[client];
    if (clientOverride?.enabled !== false) {
      // 检查是否使用客户端全局转换器（默认为 true）
      const useGlobalTransforms = clientOverride?.useGlobalTransforms ?? true;
      
      // 获取客户端全局转换器
      const clientConfig = clientsConfig?.find(c => c.id === client);
      const globalTransforms = clientConfig?.transforms || [];
      
      // 先应用客户端全局转换器
      if (useGlobalTransforms && globalTransforms.length > 0) {
        const transformedContents = applyNewTransforms(
          [baseContent],
          globalTransforms,
          transformersConfig
        );
        baseContent = transformedContents[0] || baseContent;
      }
      
      // 再应用规则级别的额外转换器
      if (clientOverride?.transforms && clientOverride.transforms.length > 0) {
        const transformedContents = applyNewTransforms(
          [baseContent],
          clientOverride.transforms,
          transformersConfig
        );
        baseContent = transformedContents[0] || baseContent;
      }
    }

    result.contents.set(client, baseContent);
  }

  return result;
}
