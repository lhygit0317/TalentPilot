package config

import "os"

type Config struct {
	Env                 string
	APIAddr             string
	DatabaseDriver      string
	DatabaseDSN         string
	FrontendOrigin      string
	SecureCookies       bool
	TrustForwardedProto bool
	W3Mode              string
}

func Load() Config {
	env := envOrDefault("APP_ENV", "development")
	databaseDriver, databaseDSN := databaseConfig()

	w3Mode := os.Getenv("W3_MODE")
	if w3Mode == "" {
		w3Mode = "mock"
		if env == "production" {
			w3Mode = "production"
		}
	}

	return Config{
		Env:                 env,
		APIAddr:             envOrDefault("API_ADDR", ":8080"),
		DatabaseDriver:      databaseDriver,
		DatabaseDSN:         databaseDSN,
		FrontendOrigin:      envOrDefault("FRONTEND_ORIGIN", "http://localhost:5173"),
		SecureCookies:       secureCookies(env),
		TrustForwardedProto: parseBool(os.Getenv("TRUST_FORWARDED_PROTO")),
		W3Mode:              w3Mode,
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func databaseConfig() (driver string, dsn string) {
	databaseURL := os.Getenv("DATABASE_URL")
	defaultDriver := "sqlite"
	defaultDSN := "file:talentpilot_dev.db?_foreign_keys=on"
	if databaseURL != "" {
		defaultDriver = "postgres"
		defaultDSN = databaseURL
	}
	return envOrDefault("DATABASE_DRIVER", defaultDriver), envOrDefault("DATABASE_DSN", defaultDSN)
}

func parseBool(value string) bool {
	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}

func secureCookies(env string) bool {
	if env == "production" {
		return true
	}
	return parseBool(os.Getenv("SECURE_COOKIES"))
}
