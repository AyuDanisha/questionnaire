package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"questionnaire-app/internal/database"
	"questionnaire-app/internal/models"
	"strconv"
	"strings"
)

func FormHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Extract token & Validate Questionnaire
	token := strings.TrimPrefix(r.URL.Path, "/form/")
	questionnaire, err := database.GetQuestionnaireByToken(token)
	if err != nil {
		RenderTemplate(w, "error.html", map[string]interface{}{
			"Error": "Form tidak ditemukan atau sudah tidak aktif",
		})
		return
	}

	// Validasi status aktif
	if questionnaire.Status != "active" {
		RenderTemplate(w, "error.html", map[string]interface{}{
			"Error": "Form ini sedang tidak aktif (Draft).",
		})
		return
	}

	// 2. Check Client Session (Cookie)
	sessionCookie, err := r.Cookie("client_session")
	var clientSession *models.ClientSession

	if err == nil {
		clientSession, _ = database.GetClientSessionByToken(sessionCookie.Value)
	}

	// 3. Handle New Client (Landing Page)
	// Jika tidak ada session atau session tidak cocok dengan questionnaire ID
	if clientSession == nil || clientSession.QuestionnaireID != questionnaire.ID {
		if r.Method == "GET" {
			RenderTemplate(w, "form_landing.html", map[string]interface{}{
				"Questionnaire": questionnaire,
			})
			return
		}

		// Create new session (POST from Landing Page)
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

			sessionToken := generateToken()
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

			http.SetCookie(w, &http.Cookie{
				Name:     "client_session",
				Value:    sessionToken,
				Path:     "/form/" + token,
				MaxAge:   86400, // 24 hours
				HttpOnly: true,
			})

			// Redirect ke halaman form pertama setelah buat session
			http.Redirect(w, r, "/form/"+token, http.StatusSeeOther)
			return
		}
	}

	// 4. Prepare Questions & Pagination
	questions, err := database.GetQuestionsByQuestionnaire(questionnaire.ID)
	if err != nil {
		questions = []models.Question{}
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		page, _ = strconv.Atoi(p)
		if page < 1 {
			page = 1
		}
	}

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

	// Get existing answers for pre-filling form
	answers, _ := database.GetAnswersBySession(clientSession.ID)
	answerMap := make(map[int]string)
	for _, a := range answers {
		answerMap[a.QuestionID] = a.AnswerText
	}

	// 5. Handle Form Submission (Answer Save)
	if r.Method == "POST" {
		action := r.FormValue("action")

		// Parse Multipart Form untuk handle file upload (maks 32MB)
		r.ParseMultipartForm(32 << 20)

		for _, q := range pageQuestions {
			var answer string
			formKey := "answer_" + strconv.Itoa(q.ID)

			switch q.QuestionType {
			case "file":
				// Handle File Upload
				file, handler, err := r.FormFile(formKey)
				if err == nil {
					defer file.Close()

					// Buat folder uploads jika belum ada
					uploadDir := "static/uploads"
					if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
						os.MkdirAll(uploadDir, 0755)
					}

					// Generate unique filename: sessionID_questionID_filename
					fileName := fmt.Sprintf("%d_%d_%s", clientSession.ID, q.ID, handler.Filename)
					filePath := filepath.Join(uploadDir, fileName)

					// Create file di server
					dst, err := os.Create(filePath)
					if err == nil {
						io.Copy(dst, file)
						dst.Close()

						// Simpan path relatif ke database
						answer = "/" + filePath
					}
				} else {
					// Jika tidak ada file baru upload, pertahankan jawaban lama
					if existingAns, ok := answerMap[q.ID]; ok {
						answer = existingAns
					}
				}

			case "checkbox":
				// Handle Checkbox (multiple values)
				vals := r.Form[formKey]
				answer = strings.Join(vals, ", ")

			default:
				// Handle Text, Radio, Text Short/Long
				answer = r.FormValue(formKey)
			}

			database.SaveAnswer(clientSession.ID, q.ID, answer)
		}

		// Action Logic
		if action == "save_later" {
			database.UpdateClientSessionStatus(clientSession.ID, "in_progress")
			RenderTemplate(w, "form_saved.html", map[string]interface{}{
				"Questionnaire": questionnaire,
				"Message":       "Progress Anda telah disimpan. Anda dapat melanjutkan nanti dengan membuka link yang sama.",
			})
			return
		}

		// Default action: "save"
		database.UpdateClientSessionStatus(clientSession.ID, "in_progress")

		if page >= totalPages {
			// Jika halaman terakhir, tandai selesai
			database.UpdateClientSessionStatus(clientSession.ID, "completed")
			http.Redirect(w, r, "/form/"+token+"/success", http.StatusSeeOther)
			return
		}

		// Redirect ke halaman berikutnya
		http.Redirect(w, r, "/form/"+token+"?page="+strconv.Itoa(page+1), http.StatusSeeOther)
		return
	}

	// 6. Render Form (GET Request)
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
