import { RuleConfig } from "../schema";

export function extractDependencies(rule: RuleConfig): Set<string> {
  const deps = new Set<string>();

  if (rule.compose_from) {
    for (const ref of rule.compose_from) {
      deps.add(ref);
    }
  }

  if (rule.sources) {
    for (const source of rule.sources) {
      if (source.type === "ref" && source.ref) {
        deps.add(source.ref);
      }
    }
  }

  return deps;
}

export function detectCircularDependency(rules: RuleConfig[]): string[] | null {
  const ruleMap = new Map<string, RuleConfig>();
  const dependencies = new Map<string, Set<string>>();

  for (const rule of rules) {
    ruleMap.set(rule.name, rule);
    dependencies.set(rule.name, extractDependencies(rule));
  }

  const visited = new Set<string>();
  const inStack = new Set<string>();
  const path: string[] = [];

  function dfs(node: string): string[] | null {
    if (inStack.has(node)) {
      const cycleStart = path.indexOf(node);
      return [...path.slice(cycleStart), node];
    }
    if (visited.has(node)) {
      return null;
    }

    visited.add(node);
    inStack.add(node);
    path.push(node);

    const deps = dependencies.get(node);
    if (deps) {
      for (const dep of deps) {
        if (ruleMap.has(dep)) {
          const cycle = dfs(dep);
          if (cycle) {
            return cycle;
          }
        }
      }
    }

    path.pop();
    inStack.delete(node);
    return null;
  }

  for (const rule of rules) {
    visited.clear();
    inStack.clear();
    path.length = 0;

    const cycle = dfs(rule.name);
    if (cycle) {
      return cycle;
    }
  }

  return null;
}

export function topologicalSort(
  rules: RuleConfig[],
  skipMissingDepsCheck: boolean = false
): RuleConfig[] {
  const cycle = detectCircularDependency(rules);
  if (cycle) {
    const cycleStr = cycle.join(" → ");
    throw new Error(`检测到循环依赖: ${cycleStr}`);
  }

  const ruleMap = new Map<string, RuleConfig>();
  const dependencies = new Map<string, Set<string>>();

  for (const rule of rules) {
    ruleMap.set(rule.name, rule);
  }

  for (const rule of rules) {
    const allDeps = extractDependencies(rule);
    const inSetDeps = new Set<string>();
    for (const dep of allDeps) {
      if (ruleMap.has(dep)) {
        inSetDeps.add(dep);
      }
    }
    dependencies.set(rule.name, inSetDeps);
  }

  if (!skipMissingDepsCheck) {
    const missingDeps: string[] = [];
    for (const [ruleName, deps] of dependencies) {
      for (const dep of deps) {
        if (!ruleMap.has(dep)) {
          missingDeps.push(`规则 "${ruleName}" 引用了不存在的规则 "${dep}"`);
        }
      }
    }
    if (missingDeps.length > 0) {
      throw new Error(`依赖缺失:\n${missingDeps.join("\n")}`);
    }
  }

  const sorted: RuleConfig[] = [];
  const noDepRules: string[] = [];

  for (const [name, deps] of dependencies) {
    if (deps.size === 0) {
      noDepRules.push(name);
    }
  }

  while (noDepRules.length > 0) {
    const name = noDepRules.shift()!;
    const rule = ruleMap.get(name);
    if (rule) {
      sorted.push(rule);
    }

    for (const [otherName, deps] of dependencies) {
      if (deps.has(name)) {
        deps.delete(name);
        if (deps.size === 0) {
          noDepRules.push(otherName);
        }
      }
    }
  }

  return sorted;
}
