package handlers

import (
	"net/http"

	"github.com/NotchG/BurnCup/models"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// AllTeamResponse matches the Team interface for the frontend
type AllTeamResponse struct {
	ID          string             `json:"id"`
	TeamName    string             `json:"teamName"`
	TeamCode    string             `json:"teamCode"`
	IsPaid      bool               `json:"isPaid"`
	Competition models.Competition `json:"competition"`
	Members     []models.User      `json:"members"`
	TeamLeader  models.User        `json:"teamLeader"`
	CreatedAt   string             `json:"createdAt"`
	UpdatedAt   string             `json:"updatedAt"`
}

// GetAllTeamsHandler returns all teams with their details
func GetAllTeamsHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var teams []struct {
			ID            string `db:"id"`
			TeamName      string `db:"team_name"`
			TeamCode      string `db:"team_code"`
			IsPaid        bool   `db:"is_paid"`
			CompetitionID string `db:"competition_id"`
			CreatedAt     string `db:"created_at"`
			UpdatedAt     string `db:"updated_at"`
		}

		err := db.Select(&teams, `
			SELECT id, team_name, team_code, is_paid, competition_id, created_at, updated_at
			FROM registered_competitions
			ORDER BY created_at DESC
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch teams"})
			return
		}

		var result []AllTeamResponse
		for _, t := range teams {
			// Get competition details
			var competition models.Competition
			if err := db.Get(&competition, `SELECT * FROM competitions WHERE id=$1`, t.CompetitionID); err != nil {
				continue
			}

			// Get team leader — fix: JOIN on user_id instead of user_email
			var teamLeader models.User
			err = db.Get(&teamLeader, `
				SELECT u.id, u.email, u.user_type, u.full_name, u.phone_number, u.nim, u.major, u.school
				FROM users u
				JOIN registered_competition_members rcm ON u.id = rcm.user_id
				WHERE rcm.registered_competition_id = $1 AND rcm.is_team_leader = true
				LIMIT 1
			`, t.ID)
			if err != nil {
				teamLeader = models.User{}
			}

			// Get all members except the team leader — fix: JOIN on user_id
			members := make([]models.User, 0)
			err = db.Select(&members, `
				SELECT u.id, u.email, u.user_type, u.full_name, u.phone_number, u.nim, u.major, u.school
				FROM users u
				JOIN registered_competition_members rcm ON u.id = rcm.user_id
				WHERE rcm.registered_competition_id = $1 AND rcm.is_team_leader = false
			`, t.ID)
			if err != nil {
				members = []models.User{}
			}

			result = append(result, AllTeamResponse{
				ID:          t.ID,
				TeamName:    t.TeamName,
				TeamCode:    t.TeamCode,
				IsPaid:      t.IsPaid,
				Competition: competition,
				Members:     members,
				TeamLeader:  teamLeader,
				CreatedAt:   t.CreatedAt,
				UpdatedAt:   t.UpdatedAt,
			})
		}

		c.JSON(http.StatusOK, result)
	}
}
