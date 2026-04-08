package views

import (
	"embed"
	"html/template"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*.html templates/**/*.html
var TemplatesFS embed.FS

type Renderer struct {
	pages map[string]*template.Template
}

func NewRenderer(_ bool) *Renderer {
	r := &Renderer{pages: make(map[string]*template.Template)}
	r.loadTemplates()
	return r
}

func (r *Renderer) funcMap() template.FuncMap {
	return template.FuncMap{
		"hasPermission": func(perms map[string]bool, perm string) bool {
			return perms != nil && perms[perm]
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			d := make(map[string]interface{})
			for i := 0; i < len(values)-1; i += 2 {
				if key, ok := values[i].(string); ok {
					d[key] = values[i+1]
				}
			}
			return d
		},
		"gt": func(a, b int) bool { return a > b },
		"lt": func(a, b int) bool { return a < b },
		"eq": func(a, b interface{}) bool { return a == b },
	}
}

func (r *Renderer) loadTemplates() {
	// Read shared files: layout + partials
	layoutBytes, _ := fs.ReadFile(TemplatesFS, "templates/layout.html")
	layoutStr := string(layoutBytes)

	// Collect partial files
	var partialStrs []string
	fs.WalkDir(TemplatesFS, "templates/partials", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, _ := fs.ReadFile(TemplatesFS, path)
		partialStrs = append(partialStrs, string(b))
		return nil
	})
	partials := strings.Join(partialStrs, "\n")

	// For each page template, create an isolated template set = layout + partials + that page
	entries, _ := fs.ReadDir(TemplatesFS, "templates")
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "layout.html" {
			continue
		}

		pageBytes, _ := fs.ReadFile(TemplatesFS, "templates/"+name)
		pageName := strings.TrimSuffix(name, ".html")

		combined := layoutStr + "\n" + partials + "\n" + string(pageBytes)
		t, err := template.New(pageName).Funcs(r.funcMap()).Parse(combined)
		if err != nil {
			slog.Error("template parse failed", "page", pageName, "error", err)
			continue
		}
		r.pages[pageName] = t
	}

	// Register a "fragments" pseudo-page for rendering modals/partials directly
	fragCombined := partials
	ft, err := template.New("_fragments").Funcs(r.funcMap()).Parse(fragCombined)
	if err == nil {
		r.pages["_fragments"] = ft
	}

	slog.Info("templates loaded", "count", len(r.pages))
}

// HTML renders a full page (layout wrapper + page content).
func (r *Renderer) HTML(c *gin.Context, code int, name string, data interface{}) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(code)

	t, ok := r.pages[name]
	if !ok {
		slog.Error("template not found", "name", name)
		c.String(500, "template %q not found", name)
		return
	}

	if err := t.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		slog.Error("template render failed", "name", name, "error", err)
	}
}

// HTMLFragment renders a named define block (e.g., a modal) without the layout wrapper.
func (r *Renderer) HTMLFragment(c *gin.Context, code int, block string, data interface{}) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Status(code)

	t, ok := r.pages["_fragments"]
	if !ok {
		slog.Error("fragments template not loaded")
		c.String(500, "fragments template not loaded")
		return
	}

	if err := t.ExecuteTemplate(c.Writer, block, data); err != nil {
		slog.Error("fragment render failed", "block", block, "error", err)
	}
}
