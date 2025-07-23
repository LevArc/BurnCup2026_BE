package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// GetRemainingTeamSlotHandler returns the remaining team slots for a competition
func GetRemainingTeamSlotHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		competitionID := c.Param("competitionId")
		if competitionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Competition ID is required"})
			return
		}

		// Get total team slots for the competition
		var totalSlots int
		err := db.Get(&totalSlots, `
            SELECT team_slot FROM competitions 
            WHERE id = $1
        `, competitionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Competition not found"})
			return
		}

		// Get current number of PAID teams registered (only paid teams count towards slot limit)
		var paidTeams int
		err = db.Get(&paidTeams, `
            SELECT COUNT(*) FROM registered_competitions 
            WHERE competition_id = $1 AND is_paid = true
        `, competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count paid teams"})
			return
		}

		// Calculate remaining slots (based on paid teams only)
		remainingSlots := totalSlots - paidTeams
		if remainingSlots < 0 {
			remainingSlots = 0
		}

		c.JSON(http.StatusOK, gin.H{
			"remainingSlots": remainingSlots,
		})
	}
}
