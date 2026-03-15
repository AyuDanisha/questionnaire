package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"questionnaire-app/internal/ai"
	"questionnaire-app/internal/database"
	"questionnaire-app/internal/models"
	"strconv"
	"strings"
)

// Tambahkan/Update fungsi ManageQuestionsHandler
func ManageQuestionsHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Extract ID
	path := strings.TrimPrefix(r.URL.Path, "/questionnaire/")
	idStr := strings.Split(path, "/")[0]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	questionnaire, err := database.GetQuestionnaireByID(id)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	if questionnaire.DeveloperID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Handle POST Requests
	if r.Method == "POST" {
		action := r.FormValue("action")

		switch action {
		case "add":
			qText := strings.TrimSpace(r.FormValue("question_text"))
			category := r.FormValue("category")
			if qText != "" && (category == "BM" || category == "SRM") {
				database.CreateSingleQuestion(models.Question{
					QuestionnaireID: id,
					QuestionText:    qText,
					Category:        category,
				})
			}

		case "delete":
			qID, _ := strconv.Atoi(r.FormValue("question_id"))
			if qID > 0 {
				database.DeleteQuestion(qID)
			}

		case "update": // FITUR BARU: Update Pertanyaan
			qID, _ := strconv.Atoi(r.FormValue("question_id"))
			qText := strings.TrimSpace(r.FormValue("question_text"))
			category := r.FormValue("category")
			if qID > 0 && qText != "" {
				database.UpdateQuestion(qID, qText, category)
			}

		case "publish":
			database.UpdateQuestionnaireStatus(id, "active")
			http.Redirect(w, r, fmt.Sprintf("/questionnaire/%d", id), http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
		return
	}

	// GET
	questions, _ := database.GetQuestionsByQuestionnaire(id)

	data := map[string]interface{}{
		"User":          user,
		"Questionnaire": questionnaire,
		"Questions":     questions,
		"Error":         r.URL.Query().Get("error"),
	}

	RenderTemplate(w, "manage_questions.html", data)
}

func QuestionnaireDetailHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/questionnaire/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid questionnaire ID", http.StatusBadRequest)
		return
	}

	questionnaire, err := database.GetQuestionnaireByID(id)
	if err != nil {
		http.Error(w, "Questionnaire not found", http.StatusNotFound)
		return
	}

	if questionnaire.DeveloperID != user.ID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Handle Unpublish Action
	if r.Method == "POST" {
		action := r.FormValue("action")
		if action == "unpublish" {
			database.UpdateQuestionnaireStatus(id, "draft")
			http.Redirect(w, r, fmt.Sprintf("/questionnaire/%d/manage", id), http.StatusSeeOther)
			return
		}
	}

	questions, _ := database.GetQuestionsByQuestionnaire(id)
	sessions, _ := database.GetClientSessionsByQuestionnaire(id)

	type QuestionWithStats struct {
		models.Question
		AnsweredCount int
	}
	var questionsWithStats []QuestionWithStats
	for _, q := range questions {
		count, _ := database.GetAnsweredCountForQuestion(q.ID)
		questionsWithStats = append(questionsWithStats, QuestionWithStats{
			Question:      q,
			AnsweredCount: count,
		})
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	shareURL := fmt.Sprintf("%s://%s/form/%s", scheme, r.Host, questionnaire.ShareToken)

	data := map[string]interface{}{
		"User":           user,
		"Questionnaire":  questionnaire,
		"Questions":      questionsWithStats,
		"Sessions":       sessions,
		"ShareURL":       shareURL,
		"TotalQuestions": len(questions),
	}

	RenderTemplate(w, "questionnaire_detail.html", data)
}

func CreateQuestionnaireHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == "GET" {
		RenderTemplate(w, "create_questionnaire.html", map[string]interface{}{
			"User": user,
		})
		return
	}

	if r.Method == "POST" {
		title := strings.TrimSpace(r.FormValue("title"))
		description := strings.TrimSpace(r.FormValue("description"))
		projectDesc := strings.TrimSpace(r.FormValue("project_desc"))
		mode := r.FormValue("mode") // "ai" or "manual"

		if title == "" {
			RenderTemplate(w, "create_questionnaire.html", map[string]interface{}{
				"User":  user,
				"Error": "Judul harus diisi",
				"Form":  r.Form,
			})
			return
		}

		shareToken := generateShareToken()

		qID, err := database.CreateQuestionnaire(user.ID, title, description, projectDesc, shareToken)
		if err != nil {
			RenderTemplate(w, "create_questionnaire.html", map[string]interface{}{
				"User":  user,
				"Error": "Gagal membuat questionnaire: " + err.Error(),
				"Form":  r.Form,
			})
			return
		}

		// If AI Mode
		if mode == "ai" && projectDesc != "" {
			questions, err := ai.GenerateQuestions(projectDesc)
			if err != nil {
				log.Printf("Error generating questions: %v", err)
				http.Redirect(w, r, fmt.Sprintf("/questionnaire/%d/manage?error=ai_failed", qID), http.StatusSeeOther)
				return
			}

			var questionModels []models.Question
			for i, q := range questions {
				category := "BM"
				if strings.Contains(q, "[SRM]") || i >= 50 {
					category = "SRM"
				}

				qText := cleanQuestionText(q)
				if qText != "" {
					questionModels = append(questionModels, models.Question{
						QuestionnaireID: qID,
						QuestionText:    qText,
						Category:        category,
						OrderNum:        i + 1,
					})
				}
			}

			if len(questionModels) > 0 {
				database.CreateQuestions(questionModels)
				database.UpdateQuestionnaireStatus(qID, "active")
			}

			http.Redirect(w, r, fmt.Sprintf("/questionnaire/%d", qID), http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, fmt.Sprintf("/questionnaire/%d/manage", qID), http.StatusSeeOther)
	}
}

func GenerateQuestionsAPIHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectDesc string `json:"project_desc"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	questions, err := ai.GenerateQuestions(req.ProjectDesc)
	if err != nil {
		http.Error(w, "Failed to generate questions: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"questions": questions,
	})
}

func cleanQuestionText(q string) string {
	qText := strings.TrimSpace(q)
	qText = strings.TrimPrefix(qText, "[BM] ")
	qText = strings.TrimPrefix(qText, "[SRM] ")
	// Remove numbering like "1. ", "2. " etc
	for j := 0; j < 3 && j < len(qText); j++ {
		if qText[j] >= '0' && qText[j] <= '9' {
			continue
		}
		if qText[j] == '.' || qText[j] == ' ' {
			qText = strings.TrimPrefix(qText[j+1:], " ")
			break
		}
	}
	return qText
}

func UpdateQuestionAPIHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       int    `json:"id"`
		Text     string `json:"text"`
		Category string `json:"category"`
	}

	json.NewDecoder(r.Body).Decode(&req)

	err := database.UpdateQuestion(req.ID, req.Text, req.Category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func generateShareToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
