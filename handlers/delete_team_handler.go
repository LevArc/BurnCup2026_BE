package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// DeleteTeamHandler allows admin to delete teams
func DeleteTeamHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get team code from URL parameter
		teamCode := c.Param("teamCode")
		if teamCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Team code is required"})
			return
		}

		// Check if team exists
		var teamInfo struct {
			ID            string `db:"id"`
			TeamName      string `db:"team_name"`
			IsPaid        bool   `db:"is_paid"`
			CompetitionID string `db:"competition_id"`
		}

		err := db.Get(&teamInfo, `
            SELECT id, team_name, is_paid, competition_id
            FROM registered_competitions 
            WHERE team_code = $1
        `, teamCode)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}

		// Check if team has already paid
		if teamInfo.IsPaid {
			c.JSON(http.StatusConflict, gin.H{"error": "Cannot delete team that has already paid"})
			return
		}

		// Start transaction
		tx, err := db.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()

		// Delete team members first (due to foreign key constraints)
		_, err = tx.Exec(`
            DELETE FROM registered_competition_members 
            WHERE registered_competition_id = $1
        `, teamInfo.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete team members"})
			return
		}

		// Delete the team
		result, err := tx.Exec(`
            DELETE FROM registered_competitions 
            WHERE id = $1
        `, teamInfo.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete team"})
			return
		}

		// Check if any rows were affected
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify deletion"})
			return
		}

		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit deletion"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Team deleted successfully",
			"teamCode":      teamCode,
			"teamName":      teamInfo.TeamName,
			"competitionId": teamInfo.CompetitionID,
		})
	}
}
