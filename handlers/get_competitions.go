package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jmoiron/sqlx"
)

// Prize struct matches the Prize interface and DB schema
type Prize struct {
	ID          string `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description"`
}

// Competition struct matches the Competition interface and DB schema
type Competition struct {
	ID                    string   `db:"id" json:"id"`
	Name                  string   `db:"name" json:"name"`
	Description           string   `db:"description" json:"description"`
	Category              string   `db:"category" json:"category"`
	ImageURL              string   `db:"image_url" json:"imageUrl"`
	RegistrationStartDate string   `db:"registration_start_date" json:"registrationStartDate"`
	RegistrationEndDate   string   `db:"registration_end_date" json:"registrationEndDate"`
	CompetitionStartDate  string   `db:"competition_start_date" json:"competitionStartDate"`
	CompetitionEndDate    string   `db:"competition_end_date" json:"competitionEndDate"`
	CompetitionType       string   `db:"competition_type" json:"competitionType"`
	Venue                 string   `db:"venue" json:"venue"`
	RegistrationFee       int      `db:"registration_fee" json:"registrationfee"`
	MaxMembers            *int     `db:"max_members" json:"maxMembers,omitempty"`
	MinMembers            *int     `db:"min_members" json:"minMembers,omitempty"`
	Prizes                []Prize  `json:"prizes"`
	Requirements          []string `json:"requirements"`
	Rules                 []string `json:"rules"`
}

// GetCompetitionsHandler returns all competitions with their prizes, requirements, and rules
func GetCompetitionsHandler(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var competitions []Competition
		// Query all competitions
		if err := db.Select(&competitions, `SELECT * FROM competitions`); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// For each competition, fetch prizes, requirements, and rules
		for i, c := range competitions {
			// Prizes
			var prizes []Prize
			if err := db.Select(&prizes, `SELECT id, name, description FROM prizes WHERE competition_id=$1`, c.ID); err == nil {
				competitions[i].Prizes = prizes
			}
			// Requirements
			var reqs []string
			if err := db.Select(&reqs, `SELECT requirement FROM competition_requirements WHERE competition_id=$1`, c.ID); err == nil {
				competitions[i].Requirements = reqs
			}
			// Rules
			var rules []string
			if err := db.Select(&rules, `SELECT rule FROM competition_rules WHERE competition_id=$1`, c.ID); err == nil {
				competitions[i].Rules = rules
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(competitions)
	}
}
