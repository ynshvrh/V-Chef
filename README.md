# V-Chef

High-performance AI culinary microservice for the V-Fridge ecosystem built with Go.

V-Chef provides structured recipe generation from available fridge ingredients, meal types, preparation time limits, and dietary/macro constraints. It exposes both HTTP REST and gRPC endpoints, uses a multi-tier AI fallback engine (OpenRouter with multi-model pools, Gemini, and heuristic fallbacks), and enforces internal authentication via a shared-secret token.

---

## Tech Stack

* Runtime: Go 1.22+
* Protocols: HTTP/1.1 (REST) and HTTP/2 (gRPC / Protocol Buffers)
* AI Providers: OpenRouter API (multi-model fallback chain), Google Gemini API, deterministic heuristic generator
* Testing: Go standard library `testing`, `httptest`, and `bufconn` for in-memory gRPC testing
* CI: GitHub Actions (`go vet`, `go test`, `gofmt`)

---

## Architecture and Features

### Dual Protocol Support

V-Chef runs two concurrent servers inside a single binary:

1. HTTP REST Server (default port: `:8085` or `$PORT`): Standard JSON API compatible with cloud proxies, load balancers, and external HTTP clients.
2. gRPC Server (default port: `:50051` or `$GRPC_PORT`): High-throughput, low-latency RPC interface based on `proto/v1/chef.proto` with strong typing and binary serialization.

### Multi-Tier AI Generation Pipeline

1. OpenRouter API: Sends structured prompts to OpenRouter using JSON object mode. Supports prioritized model lists (`OPENROUTER_MODELS`). If a multi-model array request fails, the service attempts sequential single-model retries across configured candidates.
2. Google Gemini API: Secondary fallback provider invoked if OpenRouter is unreachable or unconfigured.
3. Heuristic Engine: Deterministic offline recipe generator ensuring valid responses during external API downtime or local development without API keys.

### Authentication and Security

* Shared Secret Header: Requests to `POST /api/v1/recipes/generate` require an `X-Internal-Token` HTTP header matching the `INTERNAL_TOKEN` environment variable.
* gRPC Metadata Interceptor: gRPC requests validate the `x-internal-token` metadata key using a server-side unary interceptor.
* Public Health Check: `GET /health` (REST) and `ChefService.HealthCheck` (gRPC) remain unauthenticated to allow platform liveness probes and keep-alive warmup pings (e.g. Render free tier).
* CORS: Configured for secure cross-origin requests when called directly by web clients.

---

## Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go         # Application composition root, REST & gRPC server lifecycles
├── internal/
│   ├── chef/
│   │   ├── generator.go    # Recipe generation service, prompts builder, AI fallback logic
│   │   └── generator_test.go
│   ├── config/
│   │   └── config.go       # Strongly-typed environment variable loader
│   ├── handler/
│   │   ├── grpc.go         # gRPC server implementation and unary auth interceptor
│   │   ├── grpc_test.go    # In-memory gRPC test suite
│   │   ├── recipe.go       # HTTP REST handlers (GenerateRecipe, HealthCheck)
│   │   ├── recipe_test.go  # HTTP handler tests
│   │   ├── router.go       # HTTP router setup, auth and CORS middleware
│   │   └── router_test.go  # Middleware and routing test suite
│   └── models/
│       └── recipe.go       # Domain data models and DTO structures
├── proto/
│   └── v1/
│       ├── chef.proto      # Protocol Buffers service definition
│       ├── chef.pb.go      # Generated Go protobuf structs
│       └── chef_grpc.pb.go # Generated Go gRPC server stubs
├── .github/
│   └── workflows/
│       └── ci.yml          # GitHub Actions CI pipeline
├── go.mod
├── go.sum
└── main.go                 # Root entrypoint redirecting to cmd/server for Render compatibility
```

---

## Configuration

All configuration is supplied via environment variables:

| Variable | Description | Default |
| --- | --- | --- |
| `PORT` | HTTP REST server port | `8085` |
| `GRPC_PORT` | gRPC server listening port | `50051` |
| `INTERNAL_TOKEN` | Shared secret for internal service-to-service auth (`X-Internal-Token`) | `""` (disabled in dev) |
| `OPENROUTER_API_KEY` | OpenRouter API Key for primary LLM generation | `""` |
| `OPENROUTER_MODELS` | Comma-separated list of OpenRouter models for fallback | `google/gemma-4-31b-it:free,nvidia/nemotron-3-super-120b-a12b:free` |
| `GEMINI_API_KEY` | Google Gemini API Key (secondary fallback) | `""` |
| `ENV` | Environment identifier (`development`, `production`) | `development` |

---

## API Reference

### HTTP REST

#### 1. Liveness & Warmup Check

* Method: `GET`
* Path: `/health`
* Auth: None
* Response (`200 OK`):
  ```json
  {
    "status": "ok",
    "timestamp": "2026-08-17T01:30:00Z"
  }
  ```

#### 2. Generate Recipe

* Method: `POST`
* Path: `/api/v1/recipes/generate`
* Header: `X-Internal-Token: <token>` (if configured)
* Request Body:
  ```json
  {
    "ingredients": ["eggs", "tomatoes", "cheese", "bread"],
    "meal_type": "breakfast",
    "dietary_category": "vegetarian",
    "max_prep_time_mins": 20,
    "target_calories": 500
  }
  ```
* Response Body (`200 OK`):
  ```json
  {
    "title": "Tomato and Cheese Scramble",
    "description": "A quick and savory breakfast dish using fresh tomatoes and melted cheese.",
    "prep_time_mins": 5,
    "cook_time_mins": 10,
    "servings": 1,
    "calories": 420,
    "protein_grams": 22.5,
    "fat_grams": 24.0,
    "carbs_grams": 18.0,
    "ingredients": [
      {
        "name": "eggs",
        "quantity": 2,
        "unit": "pcs",
        "in_fridge": true
      },
      {
        "name": "tomatoes",
        "quantity": 1,
        "unit": "pcs",
        "in_fridge": true
      }
    ],
    "steps": [
      "Dice the tomatoes.",
      "Beat the eggs and cook in a heated skillet for 3 minutes.",
      "Add diced tomatoes and shredded cheese until melted."
    ],
    "generated_at": "2026-08-17T01:30:00Z"
  }
  ```

### gRPC Interface

Defined in `proto/v1/chef.proto`:

```protobuf
syntax = "proto3";

package vchef.v1;

option go_package = "github.com/ynshvrh/V-Chef/pkg/vchef/v1;vchefv1";

service ChefService {
  rpc GenerateRecipe (GenerateRecipeRequest) returns (GenerateRecipeResponse);
  rpc HealthCheck (HealthCheckRequest) returns (HealthCheckResponse);
}
```

Metadata key `x-internal-token` must be attached to the gRPC context when `INTERNAL_TOKEN` is set.

---

## Local Development and Running

### Prerequisites

* Go 1.22 or higher
* (Optional) `protoc` and `protoc-gen-go` / `protoc-gen-go-grpc` for updating protobuf files

### Run the Service

```bash
# Run server
go run ./cmd/server

# Or with environment variables
PORT=8085 GRPC_PORT=50051 INTERNAL_TOKEN="dev-secret" go run ./cmd/server
```

### Run Tests

```bash
go test -v ./...
```

### Recompile Protocol Buffers

If you make modifications to `proto/v1/chef.proto`:

```bash
protoc --proto_path=proto/v1 \
       --go_out=proto/v1 --go_opt=paths=source_relative \
       --go-grpc_out=proto/v1 --go-grpc_opt=paths=source_relative \
       proto/v1/chef.proto
```

### Verification Examples

#### Test REST with curl

```bash
curl -X POST http://localhost:8085/api/v1/recipes/generate \
  -H "Content-Type: application/json" \
  -H "X-Internal-Token: dev-secret" \
  -d '{
    "ingredients": ["chicken", "garlic", "rice", "broccoli"],
    "meal_type": "dinner",
    "dietary_category": "high-protein",
    "max_prep_time_mins": 30,
    "target_calories": 650
  }'
```

#### Test gRPC with grpcurl

```bash
# Health check
grpcurl -plaintext localhost:50051 vchef.v1.ChefService/HealthCheck

# Generate recipe
grpcurl -plaintext \
  -H "x-internal-token: dev-secret" \
  -d '{
    "ingredients": ["pasta", "tomato", "basil"],
    "meal_type": "lunch",
    "dietary_category": "vegetarian",
    "max_prep_time_mins": 20,
    "target_calories": 500
  }' \
  localhost:50051 vchef.v1.ChefService/GenerateRecipe
```