package handlers

import (
	"net/http"

	"github.com/NotchG/BurnCup/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
)

// TeamResponse matches the Team interface from your frontend
type TeamResponse struct {
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

// GetUserCompetitionsHandler returns all teams/competitions the current user is registered in
func GetUserCompetitionsHandler(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.Get("user")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No user claims found"})
			return
		}
		mapClaims, ok := claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user claims"})
			return
		}
		userEmail, ok := mapClaims["email"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User email not found in token"})
			return
		}

		// Query all teams where the user is a member
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
            SELECT rc.id, rc.team_name, rc.team_code, rc.is_paid, rc.competition_id, rc.created_at, rc.updated_at
            FROM registered_competitions rc
            JOIN registered_competition_members rcm ON rc.id = rcm.registered_competition_id
            WHERE rcm.user_email = $1
        `, userEmail)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch teams"})
			return
		}

		var result []TeamResponse
		for _, t := range teams {
			// Get competition details
			var competition models.Competition
			if err := db.Get(&competition, `SELECT * FROM competitions WHERE id=$1`, t.CompetitionID); err != nil {
				continue // skip if not found
			}

			// Get team leader
			var teamLeader models.User
			err = db.Get(&teamLeader, `
                SELECT u.email, u.user_type, u.full_name, u.phone_number, u.nim, u.major, u.school
                FROM users u
                JOIN registered_competition_members rcm ON u.email = rcm.user_email
                WHERE rcm.registered_competition_id = $1 AND rcm.is_team_leader = true
                LIMIT 1
            `, t.ID)
			if err != nil {
				teamLeader = models.User{}
			}

			// Get all members except the team leader
			var members []models.User
			err = db.Select(&members, `
                SELECT u.email, u.user_type, u.full_name, u.phone_number, u.nim, u.major, u.school
                FROM users u
                JOIN registered_competition_members rcm ON u.email = rcm.user_email
                WHERE rcm.registered_competition_id = $1 AND rcm.is_team_leader = false
            `, t.ID)
			if err != nil {
				members = []models.User{}
			}

			result = append(result, TeamResponse{
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
