package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)



type Todo struct {
	UserID    int    `json:"userId"`
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

// fetchTodoWithErrorChan makes an HTTP GET request and sends results/errors through channels
func fetchTodoWithErrorChan(ctx context.Context, todoID int) (<-chan *Todo, <-chan error) {
	// Create buffered channels
	todoChan := make(chan *Todo, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(todoChan)
		defer close(errChan)

		// Add artificial delay to demonstrate timeout (3 seconds)
		delay := 3 * time.Second
		log.Printf("Starting request for todo %d (artificial delay: %v)...\n", todoID, delay)
		
		select {
		case <-time.After(delay):
			// Continue after delay
		case <-ctx.Done():
			errChan <- fmt.Errorf("request cancelled before starting: %v", ctx.Err())
			return
		}

		// Create a new request
		req, err := http.NewRequestWithContext(
			ctx,
			"GET",
			fmt.Sprintf("https://jsonplaceholder.typicode.com/todos/%d", todoID),
			nil,
		)
		if err != nil {
			errChan <- fmt.Errorf("error creating request: %v", err)
			return
		}

		// Make the request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			errChan <- fmt.Errorf("request failed: %v", err)
			return
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			errChan <- fmt.Errorf("error reading response: %v", err)
			return
		}

		// Unmarshal the JSON response
		var todo Todo
		if err := json.Unmarshal(body, &todo); err != nil {
			errChan <- fmt.Errorf("error decoding response: %v", err)
			return
		}

		todoChan <- &todo
	}()

	return todoChan, errChan
}

// simulateSlowRequest simulates a slow request that takes at least the specified duration
func simulateSlowRequest(ctx context.Context, todoID int, minDuration time.Duration) (*Todo, error) {
	// Create a new context with the minimum duration
	timeoutCtx, cancel := context.WithTimeout(ctx, minDuration)
	defer cancel()

	// Get the result and error channels
	todoChan, errChan := fetchTodoWithErrorChan(timeoutCtx, todoID)

	// Wait for either the result, error, or timeout
	select {
	case todo := <-todoChan:
		if todo == nil {
			return nil, fmt.Errorf("received nil todo")
		}
		
		return todo, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("request timed out after %v: %v", minDuration, ctx.Err())
	}
}

// fetchMultipleTodos demonstrates handling multiple concurrent requests
func fetchMultipleTodos(ctx context.Context, ids ...int) ([]*Todo, error) {
	var wg sync.WaitGroup
	todos := make([]*Todo, 0, len(ids))
	errs := make([]error, 0)
	mu := sync.Mutex{}

	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			todoChan, errChan := fetchTodoWithErrorChan(ctx, id)
			
			select {
			case todo := <-todoChan:
				if todo != nil {
					mu.Lock()
					todos = append(todos, todo)
					mu.Unlock()
				}
			case err := <-errChan:
				if err != nil {
					mu.Lock()
					errs = append(errs, fmt.Errorf("todo %d: %v", id, err))
					mu.Unlock()
				}
			case <-ctx.Done():
				// Context was cancelled, just return
				return
			}
		}(id)
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for either all goroutines to complete or context to be cancelled
	select {
	case <-done:
		// All goroutines completed
	case <-ctx.Done():
		// Context was cancelled, return what we have so far
	}

	// Return any errors we encountered
	if len(errs) > 0 {
		return todos, fmt.Errorf("%d errors occurred: %v", len(errs), errs[0])
	}
	return todos, nil
}

// truncateString shortens a string to the specified length and adds "..." if truncated
func truncateString(str string, num int) string {
	if len(str) <= num {
		return str
	}
	return str[:num] + "..."
}

func main() {
	// Example 1: Single request with timeout
	{
		log.Println("=== Example 1: Single Request with Timeout ===")
		log.Println("This example demonstrates a request that will timeout after 2 seconds")
		log.Println("The server has an artificial 3-second delay to ensure timeout")
		
		// Create a context with timeout of 2 seconds
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		log.Println("Starting request...")
		start := time.Now()

		// Try to fetch with a simulated slow response
		todoChan, errChan := fetchTodoWithErrorChan(ctx, 1)
		
		select {
		case todo := <-todoChan:
			log.Printf("Successfully fetched todo after %v: %+v", time.Since(start).Round(time.Millisecond), todo)
		case err := <-errChan:
			log.Printf("Error after %v: %v", time.Since(start).Round(time.Millisecond), err)
		case <-ctx.Done():
			log.Printf("Context done after %v: %v", time.Since(start).Round(time.Millisecond), ctx.Err())
		}
		
		// Add some space between examples
		log.Println("\n" + strings.Repeat("-", 80) + "\n")
	}

	// Example 2: Multiple concurrent requests with mixed results
	{
		log.Println("=== Example 2: Multiple Concurrent Requests ===")
		log.Println("This example shows multiple concurrent requests with a 5-second timeout")
		log.Println("Some requests will succeed, others will time out")
		
		// Create a context with timeout of 5 seconds (3s artificial delay + time for requests)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Mix of fast and slow requests
		ids := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
		log.Printf("Fetching %d todos concurrently...\n", len(ids))
		
		start := time.Now()
		todos, err := fetchMultipleTodos(ctx, ids...)
		elapsed := time.Since(start).Round(time.Millisecond)
		
		log.Printf("\nCompleted in %v", elapsed)
		
		// Print results
		log.Printf("\nSuccessfully fetched %d/%d todos:", len(todos), len(ids))
		for _, todo := range todos {
			status := "Pending"
			if todo.Completed {
				status = "Completed"
			}
			log.Printf("- ID: %2d | Status: %-9s | Title: %s", 
				todo.ID, 
				status,
				truncateString(todo.Title, 30))
		}
		
		// Print any errors
		if err != nil {
			log.Printf("\nNote: Some requests failed: %v", err)
		}
	}
}
