package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/ynshvrh/V-Chef/internal/chef"
	"github.com/ynshvrh/V-Chef/internal/config"
	"github.com/ynshvrh/V-Chef/internal/handler"
	pb "github.com/ynshvrh/V-Chef/proto/v1"
)

func main() {
	cfg := config.Load()

	chefService := chef.NewService(cfg)
	recipeHandler := handler.NewRecipeHandler(chefService)
	router := handler.NewRouter(recipeHandler, cfg.InternalToken)

	// 1. Start HTTP REST Server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("👨‍🍳 V-Chef REST API listening on http://localhost:%s (Env: %s)", cfg.Port, cfg.Environment)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed to start: %v", err)
		}
	}()

	// 2. Start gRPC Server
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.GrpcPort))
	if err != nil {
		log.Printf("⚠️ Failed to listen on gRPC port %s: %v", cfg.GrpcPort, err)
	} else {
		var grpcOpts []grpc.ServerOption
		if cfg.InternalToken != "" {
			grpcOpts = append(grpcOpts, grpc.UnaryInterceptor(handler.UnaryAuthInterceptor(cfg.InternalToken)))
		}

		grpcServer := grpc.NewServer(grpcOpts...)
		grpcHandler := handler.NewGrpcServer(chefService)
		pb.RegisterChefServiceServer(grpcServer, grpcHandler)

		go func() {
			log.Printf("📡 V-Chef gRPC endpoint listening on :%s", cfg.GrpcPort)
			if err := grpcServer.Serve(grpcListener); err != nil {
				log.Printf("gRPC server exited: %v", err)
			}
		}()

		defer grpcServer.GracefulStop()
	}

	// Graceful shutdown on SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	log.Println("Shutting down V-Chef microservice...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP Server forced shutdown error: %v", err)
	}

	log.Println("V-Chef microservice stopped cleanly.")
}
