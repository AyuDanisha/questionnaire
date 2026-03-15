package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"questionnaire-app/internal/models"
	"strings"
	"time"
)

var templates *template.Template

var templateFuncs = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
	"sub": func(a, b int) int {
		return a - b
	},
	"mul": func(a, b int) int {
		return a * b
	},
	"div": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"lower": strings.ToLower,
	"upper": strings.ToUpper,
	"title": strings.Title,
	"trim":  strings.TrimSpace,
	"js": func(s string) template.JS {
		return template.JS(s)
	},
	"formatDate": func(t time.Time) string {
		return t.Format("02 Jan 2006 15:04")
	},
	"len": func(v interface{}) int {
		switch v := v.(type) {
		case []interface{}:
			return len(v)
		case []models.Question:
			return len(v)
		case []models.ClientSession:
			return len(v)
		default:
			return 0
		}
	},
	"countCompleted": func(sessions []models.ClientSession) int {
		count := 0
		for _, s := range sessions {
			if s.Status == "completed" {
				count++
			}
		}
		return count
	},
	"countInProgress": func(sessions []models.ClientSession) int {
		count := 0
		for _, s := range sessions {
			if s.Status == "in_progress" {
				count++
			}
		}
		return count
	},
	"calculateProgress": func(answered, total int) int {
		if total == 0 {
			return 0
		}
		return (answered * 100) / total
	},
	"split": func(s, sep string) []string {
		if s == "" {
			return []string{}
		}
		return strings.Split(s, sep)
	},
	"contains": func(s, substr string) bool {
		return strings.Contains(s, substr)
	},
}

func InitTemplates() {
	var allFiles []string

	err := filepath.Walk("templates", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".html") {
			allFiles = append(allFiles, path)
		}
		return nil
	})

	if err != nil {
		log.Printf("Error walking templates directory: %v", err)
		return
	}

	// Create template with functions
	templates = template.New("").Funcs(templateFuncs)

	// Parse all templates
	templates, err = templates.ParseFiles(allFiles...)
	if err != nil {
		log.Printf("Error parsing templates: %v", err)
		return
	}

	log.Printf("Loaded %d templates", len(allFiles))
}

func RenderTemplate(w http.ResponseWriter, name string, data interface{}) {
	if templates == nil {
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	tmpl := templates.Lookup(name)
	if tmpl == nil {
		http.Error(w, "Template not found: "+name, http.StatusInternalServerError)
		return
	}

	// Set header
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	err := tmpl.Execute(w, data)
	if err != nil {
		log.Printf("Error executing template %s: %v", name, err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
	}
}
