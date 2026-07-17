package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
)

// JoinTeamRequest is the expected payload for joining a team
type JoinTeamRequest struct {
	TeamCode      string `json:"teamCode" binding:"required"`
	CompetitionID string `json:"competitionId" binding:"required"`
}

// JoinTeamHandler allows a user to join a team by code with eligibility checks
func JoinTeamHandler(db *sqlx.DB) gin.HandlerFunc {
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

		// Extract email from token
		userEmail, ok := mapClaims["email"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User email not found in token"})
			return
		}

		// ADDED: Query the database to get the user ID using the email
		var userID string
		err := db.QueryRowx(`SELECT id FROM users WHERE email = $1`, userEmail).Scan(&userID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in database"})
			return
		}

		// Parse payload
		var req JoinTeamRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}
		teamCode := strings.ToUpper(req.TeamCode)

		// Find the team by code and check competition
		var teamID, teamCompetitionID string
		err = db.QueryRowx(
			`SELECT id, competition_id FROM registered_competitions WHERE team_code = $1`,
			teamCode,
		).Scan(&teamID, &teamCompetitionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}
		if teamCompetitionID != req.CompetitionID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Team does not belong to the specified competition"})
			return
		}

		// Check if user is already a member of this team (using userID)
		var exists bool
		err = db.QueryRowx(
			`SELECT EXISTS (
				SELECT 1 FROM registered_competition_members
				WHERE registered_competition_id = $1 AND user_id = $2
			)`, teamID, userID,
		).Scan(&exists)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check membership"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "You are already a member of this team"})
			return
		}

		// Fetch competition info including team slot
		var competitionType string
		var maxMembers *int
		var teamSlot int
		err = db.QueryRowx(
			`SELECT competition_type, max_members, team_slot FROM competitions WHERE id=$1`,
			teamCompetitionID,
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
		`, teamCompetitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team slots"})
			return
		}

		if currentTeams >= teamSlot {
			c.JSON(http.StatusForbidden, gin.H{"error": "Competition team slots are full"})
			return
		}

		// Fetch user info (using userID)
		// Note: user_type is a pointer in your struct, so it might be null.
		// We scan into a string pointer to avoid sql.ErrNoRows if it's null.
		var userType *string
		err = db.QueryRowx(
			`SELECT user_type FROM users WHERE id=$1`, userID,
		).Scan(&userType)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User profile not found"})
			return
		}

		// Safely dereference userType for the switch statement
		safeUserType := ""
		if userType != nil {
			safeUserType = *userType
		}

		// Check requirements based on competition type
		switch competitionType {
		case "Binusian":
			if safeUserType != "Binusian" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only Binusian users can join this competition"})
				return
			}
		case "SMA/SMK":
			if safeUserType != "SMA/SMK" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only SMA/SMK users can join this competition"})
				return
			}
		case "SMA/SMK And Others (Non-Binusian)":
			if safeUserType != "SMA/SMK" && safeUserType != "Others" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only SMA/SMK and Other users (Non-Binusian) can join this competition"})
				return
			}
		case "Public":
			// No restriction for Public
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown competition type"})
			return
		}

		// Check max members for this specific team
		if maxMembers != nil && *maxMembers > 0 {
			var currentMembers int
			err = db.Get(&currentMembers, `
				SELECT COUNT(*) FROM registered_competition_members
				WHERE registered_competition_id = $1
			`, teamID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count current team members"})
				return
			}
			if currentMembers >= *maxMembers {
				c.JSON(http.StatusForbidden, gin.H{"error": "Maximum number of team members reached"})
				return
			}
		}

		// Add user as a member (using userID)
		_, err = db.Exec(
			`INSERT INTO registered_competition_members (registered_competition_id, user_id, is_team_leader)
			 VALUES ($1, $2, $3)`,
			teamID, userID, false,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join team"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Successfully joined the team"})
	}
}
