package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fl0w1nd/proxy-rule-manager/internal/updates"
)

func TestCreateScopedUpdates(t *testing.T) {
	for _, scope := range []string{"rules", "geosite", "geoip"} {
		t.Run(scope, func(t *testing.T) {
			s, _, st := testServer(t)
			body := `{"scope":"` + scope + `"}`
			if scope == "rules" {
				body = `{"scope":"rules","rule_ids":["child","apple","apple"]}`
			}
			req := authorized(http.MethodPost, "/api/v1/updates", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var summary updateSummary
			if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
				t.Fatal(err)
			}
			job := s.updates.Job(summary.ID)
			if job == nil {
				t.Fatal("missing update job")
			}
			select {
			case <-job.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("update did not finish")
			}
			record, ok := st.GetUpdateHistory(summary.ID)
			if !ok || record.Scope != scope || record.Status != "completed" {
				t.Fatalf("record=%+v", record)
			}
			if scope == "rules" {
				if strings.Join(record.RequestedRuleIDs, ",") != "apple,child" || strings.Join(record.EffectiveRuleIDs, ",") != "apple,child" || record.RulesSucceeded != 2 {
					t.Fatalf("rule update=%+v", record)
				}
			} else if len(record.RequestedRuleIDs) != 0 || len(record.EffectiveRuleIDs) != 0 || record.RulesTotal != 0 {
				t.Fatalf("geo update=%+v", record)
			}
		})
	}
}

// deadlineRecorder records the write deadline the SSE handler installs.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	set      bool
	deadline time.Time
}

func (r *deadlineRecorder) SetWriteDeadline(deadline time.Time) error {
	r.set = true
	r.deadline = deadline
	return nil
}

// The server's absolute WriteTimeout would otherwise end a long update stream.
func TestUpdateEventsClearsWriteDeadline(t *testing.T) {
	s, _, _ := testServer(t)
	job, err := s.updates.Start(updates.Request{Scope: "all"}, "web")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-job.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("update timeout")
	}

	router := chi.NewRouter()
	router.Get("/updates/{updateID}/events", s.handleUpdateEvents)
	rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	router.ServeHTTP(rec, authorized(http.MethodGet, "/updates/"+job.ID+"/events", nil))

	if !rec.set || !rec.deadline.IsZero() {
		t.Fatalf("write deadline = %v (set=%v), want it cleared", rec.deadline, rec.set)
	}
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "event: complete") {
		t.Fatalf("events=%d %s", rec.Code, rec.Body.String())
	}
}

// A stream that only cleared the deadline on a recorder would still be cut off
// by the real server, so run this one over a live connection.
func TestUpdateEventsSurviveWriteTimeout(t *testing.T) {
	s, cfg, st := testServer(t)
	runner := &blockingAPIRunner{started: make(chan struct{}), release: make(chan struct{})}
	manager, err := updates.NewManager(cfg, s.DataDir, st, runner, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.updates = manager
	job, err := manager.Start(updates.Request{Scope: "all"}, "web")
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started

	srv := httptest.NewUnstartedServer(s.Handler())
	srv.Config.WriteTimeout = 150 * time.Millisecond
	srv.Start()
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/updates/"+job.ID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer abc")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Finish the job only after the server's write deadline has passed.
	time.Sleep(300 * time.Millisecond)
	if err := manager.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	close(runner.release)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "event: complete") {
		t.Fatalf("stream ended without the final event: %s", body)
	}
}
