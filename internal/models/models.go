package models

import "time"

type Developer struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Questionnaire struct {
	ID          int       `json:"id"`
	DeveloperID int       `json:"developer_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ProjectDesc string    `json:"project_desc"`
	Status      string    `json:"status"` // draft, active, completed
	ShareToken  string    `json:"share_token"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Question struct {
	ID              int       `json:"id"`
	QuestionnaireID int       `json:"questionnaire_id"`
	QuestionText    string    `json:"question_text"`
	Category        string    `json:"category"` // BM atau SRM
	OrderNum        int       `json:"order_num"`
	CreatedAt       time.Time `json:"created_at"`
	QuestionType    string    `json:"question_type"` // text_short, text_long, radio, checkbox, file
	Options         string    `json:"options"`       // radio/checkbox
}

type ClientSession struct {
	ID              int        `json:"id"`
	QuestionnaireID int        `json:"questionnaire_id"`
	ClientName      string     `json:"client_name"`
	ClientEmail     string     `json:"client_email"`
	ClientCompany   string     `json:"client_company"`
	Token           string     `json:"token"`
	Status          string     `json:"status"` // pending, in_progress, completed
	SubmittedAt     *time.Time `json:"submitted_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Answer struct {
	ID         int       `json:"id"`
	SessionID  int       `json:"session_id"`
	QuestionID int       `json:"question_id"`
	AnswerText string    `json:"answer_text"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type GeneratedDocument struct {
	ID         int       `json:"id"`
	SessionID  int       `json:"session_id"`
	BMContent  string    `json:"bm_content"`
	SRMContent string    `json:"srm_content"`
	CreatedAt  time.Time `json:"created_at"`
}

type DashboardStats struct {
	TotalQuestionnaires int `json:"total_questionnaires"`
	TotalQuestions      int `json:"total_questions"`
	TotalResponses      int `json:"total_responses"`
	CompletedResponses  int `json:"completed_responses"`
}

type QuestionnaireStats struct {
	TotalQuestions    int `json:"total_questions"`
	AnsweredQuestions int `json:"answered_questions"`
	TotalResponses    int `json:"total_responses"`
}
