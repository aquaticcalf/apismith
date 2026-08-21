package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"aegion-dynamic/api-console/environments"
	"aegion-dynamic/api-console/openapi"
	"aegion-dynamic/api-console/server"

	"github.com/joho/godotenv"
)

func main() {
	loadDotEnv()

	configPath := os.Getenv("CONSOLE_CONFIG")
	if configPath == "" {
		configPath = "config/environments.yaml"
	}

	cfg, err := environments.Load(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	catalog, err := openapi.Load(cfg.OpenAPISpec)
	if err != nil {
		log.Fatalf("openapi: %v", err)
	}

	srv := server.New(cfg, catalog)
	addr := cfg.Listen
	if addr == "" {
		addr = ":8090"
	}

	fmt.Printf("API Test Console\n")
	fmt.Printf("  spec        %s\n", cfg.OpenAPISpec)
	fmt.Printf("  endpoints   %d\n", len(catalog.Endpoints))
	fmt.Printf("  environment %s\n", cfg.DefaultEnvironment)
	fmt.Printf("  listen      http://localhost%s\n\n", addr)

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func loadDotEnv() {
	for _, p := range []string{".env", filepath.Join("config", ".env")} {
		_ = godotenv.Load(p)
	}
}
