import { RuleConfig, ClientType, TransformersConfig } from "../schema";
import {
  applyTransforms,
  applyNewTransforms,
  mergeContents,
  addRuleHeader,
} from "../transformer";
import { fetchSource } from "./fetcher";

interface RuleProcessResult {
  ruleName: string;
  contents: Map<ClientType, string>;
  errors: string[];
}

export async function processRule(
  rule: RuleConfig,
  transformersConfig: TransformersConfig,
  ruleContentsCache: Map<string, Map<ClientType, string>>
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
      } else if (sourceType === "local" && source.content) {
        staticContents.set(i, source.content);
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
    } else if (rule.compose_from && rule.compose_from.length > 0) {
      const composeContents: string[] = [];

      for (const depRuleName of rule.compose_from) {
        const depContents = ruleContentsCache.get(depRuleName);
        if (depContents) {
          const depContent = depContents.get(client) || depContents.values().next().value;
          if (depContent) {
            composeContents.push(depContent);
          }
        } else {
          result.errors.push(`Dependency rule "${depRuleName}" not found in cache`);
        }
      }

      if (composeContents.length === 0) {
        result.errors.push(`No compose sources available for client ${client}`);
        continue;
      }

      let processedContents = composeContents;
      if (rule.transforms && rule.transforms.length > 0) {
        processedContents = applyNewTransforms(
          composeContents,
          rule.transforms,
          transformersConfig
        );
      }

      const composeStrategy = rule.merge?.strategy || "concat";
      const composeDedupe = rule.merge?.dedupe ?? true;
      baseContent = mergeContents(processedContents, composeStrategy, composeDedupe);
    } else {
      result.errors.push("Rule has no sources or compose_from");
      return result;
    }

    if (rule.post_transforms && rule.post_transforms.length > 0) {
      baseContent = applyTransforms(baseContent, rule.post_transforms, transformersConfig);
    }

    const clientOverride = rule.output.client_overrides?.[client];
    if (
      clientOverride?.enabled !== false &&
      clientOverride?.transforms &&
      clientOverride.transforms.length > 0
    ) {
      const transformedContents = applyNewTransforms(
        [baseContent],
        clientOverride.transforms,
        transformersConfig
      );
      baseContent = transformedContents[0] || baseContent;
    }

    const finalContent = addRuleHeader(baseContent, rule.name, rule.description);
    result.contents.set(client, finalContent);
  }

  return result;
}
