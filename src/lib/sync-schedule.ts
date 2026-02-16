import { CronExpressionParser } from "cron-parser";
import { DEFAULT_SYNC_SCHEDULE, SyncSchedule } from "./schema";

export const DEFAULT_CRON_EXPRESSION = "0 0 * * *";

export function normalizeSyncSchedule(schedule?: Partial<SyncSchedule> | null): SyncSchedule {
  const mode = schedule?.mode === "cron" ? "cron" : "interval";
  const intervalHours =
    typeof schedule?.intervalHours === "number" && schedule.intervalHours >= 1
      ? schedule.intervalHours
      : DEFAULT_SYNC_SCHEDULE.intervalHours;
  const cronExpression =
    schedule?.cronExpression?.trim() ||
    DEFAULT_SYNC_SCHEDULE.cronExpression ||
    DEFAULT_CRON_EXPRESSION;

  return {
    mode,
    intervalHours,
    cronExpression,
    lastScheduledSyncAt: schedule?.lastScheduledSyncAt,
    nextSyncAt: schedule?.nextSyncAt,
  };
}

export function getNextSyncAt(schedule: SyncSchedule, baseDate: Date): string {
  if (schedule.mode === "cron") {
    const expression = CronExpressionParser.parse(
      schedule.cronExpression || DEFAULT_CRON_EXPRESSION,
      {
        currentDate: baseDate,
        tz: "UTC",
      }
    );
    return expression.next().toDate().toISOString();
  }

  return new Date(baseDate.getTime() + schedule.intervalHours * 60 * 60 * 1000).toISOString();
}

export function validateCronExpression(expression: string): void {
  CronExpressionParser.parse(expression, { currentDate: new Date(), tz: "UTC" });
}
