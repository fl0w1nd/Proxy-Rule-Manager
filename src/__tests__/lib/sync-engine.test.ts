import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/storage-adapter", () => ({
  getConfig: vi.fn(),
  getClients: vi.fn(),
  getArtifactMeta: vi.fn(),
  saveArtifactMeta: vi.fn(),
  acquireRuleLock: vi.fn(),
  releaseRuleLock: vi.fn(),
  acquireGlobalSyncLock: vi.fn(),
  releaseGlobalSyncLock: vi.fn(),
  createJob: vi.fn(),
  completeJob: vi.fn(),
  incrementDailyStats: vi.fn(),
  updateLastSyncInfo: vi.fn(),
  uploadRuleContent: vi.fn(),
  getRuleContent: vi.fn(),
}));

vi.mock("@/lib/activity-store", () => ({
  recordRuleFileChanges: vi.fn(),
  recordFailureRecords: vi.fn(),
}));

vi.mock("@/lib/sync-engine/processor", () => ({
  processRule: vi.fn(),
}));

vi.mock("@/lib/sync-engine/dependency-graph", () => ({
  extractDependencies: vi.fn(() => []),
  topologicalSort: vi.fn((rules) => rules),
}));

import { executeFullSync } from "@/lib/sync-engine";
import {
  acquireGlobalSyncLock,
  createJob,
  getArtifactMeta,
  getClients,
  getConfig,
  getRuleContent,
  releaseGlobalSyncLock,
  saveArtifactMeta,
  updateLastSyncInfo,
  uploadRuleContent,
} from "@/lib/storage-adapter";
import { recordRuleFileChanges, recordFailureRecords } from "@/lib/activity-store";
import { processRule } from "@/lib/sync-engine/processor";

const mockedAcquireGlobalSyncLock = vi.mocked(acquireGlobalSyncLock);
const mockedCreateJob = vi.mocked(createJob);
const mockedGetArtifactMeta = vi.mocked(getArtifactMeta);
const mockedGetClients = vi.mocked(getClients);
const mockedGetConfig = vi.mocked(getConfig);
const mockedGetRuleContent = vi.mocked(getRuleContent);
const mockedProcessRule = vi.mocked(processRule);
const mockedReleaseGlobalSyncLock = vi.mocked(releaseGlobalSyncLock);
const mockedRecordFailureRecords = vi.mocked(recordFailureRecords);
const mockedRecordRuleFileChanges = vi.mocked(recordRuleFileChanges);
const mockedSaveArtifactMeta = vi.mocked(saveArtifactMeta);
const mockedUpdateLastSyncInfo = vi.mocked(updateLastSyncInfo);
const mockedUploadRuleContent = vi.mocked(uploadRuleContent);

describe("executeFullSync", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    mockedAcquireGlobalSyncLock.mockResolvedValue({ acquired: true });
    mockedCreateJob.mockResolvedValue({ jobId: "job-1" } as never);
    mockedGetClients.mockResolvedValue([{ id: "clash_meta", displayName: "Clash Meta" }] as never);
    mockedGetConfig.mockResolvedValue({
      rules: [
        {
          name: "test-rule",
          sources: [{ type: "local", content: "# upstream header\nDOMAIN,test.com\n" }],
          tags: [],
          output: { clients: ["clash_meta"] },
        },
      ],
      transformers: {},
    } as never);
    mockedProcessRule.mockResolvedValue({
      ruleName: "test-rule",
      contents: new Map([["clash_meta", "# upstream header\nDOMAIN,test.com\n"]]),
      errors: [],
    });
    mockedUploadRuleContent.mockResolvedValue({
      url: "/Rules/clash_meta/test-rule.list",
      path: "/Rules/clash_meta/test-rule.list",
    });
    mockedReleaseGlobalSyncLock.mockResolvedValue(undefined);
    mockedRecordRuleFileChanges.mockResolvedValue(undefined);
    mockedRecordFailureRecords.mockResolvedValue(undefined);
    mockedUpdateLastSyncInfo.mockResolvedValue(undefined);
    mockedSaveArtifactMeta.mockResolvedValue(undefined);
  });

  it("writes latest upstream content when only comments changed", async () => {
    mockedGetArtifactMeta.mockResolvedValue({
      ruleName: "test-rule",
      client: "clash_meta",
      lastHash: "old-hash",
      lastUpdatedAt: "2026-04-14T00:00:00.000Z",
      blobPath: "/Rules/clash_meta/test-rule.list",
      blobUrl: "/Rules/clash_meta/test-rule.list",
      sizeBytes: 16,
    });
    mockedGetRuleContent.mockResolvedValue("# old comment\nDOMAIN,test.com\n");

    await executeFullSync();

    expect(mockedUploadRuleContent).toHaveBeenCalledWith(
      "test-rule",
      "clash_meta",
      "# upstream header\nDOMAIN,test.com\n"
    );
    expect(mockedSaveArtifactMeta).toHaveBeenCalledTimes(1);
    expect(mockedRecordRuleFileChanges).toHaveBeenCalledWith([]);
  });
});
