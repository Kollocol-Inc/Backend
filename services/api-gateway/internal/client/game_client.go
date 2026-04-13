package client

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type GameClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewGameClient(host, port string) *GameClient {
	log.Printf("Game Service client initialized for %s:%s", host, port)
	return &GameClient{
		baseURL:    fmt.Sprintf("http://%s:%s", host, port),
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *GameClient) TerminateInstance(ctx context.Context, instanceID string) error {
	url := fmt.Sprintf("%s/instances/%s", c.baseURL, instanceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call game-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("game-service returned status %d", resp.StatusCode)
	}

	return nil
}
