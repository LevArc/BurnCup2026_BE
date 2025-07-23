package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// PingIsPaidTeamSlotHandler returns team payment status and remaining competition slots
func PingIsPaidTeamSlotHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		teamID := c.Param("teamId")
		if teamID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Team ID is required"})
			return
		}

		// Get team payment status and competition ID
		var isPaid bool
		var competitionID string
		err := db.QueryRowx(`
            SELECT is_paid, competition_id 
            FROM registered_competitions 
            WHERE id = $1
        `, teamID).Scan(&isPaid, &competitionID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}

		// Get total team slots for the competition
		var totalSlots int
		err = db.Get(&totalSlots, `
            SELECT team_slot FROM competitions 
            WHERE id = $1
        `, competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Competition not found"})
			return
		}

		// Get current number of PAID teams registered
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
			"isPaid":         isPaid,
			"remainingSlots": remainingSlots,
		})
	}
}
