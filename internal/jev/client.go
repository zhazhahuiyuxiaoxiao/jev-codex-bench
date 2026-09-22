package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	Model           = "jev-1.13.0"
	PricePerMillion = 0.042
	Endpoint        = "https://api.typesafe.ai/v1/systemone"
	MaxPayloadBytes = 16 * 1024
)

type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
}

type Request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

type Answer struct {
	Type string  `json:"type"`
	Noul float64 `json:"noul"`
}

type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type Client struct {
	HTTP   *http.Client
	URL    string
	Key    string
	Ledger *Ledger
}

func NewClient(ledger *Ledger) *Client {
	return &Client{
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		URL:    Endpoint,
		Key:    strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY")),
		Ledger: ledger,
	}
}

func (c *Client) Evaluate(ctx context.Context, req Request) (Response, error) {
	var result Response
	if c.Key == "" {
		return result, errors.New("TYPESAFE_API_KEY is not set")
	}
	if c.Ledger == nil {
		return result, errors.New("budget ledger is required")
	}
	if req.Model != Model {
		return result, fmt.Errorf("model must be pinned to %s", Model)
	}
	if len(req.Questions) == 0 {
		return result, errors.New("at least one question is required")
	}
	data, err := json.Marshal(req)
	if err != nil {
		return result, fmt.Errorf("encode request: %w", err)
	}
	if len(data) > MaxPayloadBytes {
		return result, fmt.Errorf("request is %d bytes, maximum is %d", len(data), MaxPayloadBytes)
	}
	// Conservative local upper bound, including protocol overhead. The provider's
	// token accounting is authoritative and a billing change cannot be ruled out.
	reserved := float64(2*len(data)+1024) * PricePerMillion / 1_000_000
	if err := c.Ledger.ReserveAndExecute(reserved, func() (float64, error) {
		httpClient := c.HTTP
		if httpClient == nil {
			httpClient = &http.Client{Timeout: 30 * time.Second}
		}
		url := c.URL
		if url == "" {
			url = Endpoint
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return 0, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.Key)
		httpReq.Header.Set("Content-Type", "application/json")
		httpResp, err := httpClient.Do(httpReq)
		if err != nil {
			return 0, fmt.Errorf("TypeSafe request failed: %w", err)
		}
		defer httpResp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
		if err != nil {
			return 0, fmt.Errorf("read TypeSafe response: %w", err)
		}
		if httpResp.StatusCode != http.StatusOK {
			// Do not include the body: it may echo submitted material.
			return 0, fmt.Errorf("TypeSafe HTTP %d", httpResp.StatusCode)
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return 0, fmt.Errorf("decode TypeSafe response: %w", err)
		}
		if result.Model != Model || result.Usage.InputTokens <= 0 {
			return 0, errors.New("TypeSafe response has unexpected model or missing usage")
		}
		for id := range req.Questions {
			answer, ok := result.Answers[id]
			if !ok || answer.Type != "noul" || answer.Noul < 0 || answer.Noul > 1 {
				return 0, fmt.Errorf("TypeSafe response has invalid answer for %s", id)
			}
		}
		return float64(result.Usage.InputTokens) * PricePerMillion / 1_000_000, nil
	}); err != nil {
		return Response{}, err
	}
	return result, nil
}
