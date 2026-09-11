# V-Chef

High-performance AI culinary microservice for the V-Fridge ecosystem built with Go.

V-Chef provides structured recipe generation from available fridge ingredients, meal types, preparation time limits, and dietary/macro constraints. It exposes a lightweight, high-performance HTTP REST API, uses a multi-tier AI fallback engine (OpenRouter with multi-model pools, Gemini, and heuristic fallbacks), and enforces internal authentication via a shared-secret token.

---

## Tech Stack

* Runtime: Go 1.22+
* Protocols: HTTP/1.1 (REST JSON API)
* AI Providers: OpenRouter API (multi-model fallback chain), Google Gemini API, deterministic heuristic generator
* Testing: Go standard library `testing`, `httptest`
* CI: GitHub Actions (`go vet`, `go test`, `gofmt`)

---

## Architecture and Features

### HTTP REST Server

V-Chef runs a fast, lightweight HTTP server (default port: `:8085` or `$PORT`), compatible with cloud proxies, load balancers, and internal service clients.

### Multi-Tier AI Generation Pipeline

1. OpenRouter API: Sends structured prompts to OpenRouter using JSON object mode. Supports prioritized model lists (`OPENROUTER_MODELS`). If a multi-model array request fails, the service attempts sequential single-model retries across configured candidates.
2. Google Gemini API: Secondary fallback provider invoked if OpenRouter is unreachable or unconfigured.
3. Heuristic Engine: Deterministic offline recipe generator ensuring valid responses during external API downtime or local development without API keys.

### Authentication and Security

* Shared Secret Header: Requests to `POST /api/v1/recipes/generate` and `POST /api/v1/chat` require an `X-Internal-Token` HTTP header matching the `INTERNAL_TOKEN` environment variable.
* Public Health Check: `GET /health` remains unauthenticated to allow platform liveness probes and keep-alive warmup pings (e.g. Render free tier).
* CORS: Configured for secure cross-origin requests when called directly by web clients.

---

## Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go         # Application composition root and HTTP server lifecycle
├── internal/
│   ├── chef/
│   │   ├── chat.go         # AI Chat handler with structured recipe/shopping suggestions
│   │   ├── generator.go    # Recipe generation service, prompts builder, AI fallback logic
│   │   ├── generator_test.go
│   │   ├── normalizer.go   # Ingredient parsing and unit standardization
│   │   └── normalizer_test.go
│   ├── config/
│   │   └── config.go       # Strongly-typed environment variable loader
│   ├── handler/
│   │   ├── recipe.go       # HTTP REST handlers (GenerateRecipe, Chat, HealthCheck)
│   │   ├── recipe_test.go  # HTTP handler tests
│   │   ├── router.go       # HTTP router setup, auth and CORS middleware
│   │   └── router_test.go  # Middleware and routing test suite
│   └── models/
│       ├── chat.go         # Chat domain models and request/response DTOs
│       └── recipe.go       # Recipe domain data models and DTO structures
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

#### 3. AI Chef Chat

* Method: `POST`
* Path: `/api/v1/chat`
* Header: `X-Internal-Token: <token>` (if configured)

---

## Local Development and Running

### Prerequisites

* Go 1.22 or higher

### Run the Service

```bash
# Run server
go run ./cmd/server

# Or with environment variables
PORT=8085 INTERNAL_TOKEN="dev-secret" go run ./cmd/server
```

### Run Tests

```bash
go test -v ./...
```