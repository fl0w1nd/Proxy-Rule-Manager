package geodata

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func retryRequestTestContext(
	retries int,
	onRetry func(attempt, total int, delay time.Duration, err error),
) context.Context {
	return context.WithValue(context.Background(), providerRetryContextKey{}, providerRetryConfig{
		retries:    retries,
		retryDelay: time.Millisecond,
		onRetry:    onRetry,
	})
}

func TestRetryProviderRequestSucceedsAfterTransientFailures(t *testing.T) {
	attempts := 0
	var retries []int
	ctx := retryRequestTestContext(
		2,
		func(attempt, total int, _ time.Duration, _ error) {
			if total != 2 {
				t.Fatalf("retry total = %d", total)
			}
			retries = append(retries, attempt)
		},
	)
	err := retryProviderRequest(
		ctx,
		func() error {
			attempts++
			if attempts < 3 {
				return markProviderErrorRetryable(errors.New("TLS handshake timeout"))
			}
			return nil
		},
	)
	if err != nil || attempts != 3 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
	if len(retries) != 2 || retries[0] != 1 || retries[1] != 2 {
		t.Fatalf("retry notices = %v", retries)
	}
}

func TestRetryProviderRequestStopsOnPermanentFailure(t *testing.T) {
	attempts := 0
	err := retryProviderRequest(
		retryRequestTestContext(2, nil),
		func() error {
			attempts++
			return errors.New("release is missing dlc.dat")
		},
	)
	if err == nil || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRetryProviderRequestReportsExhaustedAttempts(t *testing.T) {
	attempts := 0
	err := retryProviderRequest(
		retryRequestTestContext(2, nil),
		func() error {
			attempts++
			return markProviderErrorRetryable(errors.New("temporary network failure"))
		},
	)
	if err == nil || attempts != 3 || err.Error() != "after 3 attempts: temporary network failure" {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestRetryProviderRequestHonorsCancellation(t *testing.T) {
	baseCtx, cancel := context.WithCancel(context.Background())
	ctx := context.WithValue(baseCtx, providerRetryContextKey{}, providerRetryConfig{
		retries:    2,
		retryDelay: time.Millisecond,
		onRetry: func(_, _ int, _ time.Duration, _ error) {
			cancel()
		},
	})
	attempts := 0
	err := retryProviderRequest(ctx, func() error {
		attempts++
		return markProviderErrorRetryable(errors.New("temporary network failure"))
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

// Refresh must keep usable caches when upstream checks are unavailable, and
// transfer the database only when its content cannot be reused.
func TestRefreshContent(t *testing.T) {
	for _, tc := range []struct {
		name, ref string
		checksum  bool
	}{
		{"release checksum", "", true},
		{"branch checksum", "release", true},
		{"release digest", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := Source{Repository: "owner/repo", Asset: "data.dat", Checksum: tc.checksum, Ref: tc.ref}
			payload := "first database"
			apiStatus, checksumStatus := http.StatusOK, http.StatusOK
			checksumOverride := ""
			downloads := 0
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				status, body := http.StatusOK, ""
				switch {
				case strings.Contains(req.URL.Path, "/repos/"):
					status, body = apiStatus, `{"tag_name":"same-tag","assets":[{"name":"data.dat","browser_download_url":"https://fixture.test/data.dat"},{"name":"data.dat.sha256sum","browser_download_url":"https://fixture.test/data.dat.sha256sum"}]}`
					if !tc.checksum {
						digest := fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
						if checksumOverride != "" {
							digest = checksumOverride
						}
						body = strings.Replace(body, `"name":"data.dat",`, `"name":"data.dat","digest":"sha256:`+digest+`",`, 1)
					}
				case strings.HasSuffix(req.URL.Path, ".sha256sum"):
					status, body = checksumStatus, fmt.Sprintf("%x data.dat", sha256.Sum256([]byte(payload)))
					if checksumOverride != "" {
						body = checksumOverride
					}
				case req.URL.String() == "https://fixture.test/data.dat",
					req.URL.String() == "https://github.com/owner/repo/releases/latest/download/data.dat",
					req.URL.String() == "https://raw.githubusercontent.com/owner/repo/release/data.dat":
					downloads++
					body = payload
				default:
					status = http.StatusNotFound
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			newManager := func() *Manager[string] {
				m := NewManager(dir, "test", map[string]Source{"test": source}, func(data []byte, provider, version string) (*Cache[string], error) {
					return &Cache[string]{Provider: provider, ResolvedVersion: version, FetchedAt: time.Now().Format(time.RFC3339Nano), Entries: map[string][]string{"all": {string(data)}}}, nil
				}, nil)
				m.SetHTTPClient(client)
				return m
			}
			m := newManager()
			first, err := m.Refresh(context.Background(), "test")
			if err != nil {
				t.Fatal(err)
			}
			// A new manager exercises persisted cache reuse after a restart.
			m = newManager()
			cached, err := m.Refresh(context.Background(), "test")
			if err != nil || downloads != 1 || cached.FetchedAt != first.FetchedAt {
				t.Fatalf("cached=%+v downloads=%d err=%v", cached, downloads, err)
			}
			apiStatus, checksumStatus = http.StatusForbidden, http.StatusNotFound
			fallback, err := m.Refresh(context.Background(), "test")
			if err != nil || downloads != 2 || fallback.FetchedAt != first.FetchedAt {
				t.Fatalf("fallback=%+v downloads=%d err=%v", fallback, downloads, err)
			}
			payload = "second database"
			changed, err := m.Refresh(context.Background(), "test")
			if err != nil || changed.Entries["all"][0] != payload {
				t.Fatalf("changed=%+v err=%v", changed, err)
			}
			apiStatus, checksumStatus = http.StatusOK, http.StatusOK
			checksumOverride = "malformed checksum"
			payload = "third database"
			changed, err = m.Refresh(context.Background(), "test")
			if err != nil || changed.Entries["all"][0] != payload {
				t.Fatalf("malformed checksum fallback=%+v err=%v", changed, err)
			}
			checksumOverride = ""
			path, _ := m.RawPath("test")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if _, err := m.Refresh(context.Background(), "test"); err != nil {
				t.Fatal(err)
			}
			if restored, err := os.ReadFile(path); err != nil || string(restored) != payload {
				t.Fatalf("restored original=%q err=%v", restored, err)
			}
			savedPayload := payload
			payload = "unverified database"
			checksumOverride = strings.Repeat("0", 64)
			if _, err := m.Refresh(context.Background(), "test"); err == nil {
				t.Fatal("accepted a checksum mismatch")
			}
			retained, err := m.Read("test")
			if err != nil || retained.Entries["all"][0] != savedPayload {
				t.Fatalf("cache lost after failure: %+v %v", retained, err)
			}
			if raw, err := os.ReadFile(path); err != nil || string(raw) != savedPayload {
				t.Fatalf("original lost after failure: %q %v", raw, err)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
