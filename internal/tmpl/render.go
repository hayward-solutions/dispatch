package tmpl

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Renderer struct {
	templates map[string]*template.Template
	partials  map[string]*template.Template
	fs        fs.FS
	funcMap   template.FuncMap
	dev       bool
	mu        sync.RWMutex
}

func New(fsys fs.FS, dev bool) (*Renderer, error) {
	r := &Renderer{
		fs:  fsys,
		dev: dev,
		funcMap: template.FuncMap{
			"upper":    strings.ToUpper,
			"lower":    strings.ToLower,
			"contains": strings.Contains,
			"join":     strings.Join,
			"dict":     dictFunc,
			"duration": durationFunc,
			"timeAgo":  timeAgoFunc,
			"shortSHA": func(s string) string { if len(s) > 7 { return s[:7] }; return s },
			"add":      func(a, b int) int { return a + b },
			"sub":      func(a, b int) int { return a - b },
			"seq": func(n int) []int {
				s := make([]int, n)
				for i := range s {
					s[i] = i
				}
				return s
			},
			"jsonPretty":  jsonPrettyFunc,
			"mapGet":      mapGetFunc,
			"mapEntries":  mapEntriesFunc,
			"toSlice":     toSliceFunc,
			"toBool":      toBoolFunc,
		},
	}

	if err := r.parse(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Renderer) parse() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.templates = make(map[string]*template.Template)
	r.partials = make(map[string]*template.Template)

	layoutFiles, err := fs.Glob(r.fs, "layouts/*.html")
	if err != nil {
		return fmt.Errorf("glob layouts: %w", err)
	}

	// Parse pages (each page inherits layouts)
	pageFiles, err := fs.Glob(r.fs, "pages/*.html")
	if err != nil {
		return fmt.Errorf("glob pages: %w", err)
	}

	// Also include partials that are referenced from pages
	partialFiles, err := fs.Glob(r.fs, "partials/*.html")
	if err != nil {
		return fmt.Errorf("glob partials: %w", err)
	}

	for _, page := range pageFiles {
		name := strings.TrimSuffix(filepath.Base(page), ".html")

		files := make([]string, 0, len(layoutFiles)+len(partialFiles)+1)
		files = append(files, layoutFiles...)
		files = append(files, partialFiles...)
		files = append(files, page)

		t, err := template.New("").Funcs(r.funcMap).ParseFS(r.fs, files...)
		if err != nil {
			return fmt.Errorf("parse page %s: %w", name, err)
		}
		r.templates[name] = t
	}

	// Parse partials together so they can reference each other
	if len(partialFiles) > 0 {
		base, err := template.New("").Funcs(r.funcMap).ParseFS(r.fs, partialFiles...)
		if err != nil {
			return fmt.Errorf("parse partials: %w", err)
		}
		for _, partial := range partialFiles {
			name := strings.TrimSuffix(filepath.Base(partial), ".html")
			r.partials[name] = base
		}
	}

	return nil
}

func (r *Renderer) Page(w http.ResponseWriter, name string, data any) {
	if r.dev {
		if err := r.parse(); err != nil {
			slog.Error("reparse templates", "error", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
	}

	r.mu.RLock()
	t, ok := r.templates[name]
	r.mu.RUnlock()

	if !ok {
		slog.Error("template not found", "name", name)
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		slog.Error("render page", "name", name, "error", err)
	}
}

func (r *Renderer) Partial(w http.ResponseWriter, name string, data any) {
	if r.dev {
		if err := r.parse(); err != nil {
			slog.Error("reparse templates", "error", err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
	}

	r.mu.RLock()
	t, ok := r.partials[name]
	r.mu.RUnlock()

	if !ok {
		slog.Error("partial not found", "name", name)
		http.Error(w, "partial not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name+".html", data); err != nil {
		slog.Error("render partial", "name", name, "error", err)
	}
}

func (r *Renderer) RenderPartialTo(wr io.Writer, name string, data any) error {
	r.mu.RLock()
	t, ok := r.partials[name]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("partial not found: %s", name)
	}
	return t.ExecuteTemplate(wr, name+".html", data)
}

// durationFunc returns a human-readable duration between two times.
func durationFunc(start, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return ""
	}
	d := end.Sub(start)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
}

// timeAgoFunc returns a human-readable relative time string.
func timeAgoFunc(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// jsonPrettyFunc returns indented JSON for displaying complex values.
func jsonPrettyFunc(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// dictFunc creates a map from alternating key-value pairs for use in templates.
// Usage: {{template "partial" dict "key1" val1 "key2" val2}}
func dictFunc(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict requires even number of arguments")
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict keys must be strings")
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}

// MapEntry is a key-value pair for iterating maps in templates.
type MapEntry struct {
	Key   string
	Value any
}

// mapGetFunc safely indexes into a map[string]any typed as any.
func mapGetFunc(m any, key string) any {
	if m == nil {
		return nil
	}
	switch mm := m.(type) {
	case map[string]any:
		return mm[key]
	default:
		return nil
	}
}

// mapEntriesFunc converts a map[string]any to a sorted slice of MapEntry.
func mapEntriesFunc(m any) []MapEntry {
	if m == nil {
		return nil
	}
	mm, ok := m.(map[string]any)
	if !ok {
		return nil
	}
	entries := make([]MapEntry, 0, len(mm))
	for k, v := range mm {
		entries = append(entries, MapEntry{Key: k, Value: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries
}

// toSliceFunc coerces any to []any. Returns empty slice for nil.
func toSliceFunc(v any) []any {
	if v == nil {
		return []any{}
	}
	if s, ok := v.([]any); ok {
		return s
	}
	return []any{}
}

// toBoolFunc safely coerces any to bool.
func toBoolFunc(v any) bool {
	if v == nil {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	default:
		return false
	}
}
