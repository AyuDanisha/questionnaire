package handlers

import (
	"net/http"
	"questionnaire-app/internal/database"
	"questionnaire-app/internal/models"
)

func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	stats, err := database.GetDashboardStats(user.ID)
	if err != nil {
		stats = &models.DashboardStats{}
	}

	questionnaires, err := database.GetQuestionnairesByDeveloper(user.ID)
	if err != nil {
		questionnaires = []models.Questionnaire{}
	}

	// Get additional stats for each questionnaire
	type QuestionnaireWithStats struct {
		models.Questionnaire
		TotalQuestions int
		TotalResponses int
	}

	var qsWithStats []QuestionnaireWithStats
	for _, q := range questionnaires {
		qStats, _ := database.GetQuestionnaireStats(q.ID)
		qsWithStats = append(qsWithStats, QuestionnaireWithStats{
			Questionnaire:  q,
			TotalQuestions: qStats.TotalQuestions,
			TotalResponses: qStats.TotalResponses,
		})
	}

	data := map[string]interface{}{
		"User":           user,
		"Stats":          stats,
		"Questionnaires": qsWithStats,
		"AppName":        "Business Requirement Collector",
	}

	RenderTemplate(w, "dashboard.html", data)
}
