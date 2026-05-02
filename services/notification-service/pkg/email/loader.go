package email

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"notification-service/pkg/lang"
)

//go:embed templates/*/*.html
var templatesFS embed.FS

var templates = mustLoadTemplates()

func mustLoadTemplates() map[lang.Lang]map[string]*template.Template {
	out := make(map[lang.Lang]map[string]*template.Template)
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		panic(fmt.Errorf("email: read templates dir: %w", err))
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		l := lang.Lang(e.Name())
		if !lang.IsSupported(l) {
			continue
		}
		dir := path.Join("templates", e.Name())
		files, err := fs.ReadDir(templatesFS, dir)
		if err != nil {
			panic(fmt.Errorf("email: read %s: %w", dir, err))
		}
		out[l] = make(map[string]*template.Template, len(files))
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".html") {
				continue
			}
			name := strings.TrimSuffix(f.Name(), ".html")
			data, err := fs.ReadFile(templatesFS, path.Join(dir, f.Name()))
			if err != nil {
				panic(fmt.Errorf("email: read template %s/%s: %w", e.Name(), f.Name(), err))
			}
			tmpl, err := template.New(name).Parse(string(data))
			if err != nil {
				panic(fmt.Errorf("email: parse template %s/%s: %w", e.Name(), f.Name(), err))
			}
			out[l][name] = tmpl
		}
	}
	if _, ok := out[lang.Default]; !ok {
		panic(fmt.Errorf("email: default language %q has no templates", lang.Default))
	}
	return out
}

func Template(l lang.Lang, name string) *template.Template {
	if m, ok := templates[l]; ok {
		if t, ok := m[name]; ok {
			return t
		}
	}
	return templates[lang.Default][name]
}
