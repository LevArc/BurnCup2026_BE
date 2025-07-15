package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// CompetitionStats matches the frontend interface for competition statistics
type CompetitionStats struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Category          string `json:"category"`
	TotalTeams        int    `json:"totalTeams"`
	TotalParticipants int    `json:"totalParticipants"`
	PaidTeams         int    `json:"paidTeams"`
	PendingTeams      int    `json:"pendingTeams"`
	RegistrationFee   int    `json:"registrationFee"`
	CompetitionType   string `json:"competitionType"`
}

// GetAdminCompetitionStatisticHandler returns detailed stats for each competition
func GetAdminCompetitionStatisticHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type competitionRow struct {
			ID              string `db:"id"`
			Name            string `db:"name"`
			Category        string `db:"category"`
			RegistrationFee int    `db:"registration_fee"`
			CompetitionType string `db:"competition_type"`
		}
		var competitions []competitionRow
		err := db.Select(&competitions, `
            SELECT id, name, category, registration_fee, competition_type
            FROM competitions
            ORDER BY name
        `)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch competitions"})
			return
		}

		var stats []CompetitionStats
		for _, comp := range competitions {
			var totalTeams, totalParticipants, paidTeams, pendingTeams int

			// Total teams
			_ = db.Get(&totalTeams, `
                SELECT COUNT(*) FROM registered_competitions
                WHERE competition_id = $1
            `, comp.ID)

			// Paid teams
			_ = db.Get(&paidTeams, `
                SELECT COUNT(*) FROM registered_competitions
                WHERE competition_id = $1 AND is_paid = true
            `, comp.ID)

			// Pending teams
			_ = db.Get(&pendingTeams, `
                SELECT COUNT(*) FROM registered_competitions
                WHERE competition_id = $1 AND is_paid = false
            `, comp.ID)

			// Total participants (unique user_email)
			_ = db.Get(&totalParticipants, `
                SELECT COUNT(DISTINCT rcm.user_email)
                FROM registered_competition_members rcm
                JOIN registered_competitions rc ON rc.id = rcm.registered_competition_id
                WHERE rc.competition_id = $1
            `, comp.ID)

			stats = append(stats, CompetitionStats{
				ID:                comp.ID,
				Name:              comp.Name,
				Category:          comp.Category,
				TotalTeams:        totalTeams,
				TotalParticipants: totalParticipants,
				PaidTeams:         paidTeams,
				PendingTeams:      pendingTeams,
				RegistrationFee:   comp.RegistrationFee,
				CompetitionType:   comp.CompetitionType,
			})
		}

		c.JSON(http.StatusOK, stats)
	}
}
