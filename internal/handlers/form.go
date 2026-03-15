package handlers

import (
	"net/http"
	"questionnaire-app/internal/database"
	"questionnaire-app/internal/models"
	"strconv"
	"strings"
)

func FormHandler(w http.ResponseWriter, r *http.Request) {
	// Extract token from URL
	token := strings.TrimPrefix(r.URL.Path, "/form/")
	questionnaire, err := database.GetQuestionnaireByToken(token)
	if err != nil {
		RenderTemplate(w, "error.html", map[string]interface{}{
			"Error": "Form tidak ditemukan atau sudah tidak aktif",
		})
		return
	}
	if questionnaire.Status != "active" {
		RenderTemplate(w, "error.html", map[string]interface{}{
			"Error": "Form ini sedang tidak aktif.",
		})
		return
	}

	// Check for client session token in cookie
	sessionCookie, err := r.Cookie("client_session")
	var clientSession *models.ClientSession

	if err == nil {
		clientSession, _ = database.GetClientSessionByToken(sessionCookie.Value)
	}

	// If no session or different questionnaire, show landing page
	if clientSession == nil || clientSession.QuestionnaireID != questionnaire.ID {
		if r.Method == "GET" {
			RenderTemplate(w, "form_landing.html", map[string]interface{}{
				"Questionnaire": questionnaire,
			})
			return
		}

		// Create new session
		if r.Method == "POST" {
			name := strings.TrimSpace(r.FormValue("name"))
			email := strings.TrimSpace(r.FormValue("email"))
			company := strings.TrimSpace(r.FormValue("company"))

			if name == "" || email == "" {
				RenderTemplate(w, "form_landing.html", map[string]interface{}{
					"Questionnaire": questionnaire,
					"Error":         "Nama dan email harus diisi",
				})
				return
			}

			// Generate session token
			sessionToken := generateToken()

			// Create session
			sessionID, err := database.CreateClientSession(questionnaire.ID, name, email, company, sessionToken)
			if err != nil {
				RenderTemplate(w, "form_landing.html", map[string]interface{}{
					"Questionnaire": questionnaire,
					"Error":         "Gagal membuat sesi",
				})
				return
			}

			clientSession = &models.ClientSession{
				ID:              sessionID,
				QuestionnaireID: questionnaire.ID,
				ClientName:      name,
				ClientEmail:     email,
				ClientCompany:   company,
				Token:           sessionToken,
				Status:          "in_progress",
			}

			// Set cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "client_session",
				Value:    sessionToken,
				Path:     "/form/" + token,
				MaxAge:   86400, // 24 hours
				HttpOnly: true,
			})
		}
	}

	// Get questions
	questions, err := database.GetQuestionsByQuestionnaire(questionnaire.ID)
	if err != nil {
		questions = []models.Question{}
	}

	// Get page number
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
		if page < 1 {
			page = 1
		}
	}

	// Pagination: 10 questions per page
	perPage := 10
	totalPages := (len(questions) + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}

	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if endIdx > len(questions) {
		endIdx = len(questions)
	}

	pageQuestions := questions[startIdx:endIdx]

	// Get existing answers
	answers, _ := database.GetAnswersBySession(clientSession.ID)
	answerMap := make(map[int]string)
	for _, a := range answers {
		answerMap[a.QuestionID] = a.AnswerText
	}

	// Handle form submission
	if r.Method == "POST" && r.FormValue("action") == "save" {
		for _, q := range pageQuestions {
			answer := r.FormValue("answer_" + strconv.Itoa(q.ID))
			database.SaveAnswer(clientSession.ID, q.ID, answer)
		}

		// Update session status
		database.UpdateClientSessionStatus(clientSession.ID, "in_progress")

		// Check if this is the last page
		if page >= totalPages {
			// Mark as completed
			database.UpdateClientSessionStatus(clientSession.ID, "completed")
			http.Redirect(w, r, "/form/"+token+"/success", http.StatusSeeOther)
			return
		}

		// Redirect to next page
		http.Redirect(w, r, "/form/"+token+"?page="+strconv.Itoa(page+1), http.StatusSeeOther)
		return
	}

	// Handle save and continue later
	if r.Method == "POST" && r.FormValue("action") == "save_later" {
		for _, q := range pageQuestions {
			answer := r.FormValue("answer_" + strconv.Itoa(q.ID))
			database.SaveAnswer(clientSession.ID, q.ID, answer)
		}
		RenderTemplate(w, "form_saved.html", map[string]interface{}{
			"Questionnaire": questionnaire,
			"Message":       "Progress Anda telah disimpan. Anda dapat melanjutkan nanti dengan membuka link yang sama.",
		})
		return
	}

	// Prepare data for template
	type QuestionWithAnswer struct {
		models.Question
		Answer string
	}

	var questionsWithAnswers []QuestionWithAnswer
	for _, q := range pageQuestions {
		questionsWithAnswers = append(questionsWithAnswers, QuestionWithAnswer{
			Question: q,
			Answer:   answerMap[q.ID],
		})
	}

	data := map[string]interface{}{
		"Questionnaire":  questionnaire,
		"Questions":      questionsWithAnswers,
		"ClientSession":  clientSession,
		"Page":           page,
		"TotalPages":     totalPages,
		"TotalQuestions": len(questions),
		"StartNumber":    startIdx + 1,
		"EndNumber":      endIdx,
	}

	RenderTemplate(w, "form.html", data)
}

func FormSuccessHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/form/")
	token = strings.TrimSuffix(token, "/success")

	questionnaire, err := database.GetQuestionnaireByToken(token)
	if err != nil {
		RenderTemplate(w, "error.html", map[string]interface{}{
			"Error": "Form tidak ditemukan",
		})
		return
	}

	RenderTemplate(w, "form_success.html", map[string]interface{}{
		"Questionnaire": questionnaire,
	})
}

func FormProgressAPIHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/api/form/progress/")

	clientSession, err := database.GetClientSessionByToken(token)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	questions, _ := database.GetQuestionsByQuestionnaire(clientSession.QuestionnaireID)
	answeredCount, _ := database.GetAnswerCountBySession(clientSession.ID)

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"total": ` + strconv.Itoa(len(questions)) + `, "answered": ` + strconv.Itoa(answeredCount) + `}`))
}
