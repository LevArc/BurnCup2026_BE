package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
)

// Prize represents a prize for a competition
type Prize struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

// AddCompetitionRequest represents the request body for adding a competition
type AddCompetitionRequest struct {
    Name                    string   `json:"name" binding:"required"`
    Description             string   `json:"description" binding:"required"`
    Category                string   `json:"category" binding:"required"`
    ImageUrl                string   `json:"imageUrl" binding:"required"`
    RegistrationStartDate   string   `json:"registrationStartDate" binding:"required"`
    RegistrationEndDate     string   `json:"registrationEndDate" binding:"required"`
    CompetitionStartDate    string   `json:"competitionStartDate" binding:"required"`
    CompetitionEndDate      string   `json:"competitionEndDate" binding:"required"`
    CompetitionType         string   `json:"competitionType" binding:"required"`
    Venue                   string   `json:"venue" binding:"required"`
    RegistrationFee         int      `json:"registrationfee" binding:"required"`
    Prizes                  []Prize  `json:"prizes"`
    Requirements            []string `json:"requirements"`
    Rules                   []string `json:"rules"`
    MaxMembers              *int     `json:"maxMembers"`
    MinMembers              *int     `json:"minMembers"`
	TeamSlot			   	int      `json:"teamSlot" binding:"required"`
}

// AddCompetitionHandler creates a new competition with associated prizes, requirements, and rules
func AddCompetitionHandler(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req AddCompetitionRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
            return
        }

        // Start transaction
        tx, err := db.Beginx()
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
            return
        }
        defer tx.Rollback()

        // Insert competition
        var competitionID string
        err = tx.QueryRowx(`
            INSERT INTO competitions (
                name, description, category, image_url, 
                registration_start_date, registration_end_date,
                competition_start_date, competition_end_date,
                competition_type, venue, registration_fee,
                max_members, min_members, team_slot
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
            RETURNING id
        `, req.Name, req.Description, req.Category, req.ImageUrl,
            req.RegistrationStartDate, req.RegistrationEndDate,
            req.CompetitionStartDate, req.CompetitionEndDate,
            req.CompetitionType, req.Venue, req.RegistrationFee,
            req.MaxMembers, req.MinMembers, req.TeamSlot).Scan(&competitionID)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create competition"})
            return
        }

        // Insert prizes
        for _, prize := range req.Prizes {
            _, err = tx.Exec(`
                INSERT INTO prizes (competition_id, name, description)
                VALUES ($1, $2, $3)
            `, competitionID, prize.Name, prize.Description)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add prizes"})
                return
            }
        }

        // Insert requirements
        for _, requirement := range req.Requirements {
            _, err = tx.Exec(`
                INSERT INTO competition_requirements (competition_id, requirement)
                VALUES ($1, $2)
            `, competitionID, requirement)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add requirements"})
                return
            }
        }

        // Insert rules
        for _, rule := range req.Rules {
            _, err = tx.Exec(`
                INSERT INTO competition_rules (competition_id, rule)
                VALUES ($1, $2)
            `, competitionID, rule)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add rules"})
                return
            }
        }

        // Commit transaction
        if err = tx.Commit(); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
            return
        }

        c.JSON(http.StatusCreated, gin.H{
            "message":       "Competition created successfully",
            "competitionId": competitionID,
        })
    }
}