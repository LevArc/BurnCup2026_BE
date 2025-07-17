package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// DeleteCompetitionHandler deletes a competition and all associated data
func DeleteCompetitionHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		competitionID := c.Param("id")
		if competitionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Competition ID is required"})
			return
		}

		// Check if competition exists
		var exists bool
		err := db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM competitions WHERE id = $1)", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check competition existence"})
			return
		}
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "Competition not found"})
			return
		}

		// Check if there are any registered teams for this competition
		var registeredTeams int
		err = db.Get(&registeredTeams, "SELECT COUNT(*) FROM registered_competitions WHERE competition_id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check registered teams"})
			return
		}

		if registeredTeams > 0 {
			c.JSON(http.StatusConflict, gin.H{
				"error":           "Cannot delete competition with registered teams",
				"registeredTeams": registeredTeams,
			})
			return
		}

		// Start transaction
		tx, err := db.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()

		// Delete related data (foreign key constraints should handle this automatically with CASCADE)
		// But we'll be explicit for clarity and error handling

		// Delete prizes
		_, err = tx.Exec("DELETE FROM prizes WHERE competition_id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete competition prizes"})
			return
		}

		// Delete requirements
		_, err = tx.Exec("DELETE FROM competition_requirements WHERE competition_id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete competition requirements"})
			return
		}

		// Delete rules
		_, err = tx.Exec("DELETE FROM competition_rules WHERE competition_id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete competition rules"})
			return
		}

		// Delete the competition itself
		result, err := tx.Exec("DELETE FROM competitions WHERE id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete competition"})
			return
		}

		// Check if any rows were affected
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify deletion"})
			return
		}

		if rowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Competition not found"})
			return
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit deletion"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Competition deleted successfully",
			"competitionId": competitionID,
		})
	}
}
