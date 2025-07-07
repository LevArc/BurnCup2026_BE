package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/NotchG/BurnCup/models"
)

// GetCompetitionsHandler returns all competitions with their prizes, requirements, and rules (Gin version)
func GetCompetitionsHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var competitions []models.Competition
		// Query all competitions
		if err := db.Select(&competitions, `SELECT * FROM competitions`); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// For each competition, fetch prizes, requirements, and rules
		for i, comp := range competitions {
			// Prizes
			var prizes []models.Prize
			if err := db.Select(&prizes, `SELECT id, name, description FROM prizes WHERE competition_id=$1`, comp.ID); err == nil {
				competitions[i].Prizes = prizes
			}
			// Requirements
			var reqs []string
			if err := db.Select(&reqs, `SELECT requirement FROM competition_requirements WHERE competition_id=$1`, comp.ID); err == nil {
				competitions[i].Requirements = reqs
			}
			// Rules
			var rules []string
			if err := db.Select(&rules, `SELECT rule FROM competition_rules WHERE competition_id=$1`, comp.ID); err == nil {
				competitions[i].Rules = rules
			}
		}
		c.JSON(http.StatusOK, competitions)
	}
}
