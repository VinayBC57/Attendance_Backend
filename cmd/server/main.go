package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"attendance-backend/internal/auth"
	"attendance-backend/internal/config"
	"attendance-backend/internal/db"
	"attendance-backend/internal/models"
)

type loginRequest struct {
	EmployeeID string `json:"employee_id" binding:"required"`
	Password   string `json:"password" binding:"required"`
}

type changePasswordRequest struct {
	EmployeeID      string `json:"employee_id" binding:"required"`
	OldPassword     string `json:"old_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

type employeeRequest struct {
	FullName    string `json:"full_name" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Phone       string `json:"phone" binding:"required"`
	Department  string `json:"department"`
	Designation string `json:"designation"`
}

func main() {
	if err := config.Load(); err != nil {
		log.Fatal("config load failed: ", err)
	}
	cfg := config.Get()

	gormDB, err := gorm.Open(postgres.Open(cfg.DatabaseURL()), &gorm.Config{})
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	if err := db.Migrate(gormDB); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}
	if err := db.EnsureAdminUser(gormDB); err != nil {
		log.Fatalf("default admin creation failed: %v", err)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/api/v1/auth/login", func(c *gin.Context) {
		loginHandler(c, gormDB, cfg.JWTSecret)
	})
	r.GET("/api/v1/auth/me", auth.UserMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		currentUserHandler(c, gormDB)
	})

	r.POST("/api/v1/auth/change-password", func(c *gin.Context) {
		changePasswordHandler(c, gormDB)
	})

	r.POST("/api/v1/admin/employees", auth.AdminMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		createEmployeeHandler(c, gormDB, cfg)
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Attendance API running on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func loginHandler(c *gin.Context, dbConn *gorm.DB, secret string) {
	var payload loginRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "INVALID_REQUEST", "message": err.Error()}})
		return
	}

	var user models.User
	if err := dbConn.Where("employee_id = ?", payload.EmployeeID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "INVALID_CREDENTIALS", "message": "Invalid employee ID or password"}})
		return
	}
	if !auth.CheckPasswordHash(payload.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "INVALID_CREDENTIALS", "message": "Invalid employee ID or password"}})
		return
	}

	token, err := auth.CreateToken(user, secret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "TOKEN_ERROR", "message": "Unable to issue token"}})
		return
	}

	data := gin.H{
		"token": token,
		"user": gin.H{
			"employee_id":             user.EmployeeID,
			"role":                    user.Role,
			"full_name":               user.FullName,
			"is_first_login_required": user.IsFirstLoginRequired,
		},
	}
	if user.IsFirstLoginRequired {
		data["require_password_change"] = true
		data["message"] = "Password change required on first login."
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func currentUserHandler(c *gin.Context, dbConn *gorm.DB) {
	employeeID := c.GetString("employee_id")
	var user models.User
	if err := dbConn.Where("employee_id = ?", employeeID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "USER_NOT_FOUND", "message": "User not found."}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "USER_LOOKUP_FAILED", "message": "Unable to load user profile."}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"user": gin.H{
		"employee_id":             user.EmployeeID,
		"role":                    user.Role,
		"full_name":               user.FullName,
		"is_first_login_required": user.IsFirstLoginRequired,
	}}})
}

func changePasswordHandler(c *gin.Context, dbConn *gorm.DB) {
	var payload changePasswordRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "INVALID_REQUEST", "message": err.Error()}})
		return
	}
	if payload.NewPassword != payload.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "PASSWORD_MISMATCH", "message": "New password and confirm password do not match."}})
		return
	}
	if !auth.ValidatePassword(payload.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "WEAK_PASSWORD", "message": "Password must contain at least 7 chars, 1 uppercase, 1 number, and 1 special character."}})
		return
	}

	var user models.User
	if err := dbConn.Where("employee_id = ?", payload.EmployeeID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "USER_NOT_FOUND", "message": "Employee not found."}})
		return
	}
	if !auth.CheckPasswordHash(payload.OldPassword, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "INVALID_PASSWORD", "message": "Current password is incorrect."}})
		return
	}

	oldHash := user.PasswordHash
	newHash, err := auth.HashPassword(payload.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "HASH_ERROR", "message": "Unable to save new password."}})
		return
	}

	now := time.Now()
	user.PasswordHash = newHash
	user.IsFirstLoginRequired = false
	user.PasswordChangedAt = &now
	if err := dbConn.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "UPDATE_FAILED", "message": "Unable to update password."}})
		return
	}

	if err := dbConn.Create(&models.PasswordHistory{
		ID:              uuid.New(),
		EmployeeID:      user.EmployeeID,
		ChangedBy:       user.EmployeeID,
		OldPasswordHash: oldHash,
		NewPasswordHash: newHash,
		ChangeType:      "first_login",
		ChangedAt:       now,
		CreatedAt:       now,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "AUDIT_FAILED", "message": "Password changed but audit log failed."}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"message": "Password changed successfully", "first_login_completed": true}})
}

func createEmployeeHandler(c *gin.Context, dbConn *gorm.DB, cfg config.Config) {
	var payload employeeRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "INVALID_REQUEST", "message": err.Error()}})
		return
	}

	var existing models.User
	if err := dbConn.Where("email = ? OR phone = ?", payload.Email, payload.Phone).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": gin.H{"code": "DUPLICATE_EMPLOYEE", "message": "Email or phone already exists."}})
		return
	}

	var count int64
	if err := dbConn.Model(&models.User{}).Where("role = ?", "employee").Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "COUNT_ERROR", "message": "Unable to generate employee ID."}})
		return
	}

	employeeID := auth.GenerateEmployeeID(int(count))
	tempPass, err := auth.GenerateTemporaryPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "PASSWORD_ERROR", "message": "Unable to generate temporary password."}})
		return
	}
	passwordHash, err := auth.HashPassword(tempPass)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "HASH_ERROR", "message": "Unable to hash temporary password."}})
		return
	}

	user := models.User{
		ID:                   uuid.New(),
		EmployeeID:           employeeID,
		FullName:             payload.FullName,
		Email:                payload.Email,
		Phone:                payload.Phone,
		PasswordHash:         passwordHash,
		Role:                 "employee",
		IsFirstLoginRequired: true,
	}
	if err := dbConn.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "CREATE_USER_FAILED", "message": "Unable to create employee."}})
		return
	}

	if err := dbConn.Create(&models.Employee{
		ID:                  uuid.New(),
		UserID:              user.ID,
		EmployeeID:          employeeID,
		Department:          payload.Department,
		Designation:         payload.Designation,
		OfficeLatitude:      cfg.OfficeLatitude,
		OfficeLongitude:     cfg.OfficeLongitude,
		AllowedRadiusMeters: cfg.AllowedRadiusMeters,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "CREATE_PROFILE_FAILED", "message": "Employee account created but profile creation failed."}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"employee_id":        employeeID,
			"temporary_password": tempPass,
			"message":            "Employee created successfully. Temporary password was generated and must be changed on first login.",
		},
	})
}

func init() {
	_ = os.Setenv("TZ", "UTC")
}
