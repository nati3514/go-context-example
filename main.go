package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Todo struct {
	UserID    int    `json:"userId"`
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

// fetchTodo makes an HTTP GET request with a timeout
func fetchTodo(ctx context.Context, todoID int) (*Todo, error) {
	// Create a new request
	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("https://jsonplaceholder.typicode.com/todos/%d", todoID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Unmarshal the JSON response
	var todo Todo
	if err := json.Unmarshal(body, &todo); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	return &todo, nil
}

// simulateSlowRequest simulates a slow request that takes at least the specified duration
func simulateSlowRequest(ctx context.Context, todoID int, minDuration time.Duration) (*Todo, error) {
	// Create a channel to receive the result
	resultChan := make(chan *Todo, 1)
	errChan := make(chan error, 1)

	// Start a goroutine to make the actual request
	go func() {
		// Simulate some processing time
		time.Sleep(minDuration)
		
		todo, err := fetchTodo(ctx, todoID)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- todo
	}()

	// Wait for either the result or a timeout
	select {
	case todo := <-resultChan:
		return todo, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("request timed out after %v: %v", minDuration, ctx.Err())
	}
}

func main() {
	// Create a context with a timeout of 2 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	log.Println("Fetching todo with ID 1...")

	// Try to fetch a todo with a simulated slow response (3 seconds)
	todo, err := simulateSlowRequest(ctx, 1, 3*time.Second)
	if err != nil {
		log.Printf("Error: %v\n", err)
		log.Println("This demonstrates how context timeout prevents waiting too long for a response.")
		return
	}

	// If we get here, the request completed before the timeout
	log.Printf("Successfully fetched todo: %+v\n", todo)
}
