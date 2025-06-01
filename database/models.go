package database

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	gorm.Model
	Name         string `json:"name"`
	Email        string `json:"email"`
	Password     string `json:"-"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	FranchiseID  *uint  `json:"franchise_id"`
	Phone        string `json:"phone"`
	Address      string `json:"address"`
	City         string `json:"city"`
	State        string `json:"state"`
	ZipCode      string `json:"zip_code"`
	// models/user.go
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Product represents a water purifier product
type Product struct {
	gorm.Model
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	MonthlyRent      float64   `json:"monthly_rent"`
	SecurityDeposit  float64   `json:"security_deposit"`
	InstallationFee  float64   `json:"installation_fee"`
	ImageURL         string    `json:"image_url"`
	Features         string    `json:"features"`
	Specifications   string    `json:"specifications"`
	AvailableStock   int       `json:"available_stock"`
	MaintenanceCycle int       `json:"maintenance_cycle"`
	IsActive         bool      `json:"is_active" gorm:"column:is_active"` // ED THIS
	FranchiseID      uint      `json:"franchise_id"`                      // ✅ NEW
	Franchise        Franchise `gorm:"foreignKey:FranchiseID" json:"franchise"`
}

// Franchise repreents a franchise location
type Franchise struct {
	gorm.Model
	OwnerID        uint    `json:"owner_id"`
	Name           string  `json:"name"`
	Address        string  `json:"address"`
	City           string  `json:"city"`
	State          string  `json:"state"`
	ZipCode        string  `json:"zip_code"`
	Phone          string  `json:"phone"`
	Email          string  `json:"email"`
	IsActive       bool    `json:"is_active"`
	ServiceArea    string  `json:"service_area"`
	CoverageRadius float64 `json:"coverage_radius"`
	ApprovalState  string  `json:"approval_state"`

	Owner User `gorm:"foreignKey:OwnerID" json:"owner"`

	// 🆕 ADD THIS LINE:
	Locations []Location `gorm:"many2many:franchise_locations;" json:"locations"`
}

// Location represents a serviceable ZIP area
type Location struct {
	gorm.Model
	Name       string         `json:"name"`
	ZipCodes   pq.StringArray `gorm:"type:text[]" json:"zip_codes"` // comma-separated ZIPs
	IsActive   bool           `json:"is_active"`
	Franchises []Franchise    `gorm:"many2many:franchise_locations;" json:"franchises"`
}

// FranchiseLocation is the join table for many-to-many Franchise ↔ Location
type FranchiseLocation struct {
	ID          uint `gorm:"primaryKey"`
	FranchiseID uint
	LocationID  uint
}

// Order represents a customer order
type Order struct {
	gorm.Model
	// ID                 uint      `json:"id"`
	CustomerID         uint      `json:"customer_id"`
	ProductID          uint      `json:"product_id"`
	FranchiseID        uint      `json:"franchise_id"`
	OrderType          string    `json:"order_type"`
	ServiceAgentID     *uint     `json:"service_agent_id"`
	Status             string    `json:"status"`
	ShippingAddress    string    `json:"shipping_address"`
	BillingAddress     string    `json:"billing_address"`
	RentalStartDate    time.Time `json:"rental_start_date"`
	RentalDuration     int       `json:"rental_duration"`
	MonthlyRent        float64   `json:"monthly_rent"`
	DeliveryDate       time.Time `json:"delivery_date"`
	SecurityDeposit    float64   `json:"security_deposit"`
	InstallationFee    float64   `json:"installation_fee"`
	TotalInitialAmount float64   `json:"total_initial_amount"`
	Notes              string    `json:"notes"`
	Customer           User      `gorm:"foreignKey:CustomerID" json:"customer"`
	Product            Product   `gorm:"foreignKey:ProductID" json:"product"`
	Franchise          Franchise `gorm:"foreignKey:FranchiseID" json:"franchise"`
	ServiceAgent       *User     `gorm:"foreignKey:ServiceAgentID" json:"service_agent"`
}

// Subscription represents an active rental subscription
type Subscription struct {
	gorm.Model
	OrderID          uint      `json:"order_id"`
	CustomerID       uint      `json:"customer_id"`
	ProductID        uint      `json:"product_id"`
	FranchiseID      uint      `json:"franchise_id"`
	ServiceAgentID   *uint     `json:"service_agent_id"`
	Status           string    `json:"status"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	NextBillingDate  time.Time `json:"next_billing_date"`
	MonthlyRent      float64   `json:"monthly_rent"`
	LastMaintenance  time.Time `json:"last_maintenance"`
	NextMaintenance  time.Time `json:"next_maintenance"`
	MaintenanceNotes string    `json:"maintenance_notes"`
	Notes            string    `json:"notes"`
	Order            Order     `gorm:"foreignKey:OrderID" json:"order"`
	Customer         User      `gorm:"foreignKey:CustomerID" json:"customer"`
	Product          Product   `gorm:"foreignKey:ProductID" json:"product"`
	Franchise        Franchise `gorm:"foreignKey:FranchiseID" json:"franchise"`
	ServiceAgent     *User     `gorm:"foreignKey:ServiceAgentID" json:"service_agent"`
}

// Payment represents a payment made in the system
type Payment struct {
	gorm.Model
	CustomerID     uint          `json:"customer_id"`
	OrderID        *uint         `json:"order_id"`
	SubscriptionID *uint         `json:"subscription_id"`
	Amount         float64       `json:"amount"`
	PaymentType    string        `json:"payment_type"`
	Status         string        `json:"status"`
	InvoiceNumber  string        `json:"invoice_number"`
	PaymentMethod  string        `json:"payment_method"`
	TransactionID  string        `json:"transaction_id"`
	PaymentDetails string        `json:"payment_details"`
	Notes          string        `json:"notes"`
	Customer       User          `gorm:"foreignKey:CustomerID" json:"customer"`
	Order          *Order        `gorm:"foreignKey:OrderID" json:"order"`
	Subscription   *Subscription `gorm:"foreignKey:SubscriptionID" json:"subscription"`
}

// ServiceRequest represents a maintenance/service request
type ServiceRequest struct {
	gorm.Model
	CustomerID     uint         `json:"customer_id"`
	SubscriptionID uint         `json:"subscription_id"`
	FranchiseID    uint         `json:"franchise_id"` // ✅ ADD THIS LINE
	ServiceAgentID *uint        `json:"service_agent_id"`
	Type           string       `json:"type"`
	Status         string       `json:"status"`
	Description    string       `json:"description"`
	ScheduledTime  *time.Time   `json:"scheduled_time"`
	CompletionTime *time.Time   `json:"completion_time"`
	Notes          string       `json:"notes"`
	Rating         *int         `json:"rating"`
	Feedback       string       `json:"feedback"`
	Customer       User         `gorm:"foreignKey:CustomerID" json:"customer"`
	Subscription   Subscription `gorm:"foreignKey:SubscriptionID" json:"subscription"`
	ServiceAgent   *User        `gorm:"foreignKey:ServiceAgentID" json:"service_agent"`
}

// Notification represents a system notification
type Notification struct {
	gorm.Model
	UserID      uint   `json:"user_id"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Type        string `json:"type"`
	RelatedID   *uint  `json:"related_id"`
	RelatedType string `json:"related_type"`
	IsRead      bool   `json:"is_read"`
	User        User   `gorm:"foreignKey:UserID" json:"user"`
}

// PasswordReset represents a password reset request
type PasswordReset struct {
	gorm.Model
	UserID    uint      `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `gorm:"foreignKey:UserID" json:"user"`
}

// Audit represents a system audit log entry
type Audit struct {
	gorm.Model
	UserID     *uint  `json:"user_id"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   uint   `json:"entity_id"`
	OldValue   string `json:"old_value"`
	NewValue   string `json:"new_value"`
	IPAddress  string `json:"ip_address"`
	UserAgent  string `json:"user_agent"`
	User       *User  `gorm:"foreignKey:UserID" json:"user"`
}

// Constants for status values
const (
	OrderStatusPending   = "pending"
	OrderStatusConfirmed = "confirmed"
	OrderStatusApproved  = "approved"
	OrderStatusRejected  = "rejected"
	OrderStatusInTransit = "in_transit"
	OrderStatusDelivered = "delivered"
	OrderStatusInstalled = "installed"
	OrderStatusCancelled = "cancelled"
	OrderStatusCompleted = "completed"

	SubscriptionStatusActive    = "active"
	SubscriptionStatusPaused    = "paused"
	SubscriptionStatusCancelled = "cancelled"
	SubscriptionStatusExpired   = "expired"

	ServiceStatusPending    = "pending"
	ServiceStatusAssigned   = "assigned"
	ServiceStatusScheduled  = "scheduled"
	ServiceStatusInProgress = "in_progress"
	ServiceStatusCompleted  = "completed"
	ServiceStatusCancelled  = "cancelled"

	PaymentStatusPending  = "pending"
	PaymentStatusPaid     = "paid"
	PaymentStatusSuccess  = "success"
	PaymentStatusFailed   = "failed"
	PaymentStatusRefunded = "refunded"

	// User roles
	RoleAdmin          = "admin"
	RoleFranchiseOwner = "franchise_owner"
	RoleServiceAgent   = "service_agent"
	RoleCustomer       = "customer"
)
