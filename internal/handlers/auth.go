package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"questionnaire-app/internal/database"
	"questionnaire-app/internal/models"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userKey contextKey = "user"

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		RenderTemplate(w, "register.html", nil)
		return
	}

	if r.Method == "POST" {
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if email == "" || password == "" {
			RenderTemplate(w, "register.html", map[string]interface{}{
				"Error": "Email dan password harus diisi",
			})
			return
		}

		if password != confirmPassword {
			RenderTemplate(w, "register.html", map[string]interface{}{
				"Error": "Password tidak cocok",
			})
			return
		}

		if len(password) < 6 {
			RenderTemplate(w, "register.html", map[string]interface{}{
				"Error": "Password minimal 6 karakter",
			})
			return
		}

		// Check if email already exists
		_, err := database.GetDeveloperByEmail(email)
		if err == nil {
			RenderTemplate(w, "register.html", map[string]interface{}{
				"Error": "Email sudah terdaftar",
			})
			return
		}

		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			RenderTemplate(w, "register.html", map[string]interface{}{
				"Error": "Gagal memproses password",
			})
			return
		}

		// Create developer
		_, err = database.CreateDeveloper(email, string(hashedPassword))
		if err != nil {
			RenderTemplate(w, "register.html", map[string]interface{}{
				"Error": "Gagal membuat akun: " + err.Error(),
			})
			return
		}

		http.Redirect(w, r, "/login?registered=true", http.StatusSeeOther)
	}
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		registered := r.URL.Query().Get("registered") == "true"
		RenderTemplate(w, "login.html", map[string]interface{}{
			"Registered": registered,
		})
		return
	}

	if r.Method == "POST" {
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")

		if email == "" || password == "" {
			RenderTemplate(w, "login.html", map[string]interface{}{
				"Error": "Email dan password harus diisi",
			})
			return
		}

		dev, err := database.GetDeveloperByEmail(email)
		if err != nil {
			RenderTemplate(w, "login.html", map[string]interface{}{
				"Error": "Email atau password salah",
			})
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(dev.PasswordHash), []byte(password))
		if err != nil {
			RenderTemplate(w, "login.html", map[string]interface{}{
				"Error": "Email atau password salah",
			})
			return
		}

		// Create session token
		token := generateToken()

		// Set cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Path:     "/",
			MaxAge:   86400 * 7, // 7 days
			HttpOnly: true,
			Secure:   false, // Set to true in production with HTTPS
		})

		// Store session in memory (in production, use Redis or database)
		sessions[token] = dev.ID

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		delete(sessions, cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		devID, exists := sessions[cookie.Value]
		if !exists {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		dev, err := database.GetDeveloperByID(devID)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), userKey, dev)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func GetUserFromContext(r *http.Request) *models.Developer {
	user, _ := r.Context().Value(userKey).(*models.Developer)
	return user
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Simple in-memory session store (use Redis in production)
var sessions = make(map[string]int)

func init() {
	// Clean up expired sessions periodically
	go func() {
		for {
			time.Sleep(time.Hour)
			// In production, implement proper session expiration
		}
	}()
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		if _, exists := sessions[cookie.Value]; exists {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	RenderTemplate(w, "index.html", nil)
}
