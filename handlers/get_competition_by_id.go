package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	"github.com/NotchG/BurnCup/models"
)

// GetCompetitionByIDHandler returns a competition by its ID with prizes, requirements, and rules
func GetCompetitionByIDHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		row := db.QueryRowx(`
			SELECT
				id, name, description, category, image_url, booklet_url, paid_message,
				registration_start_date, registration_end_date,
				competition_start_date, competition_end_date,
				competition_type, venue, binusian_registration_fee, non_binusian_registration_fee,
				max_members, min_members, team_slot,
				faq, timeline, created_at, updated_at
			FROM competitions
			WHERE id=$1
		`, id)

		var competition models.Competition
		var maxMembers sql.NullInt64
		var minMembers sql.NullInt64
		var faqBytes []byte
		var timelineBytes []byte
		if err := row.Scan(
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
			&competition.BinusianRegistrationFee,
			&competition.NonBinusianRegistrationFee,
			&maxMembers,
			&minMembers,
			&competition.TeamSlot,
			&faqBytes,
			&timelineBytes,
			&competition.CreatedAt,
			&competition.UpdatedAt,
		); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Competition not found"})
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
