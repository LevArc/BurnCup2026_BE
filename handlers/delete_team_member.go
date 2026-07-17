	package handlers

	import (
		"database/sql"
		"log"
		"net/http"

		"github.com/gin-gonic/gin"
		"github.com/golang-jwt/jwt/v5"
		"github.com/jmoiron/sqlx"
	)

	// DeleteTeamMemberRequest represents the request body for deleting a team member
	type DeleteTeamMemberRequest struct {
		TeamID      string `json:"teamId" binding:"required"`
		MemberEmail string `json:"memberEmail" binding:"required"`
	}

	// DeleteTeamMemberHandler allows a team leader to remove a team member
	func DeleteTeamMemberHandler(db *sqlx.DB) gin.HandlerFunc {
		return func(c *gin.Context) {
			// Parse user claims from JWT
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

			// Fetch the current user's ID from the database using their email
			var currentUserID string
			err := db.QueryRowx(`SELECT id FROM users WHERE email = $1`, userEmail).Scan(&currentUserID)
			if err != nil {
				log.Printf("Error fetching current user ID for email %s: %v", userEmail, err)
				c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in database", "details": err.Error()})
				return
			}

			// Parse request body
			var req DeleteTeamMemberRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				log.Printf("Error binding JSON payload: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
				return
			}

			// Fetch the member's ID using the email provided in the request body
			var targetMemberID string
			err = db.QueryRowx(`SELECT id FROM users WHERE email = $1`, req.MemberEmail).Scan(&targetMemberID)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "User with the specified email does not exist"})
				} else {
					log.Printf("Error fetching target member ID for email %s: %v", req.MemberEmail, err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to look up target member", "details": err.Error()})
				}
				return
			}

			// Check if the team exists
			var teamExists bool
			err = db.Get(&teamExists, "SELECT EXISTS(SELECT 1 FROM registered_competitions WHERE id = $1)", req.TeamID)
			if err != nil {
				log.Printf("Error checking team existence for TeamID %s: %v", req.TeamID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team existence", "details": err.Error()})
				return
			}
			if !teamExists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
				return
			}

			// Check if the current user is the team leader
			var isTeamLeader bool
			err = db.Get(&isTeamLeader, `
				SELECT EXISTS(
					SELECT 1 FROM registered_competition_members 
					WHERE registered_competition_id = $1 
					AND user_id = $2 
					AND is_team_leader = true
				)`, req.TeamID, currentUserID)
			if err != nil {
				log.Printf("Error checking team leadership for TeamID %s, UserID %s: %v", req.TeamID, currentUserID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team leadership", "details": err.Error()})
				return
			}
			if !isTeamLeader {
				c.JSON(http.StatusForbidden, gin.H{"error": "Only team leaders can remove team members"})
				return
			}

			// Check if the team has already paid
			var isPaid bool
			err = db.Get(&isPaid, "SELECT is_paid FROM registered_competitions WHERE id = $1", req.TeamID)
			if err != nil {
				log.Printf("Error checking payment status for TeamID %s: %v", req.TeamID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check payment status", "details": err.Error()})
				return
			}
			if isPaid {
				c.JSON(http.StatusForbidden, gin.H{"error": "Cannot remove members from a team that has already paid"})
				return
			}

			// Check if the member to be deleted exists in the team
			var memberExists bool
			err = db.Get(&memberExists, `
				SELECT EXISTS(
					SELECT 1 FROM registered_competition_members 
					WHERE registered_competition_id = $1 
					AND user_id = $2
				)`, req.TeamID, targetMemberID)
			if err != nil {
				log.Printf("Error checking member existence for TeamID %s, TargetMemberID %s: %v", req.TeamID, targetMemberID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check member existence", "details": err.Error()})
				return
			}
			if !memberExists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Member not found in this team"})
				return
			}

			// Check if trying to remove the team leader themselves
			var isMemberTeamLeader bool
			err = db.Get(&isMemberTeamLeader, `
				SELECT is_team_leader FROM registered_competition_members 
				WHERE registered_competition_id = $1 
				AND user_id = $2`, req.TeamID, targetMemberID)
			if err != nil {
				log.Printf("Error checking member role for TeamID %s, TargetMemberID %s: %v", req.TeamID, targetMemberID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check member role", "details": err.Error()})
				return
			}
			if isMemberTeamLeader {
				c.JSON(http.StatusForbidden, gin.H{"error": "Team leader cannot remove themselves from the team"})
				return
			}

			// Check minimum member requirements
			var competitionID string
			var minMembers *int
			err = db.QueryRowx(`
				SELECT c.id, c.min_members 
				FROM competitions c 
				JOIN registered_competitions rc ON c.id = rc.competition_id 
				WHERE rc.id = $1`, req.TeamID).Scan(&competitionID, &minMembers)
			if err != nil {
				log.Printf("Error fetching competition details for TeamID %s: %v", req.TeamID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch competition details", "details": err.Error()})
				return
			}

			// Check current member count
			var currentMemberCount int
			err = db.Get(&currentMemberCount, `
				SELECT COUNT(*) FROM registered_competition_members 
				WHERE registered_competition_id = $1`, req.TeamID)
			if err != nil {
				log.Printf("Error counting current members for TeamID %s: %v", req.TeamID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count current members", "details": err.Error()})
				return
			}

			// Start transaction
			tx, err := db.Beginx()
			if err != nil {
				log.Printf("Error starting database transaction: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction", "details": err.Error()})
				return
			}
			defer tx.Rollback()

			// Delete the team member using targetMemberID
			result, err := tx.Exec(`
				DELETE FROM registered_competition_members 
				WHERE registered_competition_id = $1 
				AND user_id = $2 
				AND is_team_leader = false`, req.TeamID, targetMemberID)
			if err != nil {
				log.Printf("Error executing DELETE query for TeamID %s, TargetMemberID %s: %v", req.TeamID, targetMemberID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove team member", "details": err.Error()})
				return
			}

			// Check if any rows were affected
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				log.Printf("Error checking RowsAffected after DELETE: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify member removal", "details": err.Error()})
				return
			}

			if rowsAffected == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "Member not found or is a team leader"})
				return
			}

			// Update the team's updated_at timestamp
			_, err = tx.Exec(`
				UPDATE registered_competitions 
				SET updated_at = NOW() 
				WHERE id = $1`, req.TeamID)
			if err != nil {
				log.Printf("Error updating team timestamp for TeamID %s: %v", req.TeamID, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team timestamp", "details": err.Error()})
				return
			}

			// Commit transaction
			if err = tx.Commit(); err != nil {
				log.Printf("Error committing transaction: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit member removal", "details": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":       "Team member removed successfully",
				"teamId":        req.TeamID,
				"removedMember": req.MemberEmail,
			})
		}
	}