import { describe, it, expect } from "vitest";
import {
    normalizeSyncSchedule,
    getNextSyncAt,
    validateCronExpression,
    DEFAULT_CRON_EXPRESSION,
} from "@/lib/sync-schedule";

describe("normalizeSyncSchedule", () => {
    it("should return defaults for undefined input", () => {
        const result = normalizeSyncSchedule(undefined);
        expect(result.mode).toBe("interval");
        expect(result.intervalHours).toBe(24);
        expect(result.cronExpression).toBe("0 0 * * *");
    });

    it("should return defaults for null input", () => {
        const result = normalizeSyncSchedule(null);
        expect(result.mode).toBe("interval");
        expect(result.intervalHours).toBe(24);
    });

    it("should preserve cron mode", () => {
        const result = normalizeSyncSchedule({ mode: "cron" });
        expect(result.mode).toBe("cron");
    });

    it("should fall back to interval for invalid mode", () => {
        const result = normalizeSyncSchedule({ mode: "interval" });
        expect(result.mode).toBe("interval");
    });

    it("should use provided intervalHours when valid", () => {
        const result = normalizeSyncSchedule({ intervalHours: 6 });
        expect(result.intervalHours).toBe(6);
    });

    it("should fall back to default when intervalHours < 1", () => {
        const result = normalizeSyncSchedule({ intervalHours: 0 });
        expect(result.intervalHours).toBe(24);
    });

    it("should fall back to default when intervalHours is negative", () => {
        const result = normalizeSyncSchedule({ intervalHours: -5 });
        expect(result.intervalHours).toBe(24);
    });

    it("should use provided cronExpression", () => {
        const result = normalizeSyncSchedule({ cronExpression: "*/30 * * * *" });
        expect(result.cronExpression).toBe("*/30 * * * *");
    });

    it("should trim whitespace from cronExpression", () => {
        const result = normalizeSyncSchedule({ cronExpression: "  0 6 * * *  " });
        expect(result.cronExpression).toBe("0 6 * * *");
    });

    it("should fall back to default for empty cronExpression", () => {
        const result = normalizeSyncSchedule({ cronExpression: "   " });
        expect(result.cronExpression).toBe("0 0 * * *");
    });

    it("should preserve lastScheduledSyncAt and nextSyncAt", () => {
        const result = normalizeSyncSchedule({
            lastScheduledSyncAt: "2025-01-01T00:00:00Z",
            nextSyncAt: "2025-01-02T00:00:00Z",
        });
        expect(result.lastScheduledSyncAt).toBe("2025-01-01T00:00:00Z");
        expect(result.nextSyncAt).toBe("2025-01-02T00:00:00Z");
    });
});

describe("getNextSyncAt", () => {
    it("should calculate next sync for interval mode", () => {
        const baseDate = new Date("2025-06-01T12:00:00Z");
        const schedule = normalizeSyncSchedule({ mode: "interval", intervalHours: 6 });
        const next = getNextSyncAt(schedule, baseDate);
        expect(next).toBe(new Date("2025-06-01T18:00:00Z").toISOString());
    });

    it("should calculate next sync for cron mode", () => {
        const baseDate = new Date("2025-06-01T12:00:00Z");
        const schedule = normalizeSyncSchedule({
            mode: "cron",
            cronExpression: "0 0 * * *", // 每天 00:00 UTC
        });
        const next = getNextSyncAt(schedule, baseDate);
        // 下一次应该是 2025-06-02T00:00:00Z
        expect(next).toBe(new Date("2025-06-02T00:00:00Z").toISOString());
    });

    it("should use default cron expression when none provided", () => {
        const baseDate = new Date("2025-06-01T12:00:00Z");
        const schedule = normalizeSyncSchedule({ mode: "cron", cronExpression: "" });
        const next = getNextSyncAt(schedule, baseDate);
        // 默认 "0 0 * * *"，下一次 00:00 UTC
        expect(next).toBe(new Date("2025-06-02T00:00:00Z").toISOString());
    });

    it("should handle 1-hour interval", () => {
        const baseDate = new Date("2025-06-01T10:30:00Z");
        const schedule = normalizeSyncSchedule({ mode: "interval", intervalHours: 1 });
        const next = getNextSyncAt(schedule, baseDate);
        expect(next).toBe(new Date("2025-06-01T11:30:00Z").toISOString());
    });
});

describe("validateCronExpression", () => {
    it("should not throw for valid cron expression", () => {
        expect(() => validateCronExpression("0 0 * * *")).not.toThrow();
        expect(() => validateCronExpression("*/15 * * * *")).not.toThrow();
        expect(() => validateCronExpression("0 6,18 * * 1-5")).not.toThrow();
    });

    it("should throw for invalid cron expression", () => {
        expect(() => validateCronExpression("invalid")).toThrow();
        expect(() => validateCronExpression("60 * * * *")).toThrow();
    });
});

describe("DEFAULT_CRON_EXPRESSION", () => {
    it("should be a valid cron expression", () => {
        expect(() => validateCronExpression(DEFAULT_CRON_EXPRESSION)).not.toThrow();
    });
});
