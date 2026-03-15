package main

import (
	"log"
	"net/http"
	"questionnaire-app/config"
	"questionnaire-app/internal/database"
	"questionnaire-app/internal/handlers"
	"strings"
)

func main() {
	// Load configuration
	config.LoadConfig()

	// Connect to database
	database.ConnectDB()

	// Initialize templates
	handlers.InitTemplates()

	// Create router
	mux := http.NewServeMux()

	// Static files - CSS dan JS
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Public routes
	mux.HandleFunc("/", handlers.IndexHandler)
	mux.HandleFunc("/register", handlers.RegisterHandler)
	mux.HandleFunc("/login", handlers.LoginHandler)
	mux.HandleFunc("/logout", handlers.LogoutHandler)

	// Protected routes (developer)
	mux.HandleFunc("/dashboard", handlers.AuthMiddleware(handlers.DashboardHandler))
	mux.HandleFunc("/api/generate-questions", handlers.AuthMiddleware(handlers.GenerateQuestionsAPIHandler))
	mux.HandleFunc("/document/generate/", handlers.AuthMiddleware(handlers.GenerateDocumentHandler))
	mux.HandleFunc("/document/view/", handlers.AuthMiddleware(handlers.ViewDocumentHandler))
	mux.HandleFunc("/document/export/", handlers.AuthMiddleware(handlers.ExportDocumentHandler))
	mux.HandleFunc("/questionnaire/create", handlers.AuthMiddleware(handlers.CreateQuestionnaireHandler))
	mux.HandleFunc("/questionnaire/", handlers.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/manage") {
			handlers.ManageQuestionsHandler(w, r)
		} else {
			handlers.QuestionnaireDetailHandler(w, r)
		}
	}))
	mux.HandleFunc("/api/question/update", handlers.AuthMiddleware(handlers.UpdateQuestionAPIHandler))

	// Public form routes (client)
	mux.HandleFunc("/form/", handlers.FormHandler)
	mux.HandleFunc("/api/form/progress/", handlers.FormProgressAPIHandler)

	// Start server
	port := config.AppConfig.ServerPort
	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("Press Ctrl+C to stop")

	err := http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
