package syncengine

import (
	"time"

	"github.com/robfig/cron/v3"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// DefaultCronExpression mirrors the TS constant.
const DefaultCronExpression = "0 0 * * *"

// cronParser accepts both 5- and 6-field expressions plus @hourly/@daily etc.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// ComputeNextSyncAt returns an ISO-8601 string for the next scheduled sync.
func ComputeNextSyncAt(schedule schema.SyncSchedule, base *string) string {
	now := time.Now().UTC()
	if base != nil {
		if t, err := util.ParseISO(*base); err == nil {
			now = t.UTC()
		}
	}
	switch schedule.Mode {
	case "cron":
		expr := schedule.CronExpression
		if expr == "" {
			expr = DefaultCronExpression
		}
		sched, err := cronParser.Parse(expr)
		if err != nil {
			return ""
		}
		next := sched.Next(now)
		return util.FormatISO(next)
	default:
		hours := schedule.IntervalHours
		if hours < 1 {
			hours = 24
		}
		return util.FormatISO(now.Add(time.Duration(hours) * time.Hour))
	}
}

// ValidateCronExpression returns an error if the expression cannot be parsed.
func ValidateCronExpression(expr string) error {
	_, err := cronParser.Parse(expr)
	return err
}
