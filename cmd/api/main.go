package main

import (
	"context"
	"log"
	"time"

	"entradasya/ms-payments/internal/config"
	"entradasya/ms-payments/internal/database"
	"entradasya/ms-payments/internal/handler"
	"entradasya/ms-payments/internal/middleware"
	"entradasya/ms-payments/internal/repository"
	"entradasya/ms-payments/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("[ms-payments] Iniciando microservicio en puerto :%s", cfg.Port)

	// Conexión a PostgreSQL (payments_db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dbPinger database.Pinger
	var repo repository.PaymentRepository

	db, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("[ms-payments] ADVERTENCIA: No se pudo conectar a la base de datos de inmediato: %v", err)
	} else {
		defer db.Close()
		dbPinger = db
		repo = repository.NewPostgresPaymentRepository(db.Pool)
		log.Printf("[ms-payments] Conexión a PostgreSQL establecida exitosamente")
	}

	// Si no hay repo por base de datos caída, creamos un fallback mock seguro para no entrar en pánico
	if repo == nil {
		log.Printf("[ms-payments] ADVERTENCIA: Usando repositorio en memoria como fallback")
		repo = repository.NewMockPaymentRepository()
	}

	// Inicialización de capas
	paymentSvc := service.NewPaymentService(repo)
	paymentHdl := handler.NewPaymentHandler(paymentSvc)
	healthHdl := handler.NewHealthHandler(dbPinger)

	// Configuración del router Gin
	r := gin.Default()

	// Probes de salud (públicos para orquestador y Docker/K8s)
	r.GET("/health", healthHdl.Live)
	r.GET("/health/live", healthHdl.Live)
	r.GET("/health/ready", healthHdl.Ready)

	// Rutas operativas protegidas con X-Internal-Token
	api := r.Group("/api/v1")
	api.Use(middleware.InternalAuth(cfg.InternalToken))
	{
		api.POST("/payments", paymentHdl.ProcessPayment)
		api.GET("/payments/order/:orderId", paymentHdl.GetPaymentByOrderID)
	}

	log.Printf("[ms-payments] Servidor escuchando en :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("[ms-payments] Error fatal al iniciar servidor HTTP: %v", err)
	}
}
