package models

type User struct {
    UserType    string  `db:"user_type" json:"userType"`
    FullName    string  `db:"full_name" json:"fullName"`
    PhoneNumber string  `db:"phone_number" json:"phoneNumber"`
	Email	   	string  `db:"email" json:"email"`
    NIM         *string `db:"nim" json:"nim,omitempty"`
    Major       *string `db:"major" json:"major,omitempty"`
    School      *string `db:"school" json:"school,omitempty"`
}