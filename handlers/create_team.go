package handlers

import (
	"crypto/rand"
	"encoding/hex"
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
		userID, ok := mapClaims["sub"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in token"})
			return
		}

		// Parse payload
		var req CreateTeamRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		// Fetch competition type
		var competitionType string
		err := db.Get(&competitionType, `SELECT competition_type FROM competitions WHERE id=$1`, req.CompetitionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Competition not found"})
			return
		}

		// Fetch user info
		var binusian bool
		var nim, major, school *string
		err = db.QueryRowx(
			`SELECT binusian, nim, major, school FROM users WHERE id=$1`, userID,
		).Scan(&binusian, &nim, &major, &school)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User profile not found"})
			return
		}

		// Check requirements based on competition type
		switch competitionType {
		case "Binusian":
			if !binusian || nim == nil || major == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only Binusian users with NIM and major can join this competition"})
				return
			}
		case "NonBinusian":
			if binusian || school == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only Non-Binusian users with school can join this competition"})
				return
			}
		case "Mixed":
			// No restriction for Mixed, but you can add additional checks if needed
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown competition type"})
			return
		}

		// Generate random team code
		teamCode, err := generateTeamCode()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate team code"})
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
			`INSERT INTO registered_competition_members (registered_competition_id, user_id, is_team_leader)
             VALUES ($1, $2, $3)`,
			teamID, userID, true,
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
