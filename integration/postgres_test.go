package integration

import (
	"context"
	"os"
	"testing"
	"time"

	pgstore "github.com/sagarbagwe/agentflow/internal/store/postgres"
)

func TestPostgresConnection(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	storage, err := pgstore.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	storage.Close()
}
