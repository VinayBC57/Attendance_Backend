package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	EmployeeID           string     `gorm:"uniqueIndex;not null" json:"employee_id"`
	FullName             string     `gorm:"not null" json:"full_name"`
	Email                string     `gorm:"uniqueIndex;not null" json:"email"`
	Phone                string     `gorm:"uniqueIndex;not null" json:"phone"`
	PasswordHash         string     `gorm:"not null" json:"-"`
	Role                 string     `gorm:"default:'employee'" json:"role"`
	IsFirstLoginRequired bool       `gorm:"default:true" json:"is_first_login_required"`
	PasswordChangedAt    *time.Time `json:"password_changed_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type Employee struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID              uuid.UUID `gorm:"uniqueIndex;not null" json:"user_id"`
	EmployeeID          string    `gorm:"uniqueIndex;not null" json:"employee_id"`
	Department          string    `json:"department"`
	Designation         string    `json:"designation"`
	OfficeLatitude      float64   `json:"office_latitude"`
	OfficeLongitude     float64   `json:"office_longitude"`
	AllowedRadiusMeters int       `gorm:"default:100" json:"allowed_radius_meters"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Attendance struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	EmployeeID   string     `gorm:"index;not null" json:"employee_id"`
	CheckInTime  time.Time  `json:"check_in_time"`
	CheckOutTime *time.Time `json:"check_out_time,omitempty"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	Accuracy     float64    `json:"accuracy"`
	Status       string     `gorm:"default:'present'" json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type PasswordHistory struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	EmployeeID      string    `gorm:"index;not null" json:"employee_id"`
	ChangedBy       string    `json:"changed_by"`
	OldPasswordHash string    `json:"old_password_hash,omitempty"`
	NewPasswordHash string    `gorm:"not null" json:"new_password_hash"`
	ChangeType      string    `gorm:"default:'first_login'" json:"change_type"`
	ChangedAt       time.Time `json:"changed_at"`
	CreatedAt       time.Time `json:"created_at"`
}
