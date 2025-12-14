// Package api provides the REST API for GoSIP
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter creates and configures the API router
func NewRouter(deps *Dependencies) chi.Router {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // TODO: Configure for production
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Initialize handlers
	authHandler := NewAuthHandler(deps)
	deviceHandler := NewDeviceHandler(deps)
	didHandler := NewDIDHandler(deps)
	routeHandler := NewRouteHandler(deps)
	cdrHandler := NewCDRHandler(deps)
	voicemailHandler := NewVoicemailHandler(deps)
	messageHandler := NewMessageHandler(deps)
	systemHandler := NewSystemHandler(deps)
	webhookHandler := NewWebhookHandler(deps)

	// Health endpoints
	healthHandler := NewHealthHandler("0.1.0")
	r.Get("/health", healthHandler.Health)
	r.Get("/api/health", healthHandler.Health)
	r.Get("/api/ready", healthHandler.Ready)
	r.Get("/api/live", healthHandler.Live)

	// Public routes
	r.Route("/api", func(r chi.Router) {
		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authHandler.Login)
			r.Post("/logout", authHandler.Logout)
		})

		// Setup route (only accessible if setup not complete)
		r.Route("/setup", func(r chi.Router) {
			r.Use(SetupOnlyMiddleware(deps.DB))
			r.Get("/status", systemHandler.GetSetupStatus)
			r.Post("/complete", systemHandler.CompleteSetup)
		})

		// Twilio webhooks (secured by Twilio signature validation)
		r.Route("/webhooks", func(r chi.Router) {
			r.Post("/voice/incoming", webhookHandler.VoiceIncoming)
			r.Post("/voice/status", webhookHandler.VoiceStatus)
			r.Post("/sms/incoming", webhookHandler.SMSIncoming)
			r.Post("/sms/status", webhookHandler.SMSStatus)
			r.Post("/recording", webhookHandler.Recording)
			r.Post("/transcription", webhookHandler.Transcription)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(deps))

			// Current user
			r.Get("/me", authHandler.GetCurrentUser)
			r.Put("/me/password", authHandler.ChangePassword)

			// Devices
			r.Route("/devices", func(r chi.Router) {
				r.Get("/", deviceHandler.List)
				r.Post("/", deviceHandler.Create)
				r.Get("/registrations", deviceHandler.GetRegistrations)
				r.Get("/{id}", deviceHandler.Get)
				r.Put("/{id}", deviceHandler.Update)
				r.Delete("/{id}", deviceHandler.Delete)
			})

			// DIDs
			r.Route("/dids", func(r chi.Router) {
				r.Get("/", didHandler.List)
				r.Post("/", didHandler.Create)
				r.Post("/sync", didHandler.SyncFromTwilio)
				r.Get("/{id}", didHandler.Get)
				r.Put("/{id}", didHandler.Update)
				r.Delete("/{id}", didHandler.Delete)
			})

			// Routes
			r.Route("/routes", func(r chi.Router) {
				r.Get("/", routeHandler.List)
				r.Post("/", routeHandler.Create)
				r.Get("/{id}", routeHandler.Get)
				r.Put("/{id}", routeHandler.Update)
				r.Delete("/{id}", routeHandler.Delete)
				r.Put("/reorder", routeHandler.Reorder)
			})

			// CDRs (Call Detail Records)
			r.Route("/cdrs", func(r chi.Router) {
				r.Get("/", cdrHandler.List)
				r.Get("/stats", cdrHandler.GetStats)
				r.Get("/{id}", cdrHandler.Get)
			})

			// Voicemails
			r.Route("/voicemails", func(r chi.Router) {
				r.Get("/", voicemailHandler.List)
				r.Get("/unread", voicemailHandler.ListUnread)
				r.Get("/{id}", voicemailHandler.Get)
				r.Put("/{id}/read", voicemailHandler.MarkAsRead)
				r.Delete("/{id}", voicemailHandler.Delete)
			})

			// Messages
			r.Route("/messages", func(r chi.Router) {
				r.Get("/", messageHandler.List)
				r.Post("/", messageHandler.Send)
				r.Get("/conversations", messageHandler.GetConversations)
				r.Get("/conversation/{number}", messageHandler.GetConversation)
				r.Get("/{id}", messageHandler.Get)
				r.Put("/{id}/read", messageHandler.MarkAsRead)
				r.Delete("/{id}", messageHandler.Delete)
			})

			// Blocklist
			r.Route("/blocklist", func(r chi.Router) {
				r.Get("/", routeHandler.ListBlocklist)
				r.Post("/", routeHandler.AddToBlocklist)
				r.Delete("/{id}", routeHandler.RemoveFromBlocklist)
			})

			// Admin-only routes
			r.Group(func(r chi.Router) {
				r.Use(AdminOnlyMiddleware)

				// Users management
				r.Route("/users", func(r chi.Router) {
					r.Get("/", authHandler.ListUsers)
					r.Post("/", authHandler.CreateUser)
					r.Get("/{id}", authHandler.GetUser)
					r.Put("/{id}", authHandler.UpdateUser)
					r.Delete("/{id}", authHandler.DeleteUser)
				})

				// System configuration
				r.Route("/system", func(r chi.Router) {
					r.Get("/config", systemHandler.GetConfig)
					r.Put("/config", systemHandler.UpdateConfig)
					r.Post("/backup", systemHandler.CreateBackup)
					r.Post("/restore", systemHandler.RestoreBackup)
					r.Get("/status", systemHandler.GetStatus)
				})

				// DND toggle
				r.Put("/dnd", systemHandler.ToggleDND)
			})
		})
	})

	// Serve frontend static files
	r.Handle("/*", http.FileServer(http.Dir("./frontend/dist")))

	return r
}
