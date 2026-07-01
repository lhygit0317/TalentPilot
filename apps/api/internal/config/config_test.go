package config

import "testing"

func TestLoadUsesTask5Defaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("API_ADDR", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_DSN", "")
	t.Setenv("FRONTEND_ORIGIN", "")
	t.Setenv("SECURE_COOKIES", "")
	t.Setenv("W3_MODE", "")

	cfg := Load()

	if cfg.Env != "development" {
		t.Fatalf("expected Env development, got %q", cfg.Env)
	}
	if cfg.APIAddr != ":8080" {
		t.Fatalf("expected APIAddr :8080, got %q", cfg.APIAddr)
	}
	if cfg.DatabaseDriver != "sqlite" {
		t.Fatalf("expected DatabaseDriver sqlite, got %q", cfg.DatabaseDriver)
	}
	if cfg.DatabaseDSN != "file:talentpilot_dev.db?_foreign_keys=on" {
		t.Fatalf("expected local SQLite DSN, got %q", cfg.DatabaseDSN)
	}
	if cfg.FrontendOrigin != "http://localhost:5173" {
		t.Fatalf("expected local frontend origin, got %q", cfg.FrontendOrigin)
	}
	if cfg.SecureCookies {
		t.Fatalf("expected SecureCookies false outside production")
	}
	if cfg.W3Mode != "mock" {
		t.Fatalf("expected W3Mode mock outside production, got %q", cfg.W3Mode)
	}
}

func TestLoadUsesDatabaseURLForPostgres(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://talentpilot:talentpilot@localhost:5432/talentpilot_test?sslmode=disable")
	t.Setenv("DATABASE_DRIVER", "")
	t.Setenv("DATABASE_DSN", "")

	cfg := Load()

	if cfg.DatabaseDriver != "postgres" {
		t.Fatalf("expected DATABASE_URL to select postgres driver, got %q", cfg.DatabaseDriver)
	}
	if cfg.DatabaseDSN != "postgres://talentpilot:talentpilot@localhost:5432/talentpilot_test?sslmode=disable" {
		t.Fatalf("expected DatabaseDSN from DATABASE_URL, got %q", cfg.DatabaseDSN)
	}
}

func TestLoadHonorsTask5EnvironmentOverridesWithoutWeakeningProductionCookies(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("API_ADDR", ":9090")
	t.Setenv("DATABASE_DRIVER", "postgres")
	t.Setenv("DATABASE_DSN", "host=db user=talentpilot")
	t.Setenv("FRONTEND_ORIGIN", "https://talentpilot.example")
	t.Setenv("SECURE_COOKIES", "false")
	t.Setenv("W3_MODE", "production")

	cfg := Load()

	if cfg.Env != "production" {
		t.Fatalf("expected Env production, got %q", cfg.Env)
	}
	if cfg.APIAddr != ":9090" {
		t.Fatalf("expected APIAddr override, got %q", cfg.APIAddr)
	}
	if cfg.DatabaseDriver != "postgres" {
		t.Fatalf("expected DatabaseDriver override, got %q", cfg.DatabaseDriver)
	}
	if cfg.DatabaseDSN != "host=db user=talentpilot" {
		t.Fatalf("expected DatabaseDSN override, got %q", cfg.DatabaseDSN)
	}
	if cfg.FrontendOrigin != "https://talentpilot.example" {
		t.Fatalf("expected FrontendOrigin override, got %q", cfg.FrontendOrigin)
	}
	if !cfg.SecureCookies {
		t.Fatalf("expected production SecureCookies true even when SECURE_COOKIES=false")
	}
	if cfg.W3Mode != "production" {
		t.Fatalf("expected W3Mode override, got %q", cfg.W3Mode)
	}
}

func TestLoadUsesProductionSafeDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("SECURE_COOKIES", "")
	t.Setenv("W3_MODE", "")

	cfg := Load()

	if !cfg.SecureCookies {
		t.Fatalf("expected SecureCookies true by default in production")
	}
	if cfg.W3Mode == "mock" {
		t.Fatalf("expected production default W3Mode not to use mock")
	}
}
