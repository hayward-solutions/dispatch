package tmpl

import (
	"bytes"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"
)

func TestDurationFunc(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		end      time.Time
		expected string
	}{
		{"zero start", time.Time{}, time.Now(), ""},
		{"zero end", time.Now(), time.Time{}, ""},
		{"both zero", time.Time{}, time.Time{}, ""},
		{"30 seconds", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 0, 0, 30, 0, time.UTC), "30s"},
		{"2 minutes 15 seconds", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 0, 2, 15, 0, time.UTC), "2m 15s"},
		{"exact minute", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 0, 1, 0, 0, time.UTC), "1m 0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := durationFunc(tt.start, tt.end)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTimeAgoFunc(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{"zero", time.Time{}, ""},
		{"just now", now.Add(-10 * time.Second), "just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1 minute ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1 hour ago"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"1 day ago", now.Add(-24 * time.Hour), "1 day ago"},
		{"5 days ago", now.Add(-5 * 24 * time.Hour), "5 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timeAgoFunc(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTimeAgoFunc_OldDate(t *testing.T) {
	old := time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)
	result := timeAgoFunc(old)
	if result != "Jun 15, 2020" {
		t.Errorf("expected formatted date, got %q", result)
	}
}

func TestDictFunc(t *testing.T) {
	m, err := dictFunc("key1", "val1", "key2", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["key1"] != "val1" {
		t.Errorf("key1: expected 'val1', got %v", m["key1"])
	}
	if m["key2"] != 42 {
		t.Errorf("key2: expected 42, got %v", m["key2"])
	}
}

func TestDictFunc_OddArgs(t *testing.T) {
	_, err := dictFunc("key1")
	if err == nil {
		t.Fatal("expected error for odd number of args")
	}
}

func TestDictFunc_NonStringKey(t *testing.T) {
	_, err := dictFunc(123, "val")
	if err == nil {
		t.Fatal("expected error for non-string key")
	}
}

func TestDictFunc_Empty(t *testing.T) {
	m, err := dictFunc()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"layouts/base.html": &fstest.MapFile{
			Data: []byte(`{{define "base"}}<!DOCTYPE html><html><body>{{template "content" .}}</body></html>{{end}}`),
		},
		"pages/home.html": &fstest.MapFile{
			Data: []byte(`{{define "content"}}Hello {{.Name}}{{end}}`),
		},
		"partials/greeting.html": &fstest.MapFile{
			Data: []byte(`{{define "greeting.html"}}Hi {{.Name}}!{{end}}`),
		},
	}
}

func TestNew_Success(t *testing.T) {
	r, err := New(testFS(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil renderer")
	}
}

func TestPage_Render(t *testing.T) {
	r, err := New(testFS(), false)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := httptest.NewRecorder()
	r.Page(w, "home", struct{ Name string }{"World"})

	body := w.Body.String()
	if body != `<!DOCTYPE html><html><body>Hello World</body></html>` {
		t.Errorf("unexpected body: %s", body)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("unexpected content type: %s", ct)
	}
}

func TestPage_NotFound(t *testing.T) {
	r, err := New(testFS(), false)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := httptest.NewRecorder()
	r.Page(w, "nonexistent", nil)

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPartial_Render(t *testing.T) {
	r, err := New(testFS(), false)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := httptest.NewRecorder()
	r.Partial(w, "greeting", struct{ Name string }{"Alice"})

	body := w.Body.String()
	if body != "Hi Alice!" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestPartial_NotFound(t *testing.T) {
	r, err := New(testFS(), false)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	w := httptest.NewRecorder()
	r.Partial(w, "nonexistent", nil)

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestRenderPartialTo(t *testing.T) {
	r, err := New(testFS(), false)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	var buf bytes.Buffer
	err = r.RenderPartialTo(&buf, "greeting", struct{ Name string }{"Bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "Hi Bob!" {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestRenderPartialTo_NotFound(t *testing.T) {
	r, err := New(testFS(), false)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	var buf bytes.Buffer
	err = r.RenderPartialTo(&buf, "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing partial")
	}
}
