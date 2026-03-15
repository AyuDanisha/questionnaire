package database

import (
	"database/sql"
	"fmt"
	"log"

	"questionnaire-app/config"
	"questionnaire-app/internal/models"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func ConnectDB() {
	cfg := config.AppConfig
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode,
	)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	log.Println("Connected to PostgreSQL database")

	RunMigrations()
}

func RunMigrations() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS developers (
            id SERIAL PRIMARY KEY,
            email VARCHAR(255) UNIQUE NOT NULL,
            password_hash VARCHAR(255) NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS questionnaires (
            id SERIAL PRIMARY KEY,
            developer_id INTEGER REFERENCES developers(id),
            title VARCHAR(255) NOT NULL,
            description TEXT,
            project_desc TEXT,
            status VARCHAR(50) DEFAULT 'draft',
            share_token VARCHAR(100) UNIQUE,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS questions (
            id SERIAL PRIMARY KEY,
            questionnaire_id INTEGER REFERENCES questionnaires(id) ON DELETE CASCADE,
            question_text TEXT NOT NULL,
            category VARCHAR(50),
            order_num INTEGER,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS client_sessions (
            id SERIAL PRIMARY KEY,
            questionnaire_id INTEGER REFERENCES questionnaires(id),
            client_name VARCHAR(255),
            client_email VARCHAR(255),
            client_company VARCHAR(255),
            token VARCHAR(100) UNIQUE NOT NULL,
            status VARCHAR(50) DEFAULT 'pending',
            submitted_at TIMESTAMP,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE TABLE IF NOT EXISTS answers (
            id SERIAL PRIMARY KEY,
            session_id INTEGER REFERENCES client_sessions(id) ON DELETE CASCADE,
            question_id INTEGER REFERENCES questions(id),
            answer_text TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            UNIQUE(session_id, question_id)
        )`,
		`CREATE TABLE IF NOT EXISTS generated_documents (
            id SERIAL PRIMARY KEY,
            session_id INTEGER REFERENCES client_sessions(id) ON DELETE CASCADE,
            bm_content TEXT,
            srm_content TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
		`CREATE INDEX IF NOT EXISTS idx_questions_questionnaire ON questions(questionnaire_id)`,
		`CREATE INDEX IF NOT EXISTS idx_answers_session ON answers(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_client_sessions_token ON client_sessions(token)`,
		`ALTER TABLE questions ADD COLUMN IF NOT EXISTS question_type VARCHAR(50) DEFAULT 'text_long'`,
		`ALTER TABLE questions ADD COLUMN IF NOT EXISTS options TEXT`,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			log.Printf("Migration error: %v", err)
		}
	}
	log.Println("Database migrations completed")
}

// Developer operations
func CreateDeveloper(email, passwordHash string) (int, error) {
	var id int
	err := DB.QueryRow(
		"INSERT INTO developers (email, password_hash) VALUES ($1, $2) RETURNING id",
		email, passwordHash,
	).Scan(&id)
	return id, err
}

func GetClientSessionByID(id int) (*models.ClientSession, error) {
	s := &models.ClientSession{}
	err := DB.QueryRow(
		`SELECT id, questionnaire_id, client_name, client_email, client_company, token, status, submitted_at, created_at 
         FROM client_sessions WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.QuestionnaireID, &s.ClientName, &s.ClientEmail, &s.ClientCompany, &s.Token, &s.Status, &s.SubmittedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func GetDeveloperByEmail(email string) (*models.Developer, error) {
	d := &models.Developer{}
	err := DB.QueryRow(
		"SELECT id, email, password_hash, created_at FROM developers WHERE email = $1",
		email,
	).Scan(&d.ID, &d.Email, &d.PasswordHash, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func GetDeveloperByID(id int) (*models.Developer, error) {
	d := &models.Developer{}
	err := DB.QueryRow(
		"SELECT id, email, password_hash, created_at FROM developers WHERE id = $1",
		id,
	).Scan(&d.ID, &d.Email, &d.PasswordHash, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Questionnaire operations
func CreateQuestionnaire(devID int, title, desc, projectDesc, shareToken string) (int, error) {
	var id int
	err := DB.QueryRow(
		`INSERT INTO questionnaires (developer_id, title, description, project_desc, status, share_token) 
         VALUES ($1, $2, $3, $4, 'active', $5) RETURNING id`,
		devID, title, desc, projectDesc, shareToken,
	).Scan(&id)
	return id, err
}

func GetQuestionnairesByDeveloper(devID int) ([]models.Questionnaire, error) {
	rows, err := DB.Query(
		`SELECT id, developer_id, title, description, project_desc, status, share_token, created_at, updated_at 
         FROM questionnaires WHERE developer_id = $1 ORDER BY created_at DESC`,
		devID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questionnaires []models.Questionnaire
	for rows.Next() {
		var q models.Questionnaire
		err := rows.Scan(&q.ID, &q.DeveloperID, &q.Title, &q.Description, &q.ProjectDesc, &q.Status, &q.ShareToken, &q.CreatedAt, &q.UpdatedAt)
		if err != nil {
			return nil, err
		}
		questionnaires = append(questionnaires, q)
	}
	return questionnaires, nil
}

func GetQuestionnaireByToken(token string) (*models.Questionnaire, error) {
	q := &models.Questionnaire{}
	err := DB.QueryRow(
		`SELECT id, developer_id, title, description, project_desc, status, share_token, created_at, updated_at 
         FROM questionnaires WHERE share_token = $1`,
		token,
	).Scan(&q.ID, &q.DeveloperID, &q.Title, &q.Description, &q.ProjectDesc, &q.Status, &q.ShareToken, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return q, nil
}

func GetQuestionnaireByID(id int) (*models.Questionnaire, error) {
	q := &models.Questionnaire{}
	err := DB.QueryRow(
		`SELECT id, developer_id, title, description, project_desc, status, share_token, created_at, updated_at 
         FROM questionnaires WHERE id = $1`,
		id,
	).Scan(&q.ID, &q.DeveloperID, &q.Title, &q.Description, &q.ProjectDesc, &q.Status, &q.ShareToken, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return q, nil
}

// Question operations
func CreateQuestions(questions []models.Question) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		`INSERT INTO questions (questionnaire_id, question_text, category, order_num) VALUES ($1, $2, $3, $4)`,
	)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, q := range questions {
		_, err := stmt.Exec(q.QuestionnaireID, q.QuestionText, q.Category, q.OrderNum)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}
func GetQuestionsByQuestionnaire(qID int) ([]models.Question, error) {
	rows, err := DB.Query(
		`SELECT id, questionnaire_id, question_text, category, order_num, created_at, question_type, options 
         FROM questions WHERE questionnaire_id = $1 ORDER BY order_num`,
		qID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []models.Question
	for rows.Next() {
		var q models.Question
		// Handle NULL untuk options
		var opts sql.NullString

		err := rows.Scan(&q.ID, &q.QuestionnaireID, &q.QuestionText, &q.Category, &q.OrderNum, &q.CreatedAt, &q.QuestionType, &opts)
		if err != nil {
			return nil, err
		}

		if opts.Valid {
			q.Options = opts.String
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func CreateClientSession(qID int, name, email, company, token string) (int, error) {
	var id int
	err := DB.QueryRow(
		`INSERT INTO client_sessions (questionnaire_id, client_name, client_email, client_company, token, status) 
         VALUES ($1, $2, $3, $4, $5, 'pending') RETURNING id`,
		qID, name, email, company, token,
	).Scan(&id)
	return id, err
}

func GetClientSessionByToken(token string) (*models.ClientSession, error) {
	s := &models.ClientSession{}
	err := DB.QueryRow(
		`SELECT id, questionnaire_id, client_name, client_email, client_company, token, status, submitted_at, created_at 
         FROM client_sessions WHERE token = $1`,
		token,
	).Scan(&s.ID, &s.QuestionnaireID, &s.ClientName, &s.ClientEmail, &s.ClientCompany, &s.Token, &s.Status, &s.SubmittedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func UpdateClientSessionStatus(id int, status string) error {
	var submittedAt interface{}
	if status == "completed" {
		_, err := DB.Exec(
			`UPDATE client_sessions SET status = $1, submitted_at = CURRENT_TIMESTAMP WHERE id = $2`,
			status, id,
		)
		return err
	}
	_, err := DB.Exec(
		`UPDATE client_sessions SET status = $1 WHERE id = $2`,
		status, submittedAt, id,
	)
	return err
}

func GetClientSessionsByQuestionnaire(qID int) ([]models.ClientSession, error) {
	rows, err := DB.Query(
		`SELECT id, questionnaire_id, client_name, client_email, client_company, token, status, submitted_at, created_at 
         FROM client_sessions WHERE questionnaire_id = $1 ORDER BY created_at DESC`,
		qID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.ClientSession
	for rows.Next() {
		var s models.ClientSession
		err := rows.Scan(&s.ID, &s.QuestionnaireID, &s.ClientName, &s.ClientEmail, &s.ClientCompany, &s.Token, &s.Status, &s.SubmittedAt, &s.CreatedAt)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// Answer operations
func SaveAnswer(sessionID, questionID int, answerText string) error {
	_, err := DB.Exec(
		`INSERT INTO answers (session_id, question_id, answer_text) 
         VALUES ($1, $2, $3) 
         ON CONFLICT (session_id, question_id) 
         DO UPDATE SET answer_text = $3, updated_at = CURRENT_TIMESTAMP`,
		sessionID, questionID, answerText,
	)
	return err
}

func GetAnswersBySession(sessionID int) ([]models.Answer, error) {
	rows, err := DB.Query(
		`SELECT id, session_id, question_id, answer_text, created_at, updated_at 
         FROM answers WHERE session_id = $1`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []models.Answer
	for rows.Next() {
		var a models.Answer
		err := rows.Scan(&a.ID, &a.SessionID, &a.QuestionID, &a.AnswerText, &a.CreatedAt, &a.UpdatedAt)
		if err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, nil
}

func GetAnswerCountBySession(sessionID int) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM answers WHERE session_id = $1 AND answer_text IS NOT NULL AND answer_text != ''`,
		sessionID,
	).Scan(&count)
	return count, err
}

// Generated Document operations
func SaveGeneratedDocument(sessionID int, bmContent, srmContent string) (int, error) {
	var id int
	err := DB.QueryRow(
		`INSERT INTO generated_documents (session_id, bm_content, srm_content) 
         VALUES ($1, $2, $3) RETURNING id`,
		sessionID, bmContent, srmContent,
	).Scan(&id)
	return id, err
}

func GetGeneratedDocument(sessionID int) (*models.GeneratedDocument, error) {
	doc := &models.GeneratedDocument{}
	err := DB.QueryRow(
		`SELECT id, session_id, bm_content, srm_content, created_at 
         FROM generated_documents WHERE session_id = $1`,
		sessionID,
	).Scan(&doc.ID, &doc.SessionID, &doc.BMContent, &doc.SRMContent, &doc.CreatedAt)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// Dashboard stats
func GetDashboardStats(devID int) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	err := DB.QueryRow(
		`SELECT COUNT(*) FROM questionnaires WHERE developer_id = $1`,
		devID,
	).Scan(&stats.TotalQuestionnaires)
	if err != nil {
		return nil, err
	}

	err = DB.QueryRow(
		`SELECT COUNT(*) FROM questions q 
         JOIN questionnaires qn ON q.questionnaire_id = qn.id 
         WHERE qn.developer_id = $1`,
		devID,
	).Scan(&stats.TotalQuestions)
	if err != nil {
		return nil, err
	}

	err = DB.QueryRow(
		`SELECT COUNT(*) FROM client_sessions cs
         JOIN questionnaires qn ON cs.questionnaire_id = qn.id 
         WHERE qn.developer_id = $1`,
		devID,
	).Scan(&stats.TotalResponses)
	if err != nil {
		return nil, err
	}

	err = DB.QueryRow(
		`SELECT COUNT(*) FROM client_sessions cs
         JOIN questionnaires qn ON cs.questionnaire_id = qn.id 
         WHERE qn.developer_id = $1 AND cs.status = 'completed'`,
		devID,
	).Scan(&stats.CompletedResponses)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func GetQuestionnaireStats(qID int) (*models.QuestionnaireStats, error) {
	stats := &models.QuestionnaireStats{}

	err := DB.QueryRow(
		`SELECT COUNT(*) FROM questions WHERE questionnaire_id = $1`,
		qID,
	).Scan(&stats.TotalQuestions)
	if err != nil {
		return nil, err
	}

	err = DB.QueryRow(
		`SELECT COUNT(*) FROM client_sessions WHERE questionnaire_id = $1`,
		qID,
	).Scan(&stats.TotalResponses)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

func GetAnsweredCountForQuestion(qID int) (int, error) {
	var count int
	err := DB.QueryRow(
		`SELECT COUNT(*) FROM answers a
         JOIN questions q ON a.question_id = q.id
         WHERE q.id = $1 AND a.answer_text IS NOT NULL AND a.answer_text != ''`,
		qID,
	).Scan(&count)
	return count, err
}

func CreateSingleQuestion(q models.Question) (int, error) {
	var id int
	err := DB.QueryRow(
		`INSERT INTO questions (questionnaire_id, question_text, category, order_num, question_type, options) 
         VALUES ($1, $2, $3, 
                 (SELECT COALESCE(MAX(order_num), 0) + 1 FROM questions WHERE questionnaire_id = $1),
                 $4, $5
         ) 
         RETURNING id`,
		q.QuestionnaireID, q.QuestionText, q.Category, q.QuestionType, q.Options,
	).Scan(&id)
	return id, err
}
func DeleteQuestion(qID int) error {
	_, err := DB.Exec("DELETE FROM questions WHERE id = $1", qID)
	return err
}

func UpdateQuestion(qID int, text string, category string, qType string, options string) error {
	_, err := DB.Exec(
		"UPDATE questions SET question_text = $1, category = $2, question_type = $3, options = $4 WHERE id = $5",
		text, category, qType, options, qID,
	)
	return err
}

func UpdateQuestionnaireStatus(qID int, status string) error {
	_, err := DB.Exec(
		"UPDATE questionnaires SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
		status, qID,
	)
	return err
}

func GetQuestionByID(qID int) (*models.Question, error) {
	q := &models.Question{}
	err := DB.QueryRow(
		"SELECT id, questionnaire_id, question_text, category, order_num, created_at FROM questions WHERE id = $1",
		qID,
	).Scan(&q.ID, &q.QuestionnaireID, &q.QuestionText, &q.Category, &q.OrderNum, &q.CreatedAt)
	if err != nil {
		return nil, err
	}
	return q, nil
}
