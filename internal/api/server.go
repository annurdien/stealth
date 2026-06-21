package api

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"github.com/annurdien/stealth/internal/models"
	"github.com/annurdien/stealth/internal/session"
	"github.com/annurdien/stealth/internal/solver"
)

// NewServer creates and configures the Fiber application with all routes
// and middleware. It does not start listening — call app.Listen() separately.
func NewServer(sm *session.Manager, version string) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "Stealth",
		WriteTimeout: 120 * time.Second,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: globalErrorHandler,
	})

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	app.Use(requestid.New())

	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} ${path}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
	app.Get("/", indexHandler(version))
	app.Get("/health", healthHandler)

	v2 := app.Group("/v2")
	v2.Post("/request", requestHandler(sm))
	v2.Post("/sessions", sessionCreateHandler(sm))
	v2.Get("/sessions", sessionListHandler(sm))
	v2.Delete("/sessions/:id", sessionDestroyHandler(sm))

	return app
}

// globalErrorHandler converts unhandled errors into consistent JSON responses.
func globalErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{
		"status":  "error",
		"message": err.Error(),
	})
}

// indexHandler returns the service index (version + userAgent).
func indexHandler(version string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(models.IndexResponse{
			Msg:     "Stealth is ready!",
			Version: version,
		})
	}
}

// healthHandler returns a simple health check response.
// This endpoint is intentionally minimal and fast — used by Docker/k8s probes.
func healthHandler(c *fiber.Ctx) error {
	return c.JSON(models.HealthResponse{Status: "ok"})
}

// requestHandler handles POST /v2/request — the core solve endpoint.
func requestHandler(sm *session.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.V2Request
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.V2Response{
				Status:  "error",
				Message: "Invalid JSON body: " + err.Error(),
				Version: solver.Version,
			})
		}

		if req.URL == "" {
			return c.Status(fiber.StatusBadRequest).JSON(models.V2Response{
				Status:  "error",
				Message: "Field 'url' is required",
				Version: solver.Version,
			})
		}

		startTs := time.Now().UnixMilli()

		var sess *session.SessionContext
		var isEphemeral bool

		if req.Session != "" {
			existing, exists := sm.Get(req.Session)
			if exists {
				sess = existing
			} else {
				var err error
				sess, err = sm.Create(&models.SessionCreateRequest{
					Session: req.Session,
					Proxy:   req.Proxy,
				})
				if err != nil {
					return c.JSON(&models.V2Response{
						Status:         "error",
						Message:        "Error creating session: " + err.Error(),
						StartTimestamp: startTs,
						EndTimestamp:   time.Now().UnixMilli(),
						Version:        solver.Version,
					})
				}
			}
		} else {
			isEphemeral = true
			var err error
			sess, err = sm.Create(&models.SessionCreateRequest{
				Proxy: req.Proxy,
			})
			if err != nil {
				return c.JSON(&models.V2Response{
					Status:         "error",
					Message:        "Error launching browser: " + err.Error(),
					StartTimestamp: startTs,
					EndTimestamp:   time.Now().UnixMilli(),
					Version:        solver.Version,
				})
			}
		}

		if isEphemeral {
			defer func() {
				log.Printf("[request] destroying ephemeral session %s", sess.ID)
				sm.Destroy(sess.ID)
			}()
		}

		sess.Lock()
		defer sess.Unlock()

		resp, err := solver.Solve(context.Background(), sess.Page, &req)
		if err != nil {
			return c.JSON(&models.V2Response{
				Status:         "error",
				Message:        "Unexpected error: " + err.Error(),
				StartTimestamp: startTs,
				EndTimestamp:   time.Now().UnixMilli(),
				Version:        solver.Version,
			})
		}

		return c.JSON(resp)
	}
}

// sessionCreateHandler handles POST /v2/sessions.
func sessionCreateHandler(sm *session.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req models.SessionCreateRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(models.SessionResponse{
				Status:  "error",
				Message: "Invalid JSON body: " + err.Error(),
			})
		}

		sess, err := sm.Create(&req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.SessionResponse{
				Status:  "error",
				Message: "Error: " + err.Error(),
			})
		}

		return c.JSON(models.SessionResponse{
			Status:  "ok",
			Message: "Session created successfully.",
			Session: sess.ID,
		})
	}
}

// sessionListHandler handles GET /v2/sessions.
func sessionListHandler(sm *session.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(models.SessionResponse{
			Status:   "ok",
			Message:  "",
			Sessions: sm.List(),
		})
	}
}

// sessionDestroyHandler handles DELETE /v2/sessions/:id.
func sessionDestroyHandler(sm *session.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Params("id")
		if !sm.Destroy(id) {
			return c.Status(fiber.StatusNotFound).JSON(models.SessionResponse{
				Status:  "error",
				Message: "Session not found.",
			})
		}
		return c.JSON(models.SessionResponse{
			Status:  "ok",
			Message: "Session destroyed successfully.",
		})
	}
}
