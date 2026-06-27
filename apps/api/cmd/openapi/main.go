package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/talentpilot/talentpilot/apps/api/internal/app"
)

func main() {
	server := app.NewServer()

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(server.API.OpenAPI()); err != nil {
		log.Fatal(err)
	}
}
