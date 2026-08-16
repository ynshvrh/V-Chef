package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ynshvrh/V-Chef/internal/chef"
	"github.com/ynshvrh/V-Chef/internal/models"
	pb "github.com/ynshvrh/V-Chef/proto/v1"
)

type GrpcServer struct {
	pb.UnimplementedChefServiceServer
	chefService chef.Generator
}

func NewGrpcServer(chefService chef.Generator) *GrpcServer {
	return &GrpcServer{
		chefService: chefService,
	}
}

func (s *GrpcServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{Status: "ok"}, nil
}

func (s *GrpcServer) GenerateRecipe(ctx context.Context, req *pb.GenerateRecipeRequest) (*pb.GenerateRecipeResponse, error) {
	if len(req.GetIngredients()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "ingredients list cannot be empty")
	}

	modelReq := models.GenerateRecipeRequest{
		Ingredients:     req.GetIngredients(),
		MealType:        req.GetMealType(),
		DietaryCategory: req.GetDietaryCategory(),
		MaxPrepTimeMins: int(req.GetMaxPrepTimeMins()),
		TargetCalories:  int(req.GetTargetCalories()),
	}

	recipe, err := s.chefService.GenerateRecipe(ctx, modelReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate recipe: %v", err)
	}

	ingredients := make([]*pb.RecipeIngredient, 0, len(recipe.Ingredients))
	for _, ing := range recipe.Ingredients {
		ingredients = append(ingredients, &pb.RecipeIngredient{
			Name:     ing.Name,
			Quantity: ing.Quantity,
			Unit:     ing.Unit,
			InFridge: ing.InFridge,
		})
	}

	return &pb.GenerateRecipeResponse{
		Title:        recipe.Title,
		Description:  recipe.Description,
		PrepTimeMins: int32(recipe.PrepTimeMins),
		CookTimeMins: int32(recipe.CookTimeMins),
		Servings:     int32(recipe.Servings),
		Calories:     int32(recipe.Calories),
		ProteinGrams: recipe.ProteinGrams,
		FatGrams:     recipe.FatGrams,
		CarbsGrams:   recipe.CarbsGrams,
		Ingredients:  ingredients,
		Steps:        recipe.Steps,
		GeneratedAt:  recipe.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// UnaryAuthInterceptor validates X-Internal-Token metadata header for gRPC requests
func UnaryAuthInterceptor(expectedToken string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == "/vchef.v1.ChefService/HealthCheck" || expectedToken == "" {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("x-internal-token")
		if len(tokens) == 0 || tokens[0] != expectedToken {
			return nil, status.Error(codes.Unauthenticated, "invalid or missing internal service token")
		}

		return handler(ctx, req)
	}
}
