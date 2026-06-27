package db

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Config struct {
	Driver string
	DSN    string
}

func Open(cfg Config) (*gorm.DB, error) {
	switch cfg.Driver {
	case "sqlite":
		return gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{})
	case "postgres":
		return gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}
