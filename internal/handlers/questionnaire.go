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
	"regexp"
	"strconv"
	"strings"
)

// ManageQuestionsHandler handles the manual management page
func ManageQuestionsHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Extract ID safely
	// Expected path: /questionnaire/{id}/manage
	path := strings.TrimPrefix(r.URL.Path, "/questionnaire/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	idStr := parts[0]
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
			qType := r.FormValue("question_type")
			options := r.FormValue("options") // Textarea untuk opsi

			if qText != "" {
				database.CreateSingleQuestion(models.Question{
					QuestionnaireID: id,
					QuestionText:    qText,
					Category:        category,
					QuestionType:    qType,
					Options:         options,
				})
			}

		case "update":
			qID, _ := strconv.Atoi(r.FormValue("question_id"))
			qText := strings.TrimSpace(r.FormValue("question_text"))
			category := r.FormValue("category")
			qType := r.FormValue("question_type")
			options := r.FormValue("options")

			if qID > 0 && qText != "" {
				database.UpdateQuestion(qID, qText, category, qType, options)
			}

		case "delete":
			qID, _ := strconv.Atoi(r.FormValue("question_id"))
			if qID > 0 {
				database.DeleteQuestion(qID)
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

	// Extract ID safely
	idStr := strings.TrimPrefix(r.URL.Path, "/questionnaire/")
	idStr = strings.TrimSuffix(idStr, "/") // Handle trailing slash
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

	// Handle POST Actions (Publish & Unpublish)
	if r.Method == "POST" {
		action := r.FormValue("action")

		if action == "unpublish" {
			err := database.UpdateQuestionnaireStatus(id, "draft")
			if err != nil {
				log.Printf("Error unpublishing: %v", err)
			}
			// Redirect back to detail page (which will show Draft state)
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}

		if action == "publish" {
			err := database.UpdateQuestionnaireStatus(id, "active")
			if err != nil {
				log.Printf("Error publishing: %v", err)
				// Bisa tambahkan flash message error di sini jika perlu
			}
			// Redirect back to detail page (which will show Active state)
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}
	}

	// GET Logic
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
				qType := "text_long" // Default
				var options string

				// 1. Parse Category
				if strings.Contains(q, "[SRM]") {
					category = "SRM"
				}

				// 2. Parse Type
				reType := regexp.MustCompile(`\]\s*\[(.*?)\]`)
				matchesType := reType.FindStringSubmatch(q)
				if len(matchesType) > 1 {
					detectedType := strings.ToUpper(matchesType[1])
					switch detectedType {
					case "TEXT_SHORT":
						qType = "text_short"
					case "RADIO":
						qType = "radio"
					case "CHECKBOX":
						qType = "checkbox"
					case "FILE":
						qType = "file"
					default:
						qType = "text_long"
					}
				}

				// 3. Parse Options
				reOpts := regexp.MustCompile(`\(([^)]+)\)`)
				matchesOpts := reOpts.FindStringSubmatch(q)
				if len(matchesOpts) > 1 && (qType == "radio" || qType == "checkbox") {
					rawOpts := strings.Split(matchesOpts[1], ",")
					var cleanOpts []string
					for _, opt := range rawOpts {
						cleanOpts = append(cleanOpts, strings.TrimSpace(opt))
					}
					options = strings.Join(cleanOpts, "\n")
				}

				// 4. Clean Text
				cleanText := q
				cleanText = strings.TrimPrefix(cleanText, "[BM] ")
				cleanText = strings.TrimPrefix(cleanText, "[SRM] ")
				cleanText = reType.ReplaceAllString(cleanText, "")
				cleanText = reOpts.ReplaceAllString(cleanText, "")
				cleanText = strings.TrimSpace(cleanText)

				if cleanText != "" {
					questionModels = append(questionModels, models.Question{
						QuestionnaireID: qID,
						QuestionText:    cleanText,
						Category:        category,
						OrderNum:        i + 1,
						QuestionType:    qType,
						Options:         options,
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

		// If Manual Mode (or empty project desc), redirect to manage page
		http.Redirect(w, r, fmt.Sprintf("/questionnaire/%d/manage", qID), http.StatusSeeOther)
	}
}

// GenerateQuestionsAPIHandler for AJAX preview
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

// UpdateQuestionAPIHandler for updating via AJAX (optional, if needed)
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
		Type     string `json:"type"`
		Options  string `json:"options"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Call database with correct arguments
	err = database.UpdateQuestion(req.ID, req.Text, req.Category, req.Type, req.Options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func cleanQuestionText(q string) string {
	qText := strings.TrimSpace(q)
	qText = strings.TrimPrefix(qText, "[BM] ")
	qText = strings.TrimPrefix(qText, "[SRM] ")
	// Remove numbering like "1. ", "2. " etc
	if len(qText) > 0 {
		for j := 0; j < 3 && j < len(qText); j++ {
			if qText[j] >= '0' && qText[j] <= '9' {
				continue
			}
			if qText[j] == '.' || qText[j] == ' ' {
				if j+1 < len(qText) {
					qText = strings.TrimPrefix(qText[j+1:], " ")
				}
				break
			}
		}
	}
	return qText
}

func generateShareToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
