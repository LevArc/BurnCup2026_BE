package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
)

// CreateTeamRequest is the expected payload for creating a team
type CreateTeamRequest struct {
	CompetitionID string `json:"competitionId" binding:"required"`
	TeamName      string `json:"teamName" binding:"required"`
}

// generateTeamCode creates a random 8-character uppercase code
func generateTeamCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}

// generateUniqueTeamCode generates a unique team code by checking database
func generateUniqueTeamCode(db *sqlx.DB) (string, error) {
	maxAttempts := 10
	for i := 0; i < maxAttempts; i++ {
		teamCode, err := generateTeamCode()
		if err != nil {
			return "", err
		}

		// Check if team code already exists
		var exists bool
		err = db.QueryRowx(`
            SELECT EXISTS (
                SELECT 1 FROM registered_competitions 
                WHERE team_code = $1
            )
        `, teamCode).Scan(&exists)
		if err != nil {
			return "", err
		}

		if !exists {
			return teamCode, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique team code after %d attempts", maxAttempts)
}

// CreateTeamHandler creates a new team with the current user as team leader
func CreateTeamHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse user claims
		claims, ok := c.Get("user")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No user claims found"})
			return
		}
		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user claims"})
			return
		}
		userEmail, ok := mapClaims["email"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User email not found in token"})
			return
		}

		// Parse payload
		var req CreateTeamRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		// Fetch competition info
		var competitionType string
		var maxMembers *int
		var teamSlot int
		err := db.QueryRowx(
			`SELECT competition_type, max_members, team_slot FROM competitions WHERE id=$1`,
			req.CompetitionID,
		).Scan(&competitionType, &maxMembers, &teamSlot)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Competition not found"})
			return
		}

		// Check if team slots are full
		var currentTeams int
		err = db.Get(&currentTeams, `
            SELECT COUNT(*) FROM registered_competitions 
            WHERE competition_id = $1 AND is_paid = true
        `, req.CompetitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team slots"})
			return
		}

		if currentTeams >= teamSlot {
			c.JSON(http.StatusForbidden, gin.H{"error": "Competition team slots are full"})
			return
		}

		// Fetch user info
		var userType string
		err = db.QueryRowx(
			`SELECT user_type FROM users WHERE email=$1`, userEmail,
		).Scan(&userType)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User profile not found"})
			return
		}

		// Check requirements based on competition type
		switch competitionType {
			case "Binusian":
				if userType != "Binusian" {
					c.JSON(http.StatusForbidden, gin.H{"error": "Only Binusian users can join this competition"})
					return
				}
			case "SMA/SMK":
				if userType != "SMA/SMK" {
					c.JSON(http.StatusForbidden, gin.H{"error": "Only SMA/SMK users can join this competition"})
					return
				}
			case "SMA/SMK And Others (Non-Binusian)":
				if userType != "SMA/SMK" && userType != "Others" {
					c.JSON(http.StatusForbidden, gin.H{"error": "Only SMA/SMK and Other users (Non-Binusian) can join this competition"})
					return
				}
			case "Public":
				// No restriction for Public, but you can add additional checks if needed
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown competition type"})
				return
		}

		// Check if user has already joined this competition
		var existingTeamCount int
		err = db.Get(&existingTeamCount, `
            SELECT COUNT(*) 
            FROM registered_competition_members rcm
            JOIN registered_competitions rc ON rc.id = rcm.registered_competition_id
            WHERE rc.competition_id = $1 AND rcm.user_email = $2
        `, req.CompetitionID, userEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check existing participation"})
			return
		}

		if existingTeamCount > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "You have already joined this competition"})
			return
		}

		// Generate unique team code
		teamCode, err := generateUniqueTeamCode(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate unique team code"})
			return
		}

		// Insert into registered_competitions
		var teamID string
		err = db.QueryRowx(
			`INSERT INTO registered_competitions (team_name, team_code, is_paid, competition_id)
             VALUES ($1, $2, $3, $4) RETURNING id`,
			req.TeamName, teamCode, false, req.CompetitionID,
		).Scan(&teamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
			return
		}

		// Insert the user as team leader in registered_competition_members
		_, err = db.Exec(
			`INSERT INTO registered_competition_members (registered_competition_id, user_email, is_team_leader)
             VALUES ($1, $2, $3)`,
			teamID, userEmail, true,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add team leader"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"teamId":   teamID,
			"teamCode": teamCode,
		})
	}
}
