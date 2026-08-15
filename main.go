package main

import (
	"log"
	"net/http"
	"os"

	"BS_backend_screening/engine"
	"BS_backend_screening/handlers"
	"BS_backend_screening/pipeline"
	"BS_backend_screening/security"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	templateDir := os.Getenv("TEMPLATES_DIR")
	if templateDir == "" {
		templateDir = "./templates"
	}

	// Webhook secret read securely from environment variable (never hardcoded)
	webhookSecret := os.Getenv(security.EnvWebhookSecret)
	if webhookSecret == "" {
		// Fallback for local development if unset, while warning the operator
		log.Printf("[WARNING] %s is unset. Using default development secret. DO NOT USE IN PRODUCTION.\n", security.EnvWebhookSecret)
		webhookSecret = "dev_webhook_secret_key_change_in_prod"
	}

	// Step 1: Initialize External Template Engine (Strict startup validation)
	// Operator Safety: Fails immediately at startup if templates are missing or malformed
	templateEngine, err := engine.NewEngine(templateDir)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize template engine from '%s': %v", templateDir, err)
	}
	log.Printf("[INFO] Loaded templates successfully from '%s'", templateDir)

	// Step 2: Initialize Generic Processing Pipeline
	eventPipeline, err := pipeline.NewPipeline(templateEngine)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize event pipeline: %v", err)
	}

	// Step 3: Initialize Security Verifier
	verifier, err := security.NewVerifier(webhookSecret)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize security verifier: %v", err)
	}

	// Step 4: Wire HTTP Handlers and Security Middleware
	webhookHandler := handlers.NewWebhookHandler(eventPipeline)
	protectedWebhookHandler := verifier.Middleware(webhookHandler)

	http.Handle("/webhook", protectedWebhookHandler)
	http.HandleFunc("/", handlers.RootHandler)

	log.Printf("[INFO] Cashfree Webhook Consumer listening on port %s (POST http://localhost:%s/webhook)...\n", port, port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[FATAL] HTTP server failed: %v", err)
	}
}