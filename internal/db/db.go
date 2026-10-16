package db

import (
	"log"

	"attendance-backend/internal/auth"
	"attendance-backend/internal/config"
	"attendance-backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.User{}, &models.Employee{}, &models.Attendance{}, &models.PasswordHistory{}); err != nil {
		return err
	}
	log.Println("Database migrations completed successfully.")
	return nil
}

func EnsureAdminUser(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	passwordHash, err := auth.HashPassword("Admin@123")
	if err != nil {
		return err
	}

	admin := models.User{
		EmployeeID:           "ADMIN-001",
		FullName:             "System Admin",
		Email:                "admin@company.com",
		Phone:                "+10000000001",
		PasswordHash:         passwordHash,
		Role:                 "admin",
		IsFirstLoginRequired: false,
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	log.Println("Default admin created: admin@company.com / Admin@123")
	return nil
}
