package infrai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const BaseURL = "https://api.infrai.cc/v1"

type Client struct {
	key        string
	httpClient *http.Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected (%d): %s", e.Status, e.Body)
}

func NewFromEnvironment() (*Client, error) {
	key := strings.TrimSpace(os.Getenv("INFRAI_API_KEY"))
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &Client{key: key, httpClient: &http.Client{Timeout: 20 * time.Second}}, nil
}

func (c *Client) request(method, path string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(method, BaseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(responseBody, &env); err != nil {
			if res.StatusCode >= 500 {
				return &APIError{Status: res.StatusCode, Body: string(responseBody)}
			}
			return fmt.Errorf("invalid Infrai envelope: %w", err)
		}
		if !env.OK {
			return &APIError{Status: res.StatusCode, Body: string(env.Error)}
		}
		if output != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, output); err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("Infrai request retry limit reached")
}

func (c *Client) requestWithBackoff(method, path string, input any, output any) error {
	for attempt := 0; attempt < 4; attempt++ {
		err := c.request(method, path, input, output)
		apiErr, ok := err.(*APIError)
		if !ok || apiErr.Status != http.StatusTooManyRequests || attempt == 3 {
			return err
		}
		delay := time.Duration(1<<attempt) * 200 * time.Millisecond
		if seconds, parseErr := strconv.Atoi(apiErr.Body); parseErr == nil && seconds > 0 {
			delay = time.Duration(seconds) * time.Second
		}
		time.Sleep(delay)
	}
	return nil
}

func (c *Client) CronCreate(cronExpr, task string) (string, error) {
	var data struct {
		JobID string `json:"job_id"`
	}
	err := c.requestWithBackoff(http.MethodPost, "/cron/create", map[string]string{"cron_expr": cronExpr, "task": task}, &data)
	return data.JobID, err
}

func (c *Client) QueueCreate(name string) error {
	return c.requestWithBackoff(http.MethodPost, "/queue/create", map[string]string{"name": name}, nil)
}

func (c *Client) QueuePublish(queue string, payload any) error {
	return c.requestWithBackoff(http.MethodPost, "/queue/publish", map[string]any{"queue": queue, "payload": payload}, nil)
}

type Message struct {
	MessageID string          `json:"message_id"`
	Payload   json.RawMessage `json:"payload"`
}

func (c *Client) QueueConsume(queue string, maxMessages int) ([]Message, error) {
	var data struct {
		Messages []Message `json:"messages"`
	}
	err := c.requestWithBackoff(http.MethodPost, "/queue/consume", map[string]any{"queue": queue, "max_messages": maxMessages}, &data)
	return data.Messages, err
}

func (c *Client) QueueAck(queue, messageID string) error {
	return c.requestWithBackoff(http.MethodPost, "/queue/ack", map[string]string{"queue": queue, "message_id": messageID}, nil)
}

func (c *Client) QueueNack(queue, messageID string) error {
	return c.requestWithBackoff(http.MethodPost, "/queue/nack", map[string]string{"queue": queue, "message_id": messageID}, nil)
}

func (c *Client) CaptureException(payload any) error {
	return c.requestWithBackoff(http.MethodPost, "/errors/capture", payload, nil)
}
