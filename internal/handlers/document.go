package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"questionnaire-app/internal/ai"
	"questionnaire-app/internal/database"
	"questionnaire-app/internal/models"
	"strconv"
	"strings"
)

func GenerateDocumentHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract session ID from URL: /document/generate/{id}
	idStr := strings.TrimPrefix(r.URL.Path, "/document/generate/")
	sessionID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Get client session using the new function
	session, err := database.GetClientSessionByID(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Check if document already exists
	existingDoc, _ := database.GetGeneratedDocument(sessionID)
	if existingDoc != nil {
		http.Redirect(w, r, fmt.Sprintf("/document/view/%d", sessionID), http.StatusSeeOther)
		return
	}

	// Get questionnaire
	questionnaire, err := database.GetQuestionnaireByID(session.QuestionnaireID)
	if err != nil {
		http.Error(w, "Questionnaire not found", http.StatusNotFound)
		return
	}

	// Get questions and answers
	questions, _ := database.GetQuestionsByQuestionnaire(session.QuestionnaireID)
	answers, _ := database.GetAnswersBySession(sessionID)

	answerMap := make(map[int]string)
	for _, a := range answers {
		answerMap[a.QuestionID] = a.AnswerText
	}

	var qaPairs []string
	for _, q := range questions {
		answer := answerMap[q.ID]
		if answer == "" {
			answer = "Tidak dijawab"
		}
		qaPairs = append(qaPairs, fmt.Sprintf("Q: %s\nA: %s", q.QuestionText, answer))
	}

	// Generate BM
	bmContent, err := ai.GenerateBM(questionnaire.ProjectDesc, qaPairs)
	if err != nil {
		// Log error and show user feedback
		RenderTemplate(w, "error.html", map[string]interface{}{
			"Error": "Gagal generate Business Modeling: " + err.Error(),
		})
		return
	}

	// Generate SRM
	srmContent, err := ai.GenerateSRM(questionnaire.ProjectDesc, qaPairs)
	if err != nil {
		RenderTemplate(w, "error.html", map[string]interface{}{
			"Error": "Gagal generate System Requirement: " + err.Error(),
		})
		return
	}

	// Save documents
	_, err = database.SaveGeneratedDocument(sessionID, bmContent, srmContent)
	if err != nil {
		http.Error(w, "Failed to save documents", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/document/view/%d", sessionID), http.StatusSeeOther)
}

func ViewDocumentHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Extract session ID from URL: /document/view/{id}
	idStr := strings.TrimPrefix(r.URL.Path, "/document/view/")
	sessionID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Get session info
	session, err := database.GetClientSessionByID(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Get document
	doc, err := database.GetGeneratedDocument(sessionID)
	if err != nil {
		// Jika dokumen belum ada, redirect ke halaman generate
		http.Redirect(w, r, fmt.Sprintf("/document/generate/%d", sessionID), http.StatusSeeOther)
		return
	}

	questionnaire, _ := database.GetQuestionnaireByID(session.QuestionnaireID)

	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "bm"
	}

	data := map[string]interface{}{
		"User":          user,
		"Document":      doc,
		"Session":       session,
		"Questionnaire": questionnaire,
		"ActiveTab":     tab,
	}

	RenderTemplate(w, "document.html", data)
}

func ExportDocumentHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	sessionID, err := strconv.Atoi(parts[3])
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}
	docType := parts[4]

	doc, err := database.GetGeneratedDocument(sessionID)
	if err != nil {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}

	var content string
	var filename string
	if docType == "bm" {
		content = doc.BMContent
		filename = "Business_Modeling.md"
	} else {
		content = doc.SRMContent
		filename = "System_Requirement_Modeling.md"
	}

	w.Header().Set("Content-Type", "text/markdown")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write([]byte(content))
}

func init() {
}

// Helper function that should be in database package
func GetClientSessionByID(id int) (*models.ClientSession, error) {
	s := &models.ClientSession{}
	err := database.DB.QueryRow(
		`SELECT id, questionnaire_id, client_name, client_email, client_company, token, status, submitted_at, created_at 
         FROM client_sessions WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.QuestionnaireID, &s.ClientName, &s.ClientEmail, &s.ClientCompany, &s.Token, &s.Status, &s.SubmittedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
