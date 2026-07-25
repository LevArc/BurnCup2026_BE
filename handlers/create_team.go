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
	maxAttempts := 50
	for i := 0; i < maxAttempts; i++ {
		teamCode, err := generateTeamCode()
		if err != nil {
			return "", err
		}
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

		// Check team name globally
		var nameExists bool
		err := db.QueryRowx(`
			SELECT EXISTS (
				SELECT 1 FROM registered_competitions
				WHERE LOWER(team_name) = LOWER($1)
			)
		`, req.TeamName).Scan(&nameExists)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate team name"})
			return
		}
		if nameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Team name is already taken"})
			return
		}

		// Fetch competition info
		var competitionType string
		var maxMembers *int
		err = db.QueryRowx(
			`SELECT competition_type, max_members FROM competitions WHERE id=$1`,
			req.CompetitionID,
		).Scan(&competitionType, &maxMembers)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Competition not found"})
			return
		}

		// Fetch user info
		var userID string
		var userType string
		err = db.QueryRowx(
			`SELECT id, COALESCE(user_type, '') FROM users WHERE email=$1`, userEmail,
		).Scan(&userID, &userType)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User profile not found"})
			return
		}

		// Determine slot column based on leader user type
		slotColumn := "non_binusian_team_slot"
		if strings.EqualFold(userType, "Binusian") {
			slotColumn = "binusian_team_slot"
		}

		// Fetch slot limit from competition_slots
		var totalSlot int
		err = db.QueryRowx(
			fmt.Sprintf(`SELECT %s FROM competition_slots WHERE competition_id = $1`, slotColumn),
			req.CompetitionID,
		).Scan(&totalSlot)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Slot info not found for this competition"})
			return
		}

		// Count paid teams in same category (by leader user type)
		categoryFilter := "LOWER(u.user_type) != 'binusian'"
		if strings.EqualFold(userType, "Binusian") {
			categoryFilter = "LOWER(u.user_type) = 'binusian'"
		}
		var currentTeams int
		err = db.Get(&currentTeams, fmt.Sprintf(`
			SELECT COUNT(*)
			FROM registered_competitions rc
			JOIN registered_competition_members rcm ON rc.id = rcm.registered_competition_id
			JOIN users u ON u.id = rcm.user_id
			WHERE rc.competition_id = $1
			  AND rc.is_paid = true
			  AND rcm.is_team_leader = true
			  AND %s
		`, categoryFilter), req.CompetitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team slots"})
			return
		}
		if currentTeams >= totalSlot {
			c.JSON(http.StatusForbidden, gin.H{"error": "Competition team slots are full for your category"})
			return
		}

		// Check requirements based on competition type
		switch competitionType {
		case "Binusian":
			if !strings.EqualFold(userType, "Binusian") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only Binusian users can join this competition"})
				return
			}
		case "Binusian And SMA/SMK":
			if !strings.EqualFold(userType, "SMA/SMK") && !strings.EqualFold(userType, "Binusian") {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only SMA/SMK and Binusian users can join this competition"})
				return
			}
		case "Public":
			// No restriction
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown competition type"})
			return
		}

		// Check if user already joined this competition
		var existingTeamCount int
		err = db.Get(&existingTeamCount, `
			SELECT COUNT(*)
			FROM registered_competition_members rcm
			JOIN registered_competitions rc ON rc.id = rcm.registered_competition_id
			WHERE rc.competition_id = $1 AND rcm.user_id = $2
		`, req.CompetitionID, userID)
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
			fmt.Println("DB ERROR (create team):", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
			return
		}

		// Insert the user as team leader
		_, err = db.Exec(
			`INSERT INTO registered_competition_members (registered_competition_id, user_id, is_team_leader)
			 VALUES ($1, $2, $3)`,
			teamID, userID, true,
		)
		if err != nil {
			fmt.Println("DB ERROR (add team leader):", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add team leader"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"teamId":   teamID,
			"teamCode": teamCode,
		})
	}
}
