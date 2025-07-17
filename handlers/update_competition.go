package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// UpdateCompetitionRequest represents the request body for updating a competition
type UpdateCompetitionRequest struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	Category              string   `json:"category"`
	ImageUrl              string   `json:"imageUrl"`
	BookletUrl            string   `json:"bookletUrl"`
	PaidMessage           string   `json:"paidMessage"`
	RegistrationStartDate string   `json:"registrationStartDate"`
	RegistrationEndDate   string   `json:"registrationEndDate"`
	CompetitionStartDate  string   `json:"competitionStartDate"`
	CompetitionEndDate    string   `json:"competitionEndDate"`
	CompetitionType       string   `json:"competitionType"`
	Venue                 string   `json:"venue"`
	RegistrationFee       int      `json:"registrationfee"`
	Prizes                []Prize  `json:"prizes"`
	Requirements          []string `json:"requirements"`
	Rules                 []string `json:"rules"`
	MaxMembers            *int     `json:"maxMembers"`
	MinMembers            *int     `json:"minMembers"`
	TeamSlot              int      `json:"teamSlot"`
}

// UpdateCompetitionHandler updates an existing competition with associated prizes, requirements, and rules
func UpdateCompetitionHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		competitionID := c.Param("id")
		if competitionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Competition ID is required"})
			return
		}

		var req UpdateCompetitionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
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

		// Start transaction
		tx, err := db.Beginx()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()

		// Update competition with all fields
		_, err = tx.Exec(`
            UPDATE competitions SET 
                name = $2,
                description = $3,
                category = $4,
                image_url = $5,
                booklet_url = $6,
                paid_message = $7,
                registration_start_date = $8,
                registration_end_date = $9,
                competition_start_date = $10,
                competition_end_date = $11,
                competition_type = $12,
                venue = $13,
                registration_fee = $14,
                max_members = $15,
                min_members = $16,
                team_slot = $17,
                updated_at = NOW()
            WHERE id = $1`,
			competitionID,
			req.Name,
			req.Description,
			req.Category,
			req.ImageUrl,
			req.BookletUrl,
			req.PaidMessage,
			req.RegistrationStartDate,
			req.RegistrationEndDate,
			req.CompetitionStartDate,
			req.CompetitionEndDate,
			req.CompetitionType,
			req.Venue,
			req.RegistrationFee,
			req.MaxMembers,
			req.MinMembers,
			req.TeamSlot,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update competition"})
			return
		}

		// Delete and recreate prizes
		_, err = tx.Exec("DELETE FROM prizes WHERE competition_id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete existing prizes"})
			return
		}

		for _, prize := range req.Prizes {
			_, err = tx.Exec(`
                INSERT INTO prizes (competition_id, name, description)
                VALUES ($1, $2, $3)
            `, competitionID, prize.Name, prize.Description)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update prizes"})
				return
			}
		}

		// Delete and recreate requirements
		_, err = tx.Exec("DELETE FROM competition_requirements WHERE competition_id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete existing requirements"})
			return
		}

		for _, requirement := range req.Requirements {
			_, err = tx.Exec(`
                INSERT INTO competition_requirements (competition_id, requirement)
                VALUES ($1, $2)
            `, competitionID, requirement)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update requirements"})
				return
			}
		}

		// Delete and recreate rules
		_, err = tx.Exec("DELETE FROM competition_rules WHERE competition_id = $1", competitionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete existing rules"})
			return
		}

		for _, rule := range req.Rules {
			_, err = tx.Exec(`
                INSERT INTO competition_rules (competition_id, rule)
                VALUES ($1, $2)
            `, competitionID, rule)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update rules"})
				return
			}
		}

		// Commit transaction
		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Competition updated successfully",
			"competitionId": competitionID,
		})
	}
}
