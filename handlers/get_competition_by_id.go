package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/NotchG/BurnCup/models"
)

// GetCompetitionByIDHandler returns a competition by its ID with prizes, requirements, and rules
func GetCompetitionByIDHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var competition models.Competition
		if err := db.Get(&competition, `SELECT * FROM competitions WHERE id=$1`, id); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Competition not found"})
			return
		}

		// Prizes
		var prizes []models.Prize
		if err := db.Select(&prizes, `SELECT id, name, description FROM prizes WHERE competition_id=$1`, competition.ID); err == nil {
			competition.Prizes = prizes
		}

		// Requirements
		var reqs []string
		if err := db.Select(&reqs, `SELECT requirement FROM competition_requirements WHERE competition_id=$1`, competition.ID); err == nil {
			competition.Requirements = reqs
		}

		// Rules
		var rules []string
		if err := db.Select(&rules, `SELECT rule FROM competition_rules WHERE competition_id=$1`, competition.ID); err == nil {
			competition.Rules = rules
		}

		c.JSON(http.StatusOK, competition)
	}
}
