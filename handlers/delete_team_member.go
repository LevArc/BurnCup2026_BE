package handlers

import (
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

		// Parse request body
		var req DeleteTeamMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		// Check if the team exists
		var teamExists bool
		err := db.Get(&teamExists, "SELECT EXISTS(SELECT 1 FROM registered_competitions WHERE id = $1)", req.TeamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team existence"})
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
                AND user_email = $2 
                AND is_team_leader = true
            )`, req.TeamID, userEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check team leadership"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check payment status"})
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
                AND user_email = $2
            )`, req.TeamID, req.MemberEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check member existence"})
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
            AND user_email = $2`, req.TeamID, req.MemberEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check member role"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch competition details"})
			return
		}

		// Check current member count
		var currentMemberCount int
		err = db.Get(&currentMemberCount, `
            SELECT COUNT(*) FROM registered_competition_members 
            WHERE registered_competition_id = $1`, req.TeamID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count current members"})
			return
		}

		// Validate against minimum members requirement
		if minMembers != nil && *minMembers > 0 {
			if currentMemberCount-1 < *minMembers {
				c.JSON(http.StatusForbidden, gin.H{
					"error":          "Cannot remove member - would violate minimum member requirement",
					"minMembers":     *minMembers,
					"currentMembers": currentMemberCount,
				})
				return
			}
		}

		// Start transaction
		tx, err := db.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()

		// Delete the team member
		result, err := tx.Exec(`
            DELETE FROM registered_competition_members 
            WHERE registered_competition_id = $1 
            AND user_email = $2 
            AND is_team_leader = false`, req.TeamID, req.MemberEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove team member"})
			return
		}

		// Check if any rows were affected
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify member removal"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team timestamp"})
			return
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit member removal"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Team member removed successfully",
			"teamId":        req.TeamID,
			"removedMember": req.MemberEmail,
		})
	}
}
