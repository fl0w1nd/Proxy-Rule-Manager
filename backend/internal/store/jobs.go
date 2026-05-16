package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/schema"
	"github.com/fl0w1nd/proxy-rule-manager/backend/internal/util"
)

// newJobID returns an RFC 4122 UUID v4 string, matching crypto.randomUUID() in the TS original.
func newJobID() string {
	return uuid.NewString()
}

// CreateJob inserts a new running job and returns it.
func (s *Store) CreateJob(ctx context.Context, jobType string, affectedRules []string) (schema.JobRecord, error) {
	if affectedRules == nil {
		affectedRules = []string{}
	}
	jobID := newJobID()
	now := util.NowISO()
	affectedJSON, _ := json.Marshal(affectedRules)
	if err := s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO jobs (job_id, type, status, started_at, affected_rules_json, logs_json)
			 VALUES (?, ?, 'running', ?, ?, '[]')`,
			jobID, jobType, now, string(affectedJSON),
		)
		return err
	}); err != nil {
		return schema.JobRecord{}, err
	}
	return schema.JobRecord{
		JobID:         jobID,
		Type:          jobType,
		Status:        "running",
		StartedAt:     now,
		AffectedRules: affectedRules,
		Logs:          []string{},
	}, nil
}

// CompleteJob marks a job as finished with the given results.
func (s *Store) CompleteJob(ctx context.Context, jobID string, changedRules []string, failedRules []schema.JobFailedRule) error {
	status := "completed"
	if len(failedRules) > 0 {
		status = "failed"
	}
	now := util.NowISO()
	changedJSON, _ := json.Marshal(changedRules)
	failedJSON, _ := json.Marshal(failedRules)
	return s.withWriteLock(func() error {
		_, err := s.DB.ExecContext(ctx,
			`UPDATE jobs SET status = ?, completed_at = ?, changed_rules_json = ?, failed_rules_json = ? WHERE job_id = ?`,
			status, now, string(changedJSON), string(failedJSON), jobID,
		)
		return err
	})
}

// GetJob loads a job by id.
func (s *Store) GetJob(ctx context.Context, jobID string) (*schema.JobRecord, error) {
	row := s.DB.QueryRowContext(ctx,
		`SELECT job_id, type, status, started_at, completed_at, affected_rules_json, changed_rules_json, failed_rules_json, logs_json
		 FROM jobs WHERE job_id = ?`, jobID)
	var j schema.JobRecord
	var completed sql.NullString
	var affected, changed, failed, logs sql.NullString
	if err := row.Scan(&j.JobID, &j.Type, &j.Status, &j.StartedAt, &completed, &affected, &changed, &failed, &logs); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if completed.Valid {
		v := completed.String
		j.CompletedAt = &v
	}
	if affected.Valid && affected.String != "" {
		_ = json.Unmarshal([]byte(affected.String), &j.AffectedRules)
	}
	if changed.Valid && changed.String != "" {
		_ = json.Unmarshal([]byte(changed.String), &j.ChangedRules)
	}
	if failed.Valid && failed.String != "" {
		_ = json.Unmarshal([]byte(failed.String), &j.FailedRules)
	}
	if logs.Valid && logs.String != "" {
		_ = json.Unmarshal([]byte(logs.String), &j.Logs)
	}
	// Normalize nil slices so the API always serialises empty arrays as [] not null.
	if j.AffectedRules == nil {
		j.AffectedRules = []string{}
	}
	if j.ChangedRules == nil {
		j.ChangedRules = []string{}
	}
	if j.FailedRules == nil {
		j.FailedRules = []schema.JobFailedRule{}
	}
	if j.Logs == nil {
		j.Logs = []string{}
	}
	return &j, nil
}
