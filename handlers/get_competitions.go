package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/NotchG/BurnCup/models"
)

// GetCompetitionsHandler returns all competitions with their prizes, requirements, and rules (Gin version)
func GetCompetitionsHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.Queryx(`
			SELECT
				id, name, description, category, image_url, booklet_url, paid_message,
				registration_start_date, registration_end_date,
				competition_start_date, competition_end_date,
				competition_type, venue, registration_fee,
				max_members, min_members, team_slot,
				faq, timeline, created_at, updated_at
			FROM competitions
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var competitions []models.Competition
		for rows.Next() {
			var competition models.Competition
			var maxMembers sql.NullInt64
			var minMembers sql.NullInt64
			var faqBytes []byte
			var timelineBytes []byte

			if err := rows.Scan(
				&competition.ID,
				&competition.Name,
				&competition.Description,
				&competition.Category,
				&competition.ImageURL,
				&competition.BookletURL,
				&competition.PaidMessage,
				&competition.RegistrationStartDate,
				&competition.RegistrationEndDate,
				&competition.CompetitionStartDate,
				&competition.CompetitionEndDate,
				&competition.CompetitionType,
				&competition.Venue,
				&competition.RegistrationFee,
				&maxMembers,
				&minMembers,
				&competition.TeamSlot,
				&faqBytes,
				&timelineBytes,
				&competition.CreatedAt,
				&competition.UpdatedAt,
			); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if maxMembers.Valid {
				value := int(maxMembers.Int64)
				competition.MaxMembers = &value
			}
			if minMembers.Valid {
				value := int(minMembers.Int64)
				competition.MinMembers = &value
			}
			if len(faqBytes) > 0 {
				competition.FAQ = json.RawMessage(faqBytes)
			} else {
				competition.FAQ = json.RawMessage(`{}`)
			}
			if len(timelineBytes) > 0 {
				competition.Timeline = json.RawMessage(timelineBytes)
			} else {
				competition.Timeline = json.RawMessage(`[]`)
			}

			competitions = append(competitions, competition)
		}

		if err := rows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
