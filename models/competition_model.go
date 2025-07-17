package models

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
	BookletURL            string   `db:"booklet_url" json:"bookletUrl"`
	PaidMessage           string   `db:"paid_message" json:"paidMessage"`
	RegistrationStartDate string   `db:"registration_start_date" json:"registrationStartDate"`
	RegistrationEndDate   string   `db:"registration_end_date" json:"registrationEndDate"`
	CompetitionStartDate  string   `db:"competition_start_date" json:"competitionStartDate"`
	CompetitionEndDate    string   `db:"competition_end_date" json:"competitionEndDate"`
	CompetitionType       string   `db:"competition_type" json:"competitionType"`
	Venue                 string   `db:"venue" json:"venue"`
	RegistrationFee       int      `db:"registration_fee" json:"registrationfee"`
	MaxMembers            *int     `db:"max_members" json:"maxMembers,omitempty"`
	MinMembers            *int     `db:"min_members" json:"minMembers,omitempty"`
	TeamSlot              int      `db:"team_slot" json:"teamSlot"`
	CreatedAt             string   `db:"created_at" json:"createdAt"`
	UpdatedAt             string   `db:"updated_at" json:"updatedAt"`
	Prizes                []Prize  `json:"prizes"`
	Requirements          []string `json:"requirements"`
	Rules                 []string `json:"rules"`
}
