package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "attendance-backend/docs"
	"attendance-backend/internal/auth"
	"attendance-backend/internal/config"
	"attendance-backend/internal/db"
	"attendance-backend/internal/models"
)

// @title Office Attendance Management API
// @version 1.0
// @description Go backend for office attendance, employee management, and first-login password flows.
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email support@example.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /

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

type userSummary struct {
	EmployeeID string `json:"employee_id"`
	Role       string `json:"role"`
	FullName   string `json:"full_name"`
}

type loginResponseData struct {
	Token                 string      `json:"token,omitempty"`
	User                  userSummary `json:"user,omitempty"`
	RequirePasswordChange bool        `json:"require_password_change,omitempty"`
	Message               string      `json:"message,omitempty"`
}

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type employeeCreationResponseData struct {
	EmployeeID        string `json:"employee_id"`
	TemporaryPassword string `json:"temporary_password"`
	Message           string `json:"message"`
}

type employeeUpdateRequest struct {
	FullName    string `json:"full_name" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	Phone       string `json:"phone" binding:"required"`
	Department  string `json:"department"`
	Designation string `json:"designation"`
}

type employeeResponse struct {
	EmployeeID           string     `json:"employee_id"`
	FullName             string     `json:"full_name"`
	Email                string     `json:"email"`
	Phone                string     `json:"phone"`
	Department           string     `json:"department"`
	Designation          string     `json:"designation"`
	Role                 string     `json:"role"`
	IsFirstLoginRequired bool       `json:"is_first_login_required"`
	PasswordChangedAt    *time.Time `json:"password_changed_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type employeeListResponse struct {
	Items []employeeResponse `json:"items"`
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"page_size"`
}

// main initializes configuration, database state, middleware, routes, and the HTTP server.
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
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", healthHandler)

	r.POST("/api/v1/auth/login", func(c *gin.Context) {
		loginHandler(c, gormDB, cfg.JWTSecret)
	})

	r.GET("/api/v1/auth/me", auth.UserMiddleware(cfg.JWTSecret), func(c *gin.Context) {
		currentUserHandler(c, gormDB)
	})

	r.POST("/api/v1/auth/change-password", func(c *gin.Context) {
		changePasswordHandler(c, gormDB)
	})

	admin := r.Group("/api/v1/admin", auth.AdminMiddleware(cfg.JWTSecret))
	admin.POST("/employees", func(c *gin.Context) {
		createEmployeeHandler(c, gormDB, cfg)
	})
	admin.GET("/employees", func(c *gin.Context) {
		listEmployeesHandler(c, gormDB)
	})
	admin.GET("/employees/:employee_id", func(c *gin.Context) {
		getEmployeeHandler(c, gormDB)
	})
	admin.PUT("/employees/:employee_id", func(c *gin.Context) {
		updateEmployeeHandler(c, gormDB)
	})
	admin.POST("/employees/:employee_id/password-reset", func(c *gin.Context) {
		resetEmployeePasswordHandler(c, gormDB)
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Attendance API running on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

// healthHandler reports whether the HTTP service is accepting requests.
// HealthCheck godoc
// @Summary Health check
// @Description Returns the service status.
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// loginHandler authenticates an employee or admin and returns a signed JWT and login state.
// Login godoc
// @Summary Admin or employee login
// @Description Authenticates either an admin or an employee and returns a JWT.
// @Tags auth
// @Accept json
// @Produce json
// @Param loginRequest body loginRequest true "Login payload"
// @Success 200 {object} apiResponse
// @Failure 400 {object} apiResponse
// @Failure 401 {object} apiResponse
// @Failure 500 {object} apiResponse
// @Router /api/v1/auth/login [post]
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

// changePasswordHandler validates and persists a password update and creates an audit entry.
// ChangePassword godoc
// @Summary Change password
// @Description Changes a password and records the password-change audit entry.
// @Tags auth
// @Accept json
// @Produce json
// @Param changePasswordRequest body changePasswordRequest true "Password change payload"
// @Success 200 {object} apiResponse
// @Failure 400 {object} apiResponse
// @Failure 401 {object} apiResponse
// @Failure 404 {object} apiResponse
// @Failure 500 {object} apiResponse
// @Router /api/v1/auth/change-password [post]
// CurrentUser godoc
// @Summary Get current authenticated user
// @Description Returns the latest profile and first-login state for the authenticated user.
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} apiResponse
// @Failure 401 {object} apiResponse
// @Failure 404 {object} apiResponse
// @Failure 500 {object} apiResponse
// @Router /api/v1/auth/me [get]
func currentUserHandler(c *gin.Context, dbConn *gorm.DB) {
	employeeID := c.GetString("employee_id")
	var user models.User
	if err := dbConn.Where("employee_id = ?", employeeID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

// createEmployeeHandler creates a user and employee profile plus a temporary password.
// CreateEmployee godoc
// @Summary Create an employee
// @Description Creates a new employee profile and generates a temporary password.
// @Tags admin
// @Accept json
// @Produce json
// @Param employeeRequest body employeeRequest true "Employee creation payload"
// @Success 201 {object} apiResponse
// @Failure 400 {object} apiResponse
// @Failure 409 {object} apiResponse
// @Failure 500 {object} apiResponse
// @Router /api/v1/admin/employees [post]
// employeeResponseFromModels combines user account and employee profile records into an API response.
func employeeResponseFromModels(user models.User, profile models.Employee) employeeResponse {
	return employeeResponse{
		EmployeeID: user.EmployeeID, FullName: user.FullName, Email: user.Email, Phone: user.Phone,
		Department: profile.Department, Designation: profile.Designation, Role: user.Role,
		IsFirstLoginRequired: user.IsFirstLoginRequired, PasswordChangedAt: user.PasswordChangedAt,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}
}

// loadEmployee fetches the employee user account and linked employee profile.
func loadEmployee(dbConn *gorm.DB, employeeID string) (models.User, models.Employee, error) {
	var user models.User
	if err := dbConn.Where("employee_id = ? AND role = ?", employeeID, "employee").First(&user).Error; err != nil {
		return user, models.Employee{}, err
	}
	var profile models.Employee
	if err := dbConn.Where("employee_id = ?", employeeID).First(&profile).Error; err != nil {
		return user, profile, err
	}
	return user, profile, nil
}

// listEmployeesHandler returns a paginated, searchable employee directory for administrators.
func listEmployeesHandler(c *gin.Context, dbConn *gorm.DB) {
	search := c.Query("search")
	page := 1
	pageSize := 20
	if parsed, err := strconv.Atoi(c.Query("page")); err == nil && parsed > 0 {
		page = parsed
	}
	if parsed, err := strconv.Atoi(c.Query("page_size")); err == nil && parsed > 0 && parsed <= 100 {
		pageSize = parsed
	}

	query := dbConn.Model(&models.User{}).Where("role = ?", "employee")
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("employee_id ILIKE ? OR full_name ILIKE ? OR email ILIKE ? OR phone ILIKE ?", pattern, pattern, pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "LIST_FAILED", "message": "Unable to list employees."}})
		return
	}

	var users []models.User
	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "LIST_FAILED", "message": "Unable to list employees."}})
		return
	}
	items := make([]employeeResponse, 0, len(users))
	for _, user := range users {
		var profile models.Employee
		if err := dbConn.Where("employee_id = ?", user.EmployeeID).First(&profile).Error; err != nil {
			continue
		}
		items = append(items, employeeResponseFromModels(user, profile))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": employeeListResponse{Items: items, Total: total, Page: page, Size: pageSize}})
}

// getEmployeeHandler returns the account and profile details for one employee.
func getEmployeeHandler(c *gin.Context, dbConn *gorm.DB) {
	user, profile, err := loadEmployee(dbConn, c.Param("employee_id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "EMPLOYEE_NOT_FOUND", "message": "Employee not found."}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "EMPLOYEE_LOOKUP_FAILED", "message": "Unable to load employee."}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": employeeResponseFromModels(user, profile)})
}

// updateEmployeeHandler validates and atomically updates an employee's user and profile records.
func updateEmployeeHandler(c *gin.Context, dbConn *gorm.DB) {
	var payload employeeUpdateRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "INVALID_REQUEST", "message": err.Error()}})
		return
	}
	user, profile, err := loadEmployee(dbConn, c.Param("employee_id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "EMPLOYEE_NOT_FOUND", "message": "Employee not found."}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "EMPLOYEE_LOOKUP_FAILED", "message": "Unable to load employee."}})
		return
	}

	var duplicate models.User
	if err := dbConn.Where("(email = ? OR phone = ?) AND id <> ?", payload.Email, payload.Phone, user.ID).First(&duplicate).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": gin.H{"code": "DUPLICATE_EMPLOYEE", "message": "Email or phone already exists."}})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "UPDATE_FAILED", "message": "Unable to validate employee details."}})
		return
	}

	if err := dbConn.Transaction(func(tx *gorm.DB) error {
		user.FullName, user.Email, user.Phone = payload.FullName, payload.Email, payload.Phone
		profile.Department, profile.Designation = payload.Department, payload.Designation
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		return tx.Save(&profile).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "UPDATE_FAILED", "message": "Unable to update employee."}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": employeeResponseFromModels(user, profile), "message": "Employee updated successfully."})
}

// resetEmployeePasswordHandler generates a temporary password and records an admin reset audit entry.
func resetEmployeePasswordHandler(c *gin.Context, dbConn *gorm.DB) {
	user, _, err := loadEmployee(dbConn, c.Param("employee_id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "EMPLOYEE_NOT_FOUND", "message": "Employee not found."}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "EMPLOYEE_LOOKUP_FAILED", "message": "Unable to load employee."}})
		return
	}
	temporaryPassword, err := auth.GenerateTemporaryPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "PASSWORD_ERROR", "message": "Unable to generate temporary password."}})
		return
	}
	passwordHash, err := auth.HashPassword(temporaryPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "HASH_ERROR", "message": "Unable to secure temporary password."}})
		return
	}
	oldHash := user.PasswordHash
	now := time.Now()
	if err := dbConn.Transaction(func(tx *gorm.DB) error {
		user.PasswordHash, user.IsFirstLoginRequired = passwordHash, true
		user.PasswordChangedAt = nil
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		return tx.Create(&models.PasswordHistory{
			ID: uuid.New(), EmployeeID: user.EmployeeID, ChangedBy: c.GetString("employee_id"),
			OldPasswordHash: oldHash, NewPasswordHash: passwordHash, ChangeType: "admin_reset", ChangedAt: now, CreatedAt: now,
		}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "RESET_FAILED", "message": "Unable to reset employee password."}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"employee_id": user.EmployeeID, "temporary_password": temporaryPassword, "require_password_change": true, "message": "Temporary password generated. The employee must change it on next login."}})
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

// init configures process timezone defaults before server startup.
func init() {
	_ = os.Setenv("TZ", "UTC")
}
