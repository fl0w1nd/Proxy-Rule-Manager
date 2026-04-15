import * as fs from "node:fs/promises";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

interface TestDb {
  config: { version: number; transformers: Record<string, never>; rules: [] };
  configRev: number;
  clients: Array<{ id: string; displayName: string; pathName?: string }>;
  clientFiles: Record<string, {
    id: string;
    clientId: string;
    configId: string;
    displayName: string;
    ext: string;
    isPublic: boolean;
    createdAt: string;
    updatedAt: string;
  }>;
  artifacts: Record<string, { client: string; blobPath: string }>;
  jobs: Record<string, never>;
  dailyStats: Record<string, never>;
  locks: Record<string, never>;
  lastSyncInfo: {
    lastFullSyncAt: null;
    lastPartialSyncAt: null;
    lastSuccessfulSyncAt: null;
    totalRulesCount: number;
    changedRulesCount: number;
    failedRulesCount: number;
  };
  syncSchedule: {
    mode: "interval";
    intervalHours: number;
    cronExpression: string;
  };
  cdnSettings: {
    enabled: boolean;
    cacheMode: "no-cache";
    staleIfErrorSeconds: number;
    customHeaders: [];
  };
}

async function createTempDataDir() {
  return await fs.mkdtemp(path.join(os.tmpdir(), "prm-storage-"));
}

function createTestDb(): TestDb {
  return {
    config: { version: 1, transformers: {}, rules: [] },
    configRev: 1,
    clients: [
      { id: "clash_meta", displayName: "Clash Meta / Stash", pathName: "Clash Meta" },
      { id: "shadowrocket", displayName: "Shadowrocket", pathName: "Shadowrocket" },
    ],
    clientFiles: {},
    artifacts: {},
    jobs: {},
    dailyStats: {},
    locks: {},
    lastSyncInfo: {
      lastFullSyncAt: null,
      lastPartialSyncAt: null,
      lastSuccessfulSyncAt: null,
      totalRulesCount: 0,
      changedRulesCount: 0,
      failedRulesCount: 0,
    },
    syncSchedule: {
      mode: "interval",
      intervalHours: 24,
      cronExpression: "0 0 * * *",
    },
    cdnSettings: {
      enabled: false,
      cacheMode: "no-cache",
      staleIfErrorSeconds: 604800,
      customHeaders: [],
    },
  };
}

describe("storage-adapter client file paths", () => {
  let dataDir: string;

  beforeEach(async () => {
    vi.resetModules();
    dataDir = await createTempDataDir();
    process.env.DATA_DIR = dataDir;
  });

  afterEach(async () => {
    delete process.env.DATA_DIR;
    await fs.rm(dataDir, { recursive: true, force: true });
  });

  it("migrates legacy flat client files into clientId directories", async () => {
    const db = createTestDb();
    const fileId = "file-1";
    db.clientFiles[fileId] = {
      id: fileId,
      clientId: "clash_meta",
      configId: "proxy",
      displayName: "Proxy",
      ext: "yaml",
      isPublic: true,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    const clientDir = path.join(dataDir, "client");
    await fs.mkdir(clientDir, { recursive: true });
    await fs.writeFile(path.join(clientDir, "proxy.yaml"), "mixed-port: 7890", "utf-8");
    await fs.writeFile(path.join(dataDir, "db.json"), JSON.stringify(db, null, 2), "utf-8");

    const storage = await import("@/lib/storage-adapter");

    expect(await storage.getClientFileContent(fileId)).toBe("mixed-port: 7890");
    await expect(fs.access(path.join(clientDir, "proxy.yaml"))).rejects.toThrow();
    await expect(fs.readFile(path.join(clientDir, "clash_meta", "proxy.yaml"), "utf-8")).resolves.toBe("mixed-port: 7890");
  });

  it("allows the same configId under different clientIds", async () => {
    const storage = await import("@/lib/storage-adapter");

    await storage.createClientFile("clash_meta", {
      configId: "default",
      displayName: "Default Clash",
      ext: "yaml",
      isPublic: true,
      content: "mixed-port: 7890",
    });

    await storage.createClientFile("shadowrocket", {
      configId: "default",
      displayName: "Default Shadowrocket",
      ext: "yaml",
      isPublic: true,
      content: "[General]",
    });

    const clashFiles = await storage.listClientFiles("clash_meta");
    const shadowrocketFiles = await storage.listClientFiles("shadowrocket");

    expect(clashFiles).toHaveLength(1);
    expect(shadowrocketFiles).toHaveLength(1);
    await expect(fs.readFile(path.join(dataDir, "client", "clash_meta", "default.yaml"), "utf-8")).resolves.toBe("mixed-port: 7890");
    await expect(fs.readFile(path.join(dataDir, "client", "shadowrocket", "default.yaml"), "utf-8")).resolves.toBe("[General]");
  });

  it("migrates legacy rule directories to clientId paths", async () => {
    const db = createTestDb();
    db.artifacts = {
      "YouTube:clash_meta": {
        client: "clash_meta",
        blobPath: "/Rules/Clash Meta/YouTube.list",
      },
    };

    const legacyRuleDir = path.join(dataDir, "rules", "Clash Meta");
    await fs.mkdir(legacyRuleDir, { recursive: true });
    await fs.writeFile(path.join(legacyRuleDir, "YouTube.list"), "DOMAIN,youtube.com", "utf-8");
    await fs.writeFile(path.join(dataDir, "db.json"), JSON.stringify(db, null, 2), "utf-8");

    const storage = await import("@/lib/storage-adapter");

    await expect(storage.getRuleContent("YouTube", "clash_meta")).resolves.toBe("DOMAIN,youtube.com");
    expect(storage.getRulePublicPath("YouTube", "clash_meta")).toBe("/Rules/clash_meta/YouTube.list");
    await expect(fs.readFile(path.join(dataDir, "Rules", "clash_meta", "YouTube.list"), "utf-8")).resolves.toBe("DOMAIN,youtube.com");
    await expect(fs.access(path.join(dataDir, "rules", "Clash Meta", "YouTube.list"))).rejects.toThrow();
  });

  it("renames legacy rules root directory to Rules", async () => {
    const db = createTestDb();
    const legacyRuleDir = path.join(dataDir, "rules", "clash_meta");
    await fs.mkdir(legacyRuleDir, { recursive: true });
    await fs.writeFile(path.join(legacyRuleDir, "Netflix.list"), "DOMAIN,netflix.com", "utf-8");
    await fs.writeFile(path.join(dataDir, "db.json"), JSON.stringify(db, null, 2), "utf-8");

    const storage = await import("@/lib/storage-adapter");

    await expect(storage.getRuleContent("Netflix", "clash_meta")).resolves.toBe("DOMAIN,netflix.com");
    expect(storage.getRulesDir()).toBe(path.join(dataDir, "Rules"));
    await expect(fs.readFile(path.join(dataDir, "Rules", "clash_meta", "Netflix.list"), "utf-8")).resolves.toBe("DOMAIN,netflix.com");
    await expect(fs.readdir(dataDir)).resolves.toContain("Rules");
  });
});
