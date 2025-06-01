package controllers

import (
	"aquahome/database"
)

// User role constants
const (
	RoleAdmin          = database.RoleAdmin
	RoleCustomer       = database.RoleCustomer
	RoleFranchiseOwner = database.RoleFranchiseOwner
	RoleServiceAgent   = database.RoleServiceAgent
)

// Order status constants
const (
	OrderStatusPending   = database.OrderStatusPending
	OrderStatusConfirmed = database.OrderStatusConfirmed
	OrderStatusApproved  = database.OrderStatusApproved
	OrderStatusRejected  = database.OrderStatusRejected
	OrderStatusInTransit = database.OrderStatusInTransit
	OrderStatusDelivered = database.OrderStatusDelivered
	OrderStatusInstalled = database.OrderStatusInstalled
	OrderStatusCancelled = database.OrderStatusCancelled
	OrderStatusCompleted = database.OrderStatusCompleted

	// Define missing constants used in old code
	OrderStatusProcessing = "processing"
	OrderStatusShipped    = "shipped"
)

// Subscription status constants
const (
	SubscriptionStatusActive    = database.SubscriptionStatusActive
	SubscriptionStatusPaused    = database.SubscriptionStatusPaused
	SubscriptionStatusCancelled = database.SubscriptionStatusCancelled
	SubscriptionStatusExpired   = database.SubscriptionStatusExpired

	// Define missing constants used in old code
	SubscriptionStatusInactive = "inactive"
)

// Service request status constants
const (
	ServiceStatusPending    = database.ServiceStatusPending
	ServiceStatusAssigned   = database.ServiceStatusAssigned
	ServiceStatusScheduled  = database.ServiceStatusScheduled
	ServiceStatusInProgress = database.ServiceStatusInProgress
	ServiceStatusCompleted  = database.ServiceStatusCompleted
	ServiceStatusCancelled  = database.ServiceStatusCancelled
)

// Payment status constants
const (
	PaymentStatusPending  = database.PaymentStatusPending
	PaymentStatusPaid     = database.PaymentStatusPaid
	PaymentStatusSuccess  = database.PaymentStatusSuccess
	PaymentStatusFailed   = database.PaymentStatusFailed
	PaymentStatusRefunded = database.PaymentStatusRefunded
)
