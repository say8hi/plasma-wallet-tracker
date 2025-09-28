package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/say8hi/plasma-wallet-tracker/config"
	"github.com/say8hi/plasma-wallet-tracker/internal/infrastructure/blockchain"
	"github.com/say8hi/plasma-wallet-tracker/internal/infrastructure/redis"
	"github.com/say8hi/plasma-wallet-tracker/internal/usecase"

	"go.uber.org/zap"
)

func main() {
	// Load configuration first
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize logger based on config
	var logger *zap.Logger
	if cfg.Log.Level == "debug" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	// Initialize Redis client
	redisClient := redis.NewClient(cfg.Redis)

	// Test Redis connection
	if err := redisClient.Ping(context.Background()); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	// Initialize blockchain client
	blockchainClient, err := blockchain.NewPlasmaClient(cfg.Blockchain)
	if err != nil {
		logger.Fatal("Failed to initialize blockchain client", zap.Error(err))
	}

	// Initialize Redis publisher/subscriber
	publisher := redis.NewPublisher(redisClient, logger)
	subscriber := redis.NewSubscriber(redisClient, logger)

	// Initialize wallet tracker service
	walletTracker := usecase.NewWalletTracker(
		blockchainClient,
		publisher,
		logger,
	)

	// Initialize command handler
	commandHandler := usecase.NewCommandHandler(walletTracker, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start HTTP server for health checks
	go startHTTPServer(logger, redisClient, blockchainClient)

	// Start command subscriber
	go subscriber.SubscribeCommands(ctx, commandHandler.HandleCommand)

	// Start wallet tracker
	go walletTracker.Start(ctx)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down gracefully...")
	cancel()
}

func startHTTPServer(
	logger *zap.Logger,
	redisClient *redis.Client,
	blockchainClient *blockchain.PlasmaClient,
) {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		healthCheck(w, r, logger, redisClient, blockchainClient)
	})

	// Readiness check endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		readinessCheck(w, r, logger, redisClient, blockchainClient)
	})

	// Metrics endpoint (placeholder)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Metrics placeholder\n"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	logger.Info("Starting HTTP server on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("HTTP server failed", zap.Error(err))
	}
}

func healthCheck(
	w http.ResponseWriter,
	r *http.Request,
	logger *zap.Logger,
	redisClient *redis.Client,
	blockchainClient *blockchain.PlasmaClient,
) {
	w.Header().Set("Content-Type", "application/json")

	// Check Redis connection
	if err := redisClient.Ping(r.Context()); err != nil {
		logger.Error("Health check failed: Redis unavailable", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","error":"redis_unavailable"}`))
		return
	}

	// Check blockchain connection
	if _, err := blockchainClient.GetLatestBlock(r.Context()); err != nil {
		logger.Error("Health check failed: Blockchain unavailable", zap.Error(err))
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy","error":"blockchain_unavailable"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

func readinessCheck(
	w http.ResponseWriter,
	r *http.Request,
	logger *zap.Logger,
	redisClient *redis.Client,
	blockchainClient *blockchain.PlasmaClient,
) {
	w.Header().Set("Content-Type", "application/json")

	// Similar to health check but can include more comprehensive checks
	healthCheck(w, r, logger, redisClient, blockchainClient)
}
