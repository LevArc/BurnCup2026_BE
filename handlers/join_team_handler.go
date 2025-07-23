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
		userEmail, ok := mapClaims["email"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User email not found in token"})
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
		err := db.QueryRowx(
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

		// Check if user is already a member of this team
		var exists bool
		err = db.QueryRowx(
			`SELECT EXISTS (
				SELECT 1 FROM registered_competition_members
				WHERE registered_competition_id = $1 AND user_email = $2
			)`, teamID, userEmail,
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

		// Add user as a member (not team leader)
		_, err = db.Exec(
			`INSERT INTO registered_competition_members (registered_competition_id, user_email, is_team_leader)
             VALUES ($1, $2, $3)`,
			teamID, userEmail, false,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join team"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Successfully joined the team"})
	}
}
