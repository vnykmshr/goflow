// Streaming example demonstrating data processing with streams and async writing
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/vnykmshr/goflow/pkg/streaming/stream"
	"github.com/vnykmshr/goflow/pkg/streaming/writer"
)

func main() {
	// Example 1: Stream processing with functional operations
	fmt.Println("=== Stream Processing Example ===")

	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	fmt.Printf("Input: %v\n", numbers)

	// Process numbers: filter even, multiply by 2, collect results
	s := stream.FromSlice(numbers)
	defer s.Close()

	result, err := s.
		Filter(func(x int) bool {
			return x%2 == 0
		}).
		Map(func(x int) int {
			return x * 2
		}).
		ToSlice(context.Background())

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Even numbers * 2: %v\n", result)

	// Example 2: String processing
	fmt.Println("\n=== String Processing Example ===")

	words := []string{"hello", "world", "go", "streaming", "example"}
	fmt.Printf("Input words: %v\n", words)

	// Process strings: filter by length, convert to uppercase
	s2 := stream.FromSlice(words)
	defer s2.Close()

	longWords, err := s2.
		Filter(func(s string) bool {
			return len(s) > 2
		}).
		Map(func(s string) string {
			return strings.ToUpper(s)
		}).
		ToSlice(context.Background())

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Long words (uppercase): %v\n", longWords)

	// Example 3: Async writer for high-volume output
	fmt.Println("\n=== Async Writer Example ===")

	// Create temporary file for writing
	file, err := os.CreateTemp("", "goflow_example_*.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name()) // Clean up
	defer file.Close()

	// Create async writer with buffering
	asyncWriter := writer.New(file)

	// Write data asynchronously
	fmt.Printf("Writing to file: %s\n", file.Name())
	for i := 1; i <= 5; i++ {
		line := fmt.Sprintf("Line %d: Processing data at %v\n", i, time.Now())

		err := asyncWriter.Write([]byte(line))
		if err != nil {
			log.Printf("Error writing line %d: %v", i, err)
		} else {
			fmt.Printf("Queued: %s", line)
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Close writer and wait for all data to be written
	fmt.Println("Closing async writer...")
	if err := asyncWriter.Close(); err != nil {
		log.Printf("Error closing async writer: %v", err)
	}

	// Read back the file to verify
	content, err := os.ReadFile(file.Name())
	if err != nil {
		log.Printf("Error reading file: %v", err)
	} else {
		fmt.Printf("\nFile contents:\n%s", content)
	}

	// Example 4: Complex data pipeline
	fmt.Println("=== Data Pipeline Example ===")

	// Simulate log entries
	logEntries := []string{
		"INFO: User login successful",
		"ERROR: Database connection failed",
		"WARN: High memory usage",
		"INFO: Request processed",
		"ERROR: Invalid input",
		"DEBUG: Cache hit",
	}

	fmt.Printf("Log entries: %v\n", logEntries)

	// Process logs: extract errors, get error messages
	s3 := stream.FromSlice(logEntries)
	defer s3.Close()

	errorMessages, err := s3.
		Filter(func(entry string) bool {
			return strings.HasPrefix(entry, "ERROR:")
		}).
		Map(func(entry string) string {
			// Extract message after "ERROR: "
			if len(entry) > 7 {
				return entry[7:]
			}
			return entry
		}).
		ToSlice(context.Background())

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Error messages: %v\n", errorMessages)

	fmt.Println("\n=== Streaming Examples Complete ===")
}
