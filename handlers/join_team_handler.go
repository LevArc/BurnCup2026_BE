package handlers

import (
	"database/sql"
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

		// Query the database to get the user ID using the email
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

		// Check if user is already a member of this team
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

		// CHANGED: Fetch user info including major and school
		var userType, major, school *string
		err = db.QueryRowx(
			`SELECT user_type, major, school FROM users WHERE id=$1`, userID,
		).Scan(&userType, &major, &school)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User profile not found"})
			return
		}

		safeUserType := ""
		safeMajor := ""
		safeSchool := ""
		if userType != nil {
			safeUserType = *userType
		}
		if major != nil {
			safeMajor = *major
		}
		if school != nil {
			safeSchool = *school
		}

		// Check requirements based on competition type
		switch competitionType {
		case "Binusian":
			if safeUserType != "Binusian" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only Binusian users can join this competition"})
				return
			}
		case "Binusian And SMA/SMK":
			if safeUserType != "SMA/SMK" && safeUserType != "Binusian" {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only SMA/SMK and Binusian user can join this competition"})
				return
			}
		case "Public":
			// No restriction for Public on competition level
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown competition type"})
			return
		}

		// NEW VALIDATION: Check against Team Leader attributes to ensure team consistency
		var leaderType, leaderMajor, leaderSchool *string
		err = db.QueryRowx(`
			SELECT u.user_type, u.major, u.school 
			FROM users u
			JOIN registered_competition_members rcm ON u.id = rcm.user_id
			WHERE rcm.registered_competition_id = $1 AND rcm.is_team_leader = true
		`, teamID).Scan(&leaderType, &leaderMajor, &leaderSchool)

		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid team: no team leader found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch team requirements"})
			}
			return
		}

		safeLeaderType := ""
		safeLeaderMajor := ""
		safeLeaderSchool := ""
		if leaderType != nil {
			safeLeaderType = *leaderType
		}
		if leaderMajor != nil {
			safeLeaderMajor = *leaderMajor
		}
		if leaderSchool != nil {
			safeLeaderSchool = *leaderSchool
		}

		// Validate Team Compatibility
		if !strings.EqualFold(safeUserType, safeLeaderType) {
			c.JSON(http.StatusForbidden, gin.H{"error": "You must have the same user type as the rest of the team"})
			return
		}

		if strings.EqualFold(safeUserType, "Binusian") {
			if !strings.EqualFold(safeMajor, safeLeaderMajor) {
				c.JSON(http.StatusForbidden, gin.H{"error": "All Binusian team members must have the same major"})
				return
			}
		} else if strings.EqualFold(safeUserType, "SMA/SMK") {
			if !strings.EqualFold(safeSchool, safeLeaderSchool) {
				c.JSON(http.StatusForbidden, gin.H{"error": "All SMA/SMK team members must be from the same school"})
				return
			}
		}
		// If safeUserType is "Public", no further cross-checks needed

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