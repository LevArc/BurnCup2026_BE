package models

// User represents the database model for a user
type User struct {
    ID          string  `db:"id" json:"id"` 
    UserType    *string `db:"user_type" json:"userType,omitempty"` 
    Email       string  `db:"email" json:"email"`
    Password    string  `db:"password" json:"-"` 
    
    // Changed to pointers to handle NULLs
    FullName    *string `db:"full_name" json:"fullName,omitempty"`
    PhoneNumber *string `db:"phone_number" json:"phoneNumber,omitempty"`
    
    NIM         *string `db:"nim" json:"nim,omitempty"`
    Major       *string `db:"major" json:"major,omitempty"`
    School      *string `db:"school" json:"school,omitempty"`
}
// RegisterRequest ONLY requires the essentials for sign-up
type RegisterRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest is used to parse incoming login JSON payloads
type LoginRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required"`
}