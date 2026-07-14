package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/NotchG/BurnCup/models" // Update this if your module name is different
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Define the OAuth configuration globally
var googleOauthConfig = &oauth2.Config{
	ClientID:     "253415571924-8t5u5e2ai416ms9pirvfb4s3c6ocmure.apps.googleusercontent.com",     // Load from environment
	ClientSecret: "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", // Load from environment
	// Make sure this matches your Authorized Redirect URIs in Google Cloud Console
	RedirectURL:  "http://localhost:8080/api/auth/google/callback", 
	Scopes: []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	},
	Endpoint: google.Endpoint,
}

const oauthStateString = "random-secure-state-string"

// ==========================================
// 1. EMAIL / PASSWORD REGISTRATION
// ==========================================
func RegisterHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to secure password"})
			return
		}

		// Insert user into PostgreSQL and return the newly generated UUID
		var newUserID string
		query := `INSERT INTO users (email, password) VALUES ($1, $2) RETURNING id`
		err = db.QueryRow(query, req.Email, string(hashedPassword)).Scan(&newUserID)
		
		if err != nil {
			// If error occurs, it's highly likely a duplicate email constraint violation
			c.JSON(http.StatusConflict, gin.H{"error": "Email is already registered"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "User registered successfully",
			"id":      newUserID,
		})
	}
}

// ==========================================
// 2. EMAIL / PASSWORD LOGIN
// ==========================================
func LoginHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		// Only query the fields we absolutely need to avoid parsing NULL values on incomplete profiles
		var user struct {
			ID       string         `db:"id"`
			Email    string         `db:"email"`
			Password sql.NullString `db:"password"` // Using NullString in case they signed up via Google and have no password
		}

		query := `SELECT id, email, password FROM users WHERE email = $1`
		err := db.Get(&user, query, req.Email)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}

		// If the user has no password, they registered via Google OAuth
		if !user.Password.Valid || user.Password.String == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please sign in using Google"})
			return
		}

		// Compare hashes
		err = bcrypt.CompareHashAndPassword([]byte(user.Password.String), []byte(req.Password))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}

		// Generate the JWT
		token, err := generateJWT(user.ID, user.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Login successful",
			"token":   token,
		})
	}
}

// ==========================================
// 3. GOOGLE OAUTH REDIRECT
// ==========================================
func GoogleLoginHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		url := googleOauthConfig.AuthCodeURL(oauthStateString)
		c.Redirect(http.StatusTemporaryRedirect, url)
	}
}

// ==========================================
// 4. GOOGLE OAUTH CALLBACK (THE FUNNEL)
// ==========================================
func GoogleCallbackHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Query("state") != oauthStateString {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid OAuth state"})
			return
		}

		code := c.Query("code")
		token, err := googleOauthConfig.Exchange(context.Background(), code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token"})
			return
		}

		client := googleOauthConfig.Client(context.Background(), token)
		resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user info"})
			return
		}
		defer resp.Body.Close()

		var googleUser struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		json.NewDecoder(resp.Body).Decode(&googleUser)

		// Check if user exists in our DB
		var userID string
		queryCheck := `SELECT id FROM users WHERE email = $1`
		err = db.Get(&userID, queryCheck, googleUser.Email)

		// If user does not exist, insert them
		if err != nil {
			queryInsert := `INSERT INTO users (email, full_name) VALUES ($1, $2) RETURNING id`
			err = db.QueryRow(queryInsert, googleUser.Email, googleUser.Name).Scan(&userID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user account"})
				return
			}
		}

		// GENERATE THE JWT
		appToken, err := generateJWT(userID, googleUser.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session token"})
			return
		}

		// Redirect to frontend (Update this to your actual Next.js / React URL)
		frontendURL := "http://localhost:3000/auth-success?token=" + appToken
		c.Redirect(http.StatusTemporaryRedirect, frontendURL)
	}
}

// ==========================================
// HELPER: GENERATE JWT
// ==========================================
func generateJWT(userID string, email string) (string, error) {
	secret := os.Getenv("JWT_SECRET_KEY")
	if secret == "" {
		secret = "fallback-secret-key-change-me"
	}

	claims := jwt.MapClaims{
		"sub":   userID,                                // User's UUID
		"email": email,                                 // User's Email
		"exp":   time.Now().Add(time.Hour * 24 * 30).Unix(), 
		"iat":   time.Now().Unix(),                     // Issued at
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}