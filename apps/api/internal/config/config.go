package config

import "os"

type Config struct {
	Env     string
	APIAddr string
}

func Load() Config {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		Env:     env,
		APIAddr: addr,
	}
}
