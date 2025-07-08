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
		var isPaid bool
		err := db.QueryRowx(
			`SELECT id, competition_id, is_paid FROM registered_competitions WHERE team_code = $1`,
			teamCode,
		).Scan(&teamID, &teamCompetitionID, &isPaid)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}
		if isPaid {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot join a team that has already paid"})
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

		// Fetch competition type
		var competitionType string
		err = db.Get(&competitionType, `SELECT competition_type FROM competitions WHERE id=$1`, teamCompetitionID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Competition not found"})
			return
		}

		// Fetch user info
		var binusian bool
		var nim, major, school *string
		err = db.QueryRowx(
			`SELECT binusian, nim, major, school FROM users WHERE email=$1`, userEmail,
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

		// Check max members for this competition
		var maxMembers *int
		err = db.Get(&maxMembers, `SELECT max_members FROM competitions WHERE id=$1`, teamCompetitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch max members"})
			return
		}
		if maxMembers != nil && *maxMembers > 0 {
			var currentMembers int
			err = db.Get(&currentMembers, `
                SELECT COUNT(*) FROM registered_competition_members rcm
                JOIN registered_competitions rc ON rc.id = rcm.registered_competition_id
                WHERE rc.competition_id = $1
                AND rcm.registered_competition_id = rc.id
            `, teamCompetitionID)
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
