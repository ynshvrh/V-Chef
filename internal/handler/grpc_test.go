package handler

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/ynshvrh/V-Chef/internal/chef"
	"github.com/ynshvrh/V-Chef/internal/config"
	pb "github.com/ynshvrh/V-Chef/proto/v1"
)

const bufSize = 1024 * 1024

func setupGrpcServer(token string) (pb.ChefServiceClient, func()) {
	lis := bufconn.Listen(bufSize)
	cfg := &config.Config{Port: "8085", InternalToken: token}
	chefSvc := chef.NewService(cfg)
	grpcSvc := NewGrpcServer(chefSvc)

	var opts []grpc.ServerOption
	if token != "" {
		opts = append(opts, grpc.UnaryInterceptor(UnaryAuthInterceptor(token)))
	}

	s := grpc.NewServer(opts...)
	pb.RegisterChefServiceServer(s, grpcSvc)

	go func() {
		_ = s.Serve(lis)
	}()

	conn, _ := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	client := pb.NewChefServiceClient(conn)

	cleanup := func() {
		conn.Close()
		s.Stop()
		lis.Close()
	}

	return client, cleanup
}

func TestGrpcHealthCheck(t *testing.T) {
	client, cleanup := setupGrpcServer("test-secret-456")
	defer cleanup()

	resp, err := client.HealthCheck(context.Background(), &pb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if resp.GetStatus() != "ok" {
		t.Errorf("expected status 'ok', got '%s'", resp.GetStatus())
	}
}

func TestGrpcGenerateRecipeAuth(t *testing.T) {
	const secret = "test-secret-456"
	client, cleanup := setupGrpcServer(secret)
	defer cleanup()

	req := &pb.GenerateRecipeRequest{
		Ingredients: []string{"egg", "cheese"},
		MealType:    "dinner",
	}

	// 1. Without token metadata -> expect error
	t.Run("Without token", func(t *testing.T) {
		_, err := client.GenerateRecipe(context.Background(), req)
		if err == nil {
			t.Error("expected error for request without token, got nil")
		}
	})

	// 2. With valid token metadata -> expect success
	t.Run("With valid token", func(t *testing.T) {
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-internal-token", secret))
		resp, err := client.GenerateRecipe(ctx, req)
		if err != nil {
			t.Fatalf("GenerateRecipe failed with valid token: %v", err)
		}

		if resp.GetTitle() == "" {
			t.Error("expected non-empty title in response")
		}
	})
}
