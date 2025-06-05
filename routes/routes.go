package routes

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"aquahome/controllers"
	"aquahome/middleware"
)

// SetupRoutes configures all application routes
func SetupRoutes(r *gin.Engine) {
	fmt.Println("✅ SetupRoutes called")

	// Public routes (no authentication required)
	public := r.Group("/api")
	{
		// Authentication routes
		auth := public.Group("/auth")
		{
			auth.POST("/login", controllers.Login)
			auth.POST("/register", controllers.Register)
			auth.POST("/login/v2", controllers.LoginNew)
			auth.POST("/register/v2", controllers.RegisterNew)
		}

		// Products (public view for non-authenticated users)

	}

	// Protected routes (authentication required)
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	{

		protected.POST("/auth/refresh", controllers.RefreshToken)
		protected.POST("/auth/refresh/v2", controllers.RefreshTokenNew)

		protected.GET("/profile", controllers.GetUserProfile)
		protected.PUT("/profile", controllers.UpdateUserProfile)
		protected.POST("/profile/change-password", controllers.ChangePassword)
		protected.GET("/profile/v2", controllers.GetUserProfileNew)
		protected.GET("/customer/products", controllers.GetCustomerProducts)
		protected.PUT("/profile/v2", controllers.UpdateUserProfileNew)
		protected.POST("/profile/location", controllers.UpdateUserLocation)
		protected.POST("/profile/change-password/v2", controllers.ChangePasswordNew)
		protected.PATCH("/servicerequests/:id/assign-agent", middleware.AdminOrFranchiseAuthMiddleware(), controllers.AssignServiceRequestToAgent)

		// protected.POST("/customer/service-requests",controllers.CreateServiceRequest)

		// Admin routes
		// Admin routes
		admin := protected.Group("/admin")
		admin.Use(middleware.AdminAuthMiddleware())
		{
			admin.GET("/users/:id", controllers.GetUserByID)
			admin.GET("/users/role/:role", controllers.GetUsersByRole)
			admin.GET("/orders", controllers.AdminGetOrders)
			admin.GET("/users/:id/v2", controllers.GetUserByIDNew)
			admin.GET("/users/role/:role/v2", controllers.GetUsersByRoleNew)
			admin.GET("/dashboard", controllers.AdminDashboard)

			//  Products Management
			admin.POST("/products", controllers.CreateProduct)
			admin.GET("/products", controllers.GetProducts)
			admin.GET("/products/:id", controllers.GetProductByID)
			admin.PUT("/products/:id", controllers.UpdateProduct)
			admin.DELETE("/products/:id", controllers.DeleteProduct)
			admin.PATCH("/products/:id/toggle-status", controllers.ToggleProductStatus)

			//  Franchise Management
			admin.PATCH("/franchises/:id", controllers.AdminUpdateFranchise)
			admin.POST("/franchises", controllers.CreateFranchise)
			admin.GET("/franchises", controllers.GetAllFranchises)
			admin.PATCH("/franchises/:id/toggle-status", controllers.ToggleFranchiseStatus)

			//  Orders
			admin.PATCH("/orders/:id/assign", controllers.AssignOrderToFranchise)
			admin.GET("/customers/:id/subscriptions", controllers.GetCustomerSubscriptionsByAdmin)

			// NEW: Locations
			admin.GET("/locations", controllers.GetAllLocations)
		}

		// 🧑‍🔧 Service Agent Routes
		agent := protected.Group("/agent")
		agent.Use(middleware.ServiceAgentAuthMiddleware())
		{
			agent.GET("/tasks", controllers.GetAgentTasks)
			agent.GET("/dashboard", controllers.GetServiceAgentDashboard)
			agent.GET("/orders", controllers.GetAgentOrders)
		}

		// Orders
		orders := protected.Group("/orders")
		{
			fmt.Println("✅ Orders route group initializing")

			orders.POST("", middleware.CustomerAuthMiddleware(), controllers.CreateOrder)
			orders.POST("/:id/cancel", middleware.CustomerAuthMiddleware(), controllers.CancelOrder)
			orders.GET("/customer", middleware.CustomerAuthMiddleware(), controllers.GetCustomerOrders)
			orders.PUT("/:id/status", middleware.AdminOrFranchiseAuthMiddleware(), controllers.UpdateOrderStatus)
			orders.GET("/:id", controllers.GetOrderByID)

			orders.PATCH("/:id/assign-agent", middleware.FranchiseOwnerAuthMiddleware(), controllers.AssignOrderToAgent)

		}

		// Subscriptions
		subscriptions := protected.Group("/subscriptions")
		{
			subscriptions.POST("", middleware.CustomerAuthMiddleware(), controllers.CreateSubscription)
			subscriptions.GET("/customer", middleware.CustomerAuthMiddleware(), controllers.GetMySubscriptions)
			subscriptions.PUT("/:id", middleware.CustomerAuthMiddleware(), controllers.UpdateSubscription)
			subscriptions.POST("/:id/cancel", middleware.CustomerAuthMiddleware(), controllers.CancelSubscription)

			subscriptions.GET("/franchise", middleware.FranchiseOwnerAuthMiddleware(), controllers.GetFranchiseSubscriptions)

		}

		// Service requests
		services := protected.Group("/services")
		{
			services.POST("", middleware.CustomerAuthMiddleware(), controllers.CreateServiceRequest)
			services.POST("/:id/feedback", middleware.CustomerAuthMiddleware(), controllers.SubmitServiceFeedback)
			services.POST("/:id/cancel", middleware.CustomerAuthMiddleware(), controllers.CancelServiceRequest)
			services.GET("", controllers.GetServiceRequestsNew)
			services.GET("/:id", controllers.GetServiceRequestByIDNew)
			services.PUT("/:id", controllers.UpdateServiceRequestNew)

		}

		// Franchises
		franchises := protected.Group("/franchises")
		franchises.Use(middleware.FranchiseOwnerAuthMiddleware())
		{
			franchises.POST("", middleware.FranchiseOwnerAuthMiddleware(), controllers.CreateFranchise)
			franchises.POST("/:id/approve", middleware.AdminAuthMiddleware(), controllers.ApproveFranchise)
			franchises.POST("/:id/reject", middleware.AdminAuthMiddleware(), controllers.RejectFranchise)
			franchises.PUT("/:id", middleware.AdminOrFranchiseAuthMiddleware(), controllers.UpdateFranchise)
			franchises.GET("/:id/service-agents", middleware.AdminOrFranchiseAuthMiddleware(), controllers.GetFranchiseServiceAgents)
			franchises.GET("/search", controllers.SearchFranchises)
			franchises.POST("/locations", controllers.AddFranchiseLocations)
			franchises.PUT("/:id/locations", controllers.UpdateFranchiseLocations)
			franchises.GET("/locations", controllers.GetMyLocations)

			//this route for dashboard
			franchises.GET("/dashboard", controllers.GetFranchiseDashboard)

			// ✅ Orders for franchise owner
			franchises.GET("/orders", controllers.AdminGetOrders)

			// ✅ Assign service agent to order (already supports franchise_owner in controller)
			franchises.PATCH("/orders/:id/assign-agent", controllers.AssignOrderToAgent)
			franchises.GET("/service-agents", controllers.GetServiceAgentsForFranchise)

		}

		// Payments
		payments := protected.Group("/payments")
		{
			payments.POST("/generate-order", middleware.CustomerAuthMiddleware(), controllers.GeneratePaymentOrder)
			payments.POST("/generate-monthly", middleware.CustomerAuthMiddleware(), controllers.GenerateMonthlyPayment)
			payments.POST("/verify", middleware.CustomerAuthMiddleware(), controllers.VerifyPayment)
			payments.GET("", controllers.GetPaymentHistory)
			payments.GET("/:id", controllers.GetPaymentByID)
		}

		// Add this route for franchise dashboard
		protected.GET("/franchise/dashboard", controllers.GetFranchiseDashboard)
	}
}
