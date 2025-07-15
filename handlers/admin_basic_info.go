package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/jmoiron/sqlx"
)

// AdminBasicInfoResponse is the response structure for admin-basic-info
type AdminBasicInfoResponse struct {
    TotalUsers         int `json:"totalUsers"`
    BinusianUsers      int `json:"binusianUsers"`
    NonBinusianUsers   int `json:"nonBinusianUsers"`
    TotalTeams         int `json:"totalTeams"`
    ActiveCompetitions int `json:"activeCompetitions"`
    UpcomingEvents     int `json:"upcomingEvents"`
    TotalParticipants  int `json:"totalParticipants"`
    CategoryCount      int `json:"categoryCount"`
}

// GetAdminBasicInfoHandler returns basic admin dashboard info
func GetAdminBasicInfoHandler(db *sqlx.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        var totalUsers, binusianUsers, nonBinusianUsers, totalTeams, activeCompetitions, upcomingEvents, totalParticipants, categoryCount int

        // Total users
        _ = db.Get(&totalUsers, `SELECT COUNT(*) FROM users`)
        _ = db.Get(&binusianUsers, `SELECT COUNT(*) FROM users WHERE binusian = true`)
        _ = db.Get(&nonBinusianUsers, `SELECT COUNT(*) FROM users WHERE binusian = false`)

        // Total teams
        _ = db.Get(&totalTeams, `SELECT COUNT(*) FROM registered_competitions`)

        // Active competitions (registration_end_date >= today)
        _ = db.Get(&activeCompetitions, `SELECT COUNT(*) FROM competitions WHERE registration_end_date >= CURRENT_DATE`)

        // Upcoming events (competition_start_date > today)
        _ = db.Get(&upcomingEvents, `SELECT COUNT(*) FROM competitions WHERE competition_start_date > CURRENT_DATE`)

        // Total participants (unique user_emails in registered_competition_members)
        _ = db.Get(&totalParticipants, `SELECT COUNT(DISTINCT user_email) FROM registered_competition_members`)

        // Category count (distinct categories in competitions)
        _ = db.Get(&categoryCount, `SELECT COUNT(DISTINCT category) FROM competitions`)

        c.JSON(http.StatusOK, AdminBasicInfoResponse{
            TotalUsers:         totalUsers,
            BinusianUsers:      binusianUsers,
            NonBinusianUsers:   nonBinusianUsers,
            TotalTeams:         totalTeams,
            ActiveCompetitions: activeCompetitions,
            UpcomingEvents:     upcomingEvents,
            TotalParticipants:  totalParticipants,
            CategoryCount:      categoryCount,
        })
	}
}