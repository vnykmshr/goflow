package stream

import (
	"context"
	"fmt"
	"strings"
)

// Example demonstrates basic stream usage.
func Example() {
	// Create a stream from a slice
	stream := FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	defer func() { _ = stream.Close() }()

	// Chain operations: filter even numbers, multiply by 2, take first 3
	result, err := stream.
		Filter(func(x int) bool { return x%2 == 0 }).
		Map(func(x int) int { return x * 2 }).
		Limit(3).
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Result: %v\n", result)
	// Output: Result: [4 8 12]
}

// Example_dataProcessing demonstrates a data processing pipeline.
func Example_dataProcessing() {
	// Sample data: user names
	users := []string{"john.doe", "jane.smith", "bob.wilson", "alice.brown"}

	stream := FromSlice(users)
	defer func() { _ = stream.Close() }()

	// Process names: convert to proper case and create email addresses
	emails, err := stream.
		Filter(func(name string) bool { return len(name) > 5 }). // Filter long names
		Map(func(name string) string {
			// Convert to proper case
			parts := strings.Split(name, ".")
			for i, part := range parts {
				if len(part) > 0 {
					parts[i] = strings.ToUpper(part[:1]) + part[1:]
				}
			}
			return strings.Join(parts, " ")
		}).
		Map(func(name string) string {
			// Create email address
			emailName := strings.ToLower(strings.ReplaceAll(name, " ", "."))
			return emailName + "@company.com"
		}).
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for _, email := range emails {
		fmt.Println(email)
	}
	// Output:
	// john.doe@company.com
	// jane.smith@company.com
	// bob.wilson@company.com
	// alice.brown@company.com
}

// Example_aggregation demonstrates various aggregation operations.
func Example_aggregation() {
	numbers := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Count elements
	count, _ := FromSlice(numbers).Count(context.Background())
	fmt.Printf("Count: %d\n", count)

	// Sum using reduce
	sum, _ := FromSlice(numbers).Reduce(context.Background(), 0, func(acc, x int) int {
		return acc + x
	})
	fmt.Printf("Sum: %d\n", sum)

	// Find min and max
	minValue, found, _ := FromSlice(numbers).Min(context.Background(), func(a, b int) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})
	if found {
		fmt.Printf("Min: %d\n", minValue)
	}

	maxValue, found, _ := FromSlice(numbers).Max(context.Background(), func(a, b int) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})
	if found {
		fmt.Printf("Max: %d\n", maxValue)
	}

	// Check if any/all numbers meet criteria
	hasEven, _ := FromSlice(numbers).AnyMatch(context.Background(), func(x int) bool {
		return x%2 == 0
	})
	fmt.Printf("Has even numbers: %t\n", hasEven)

	allPositive, _ := FromSlice(numbers).AllMatch(context.Background(), func(x int) bool {
		return x > 0
	})
	fmt.Printf("All positive: %t\n", allPositive)

	// Output:
	// Count: 10
	// Sum: 55
	// Min: 1
	// Max: 10
	// Has even numbers: true
	// All positive: true
}

// Example_textProcessing demonstrates text processing with streams.
func Example_textProcessing() {
	text := "The quick brown fox jumps over the lazy dog"
	words := strings.Fields(text)

	stream := FromSlice(words)
	defer func() { _ = stream.Close() }()

	// Process words: filter long words, convert to uppercase, sort
	processedWords, err := stream.
		Filter(func(word string) bool { return len(word) > 3 }).        // Words longer than 3 chars
		Map(func(word string) string { return strings.ToUpper(word) }). // Uppercase
		Distinct().                                                     // Remove duplicates
		Sorted(func(a, b string) int { return strings.Compare(a, b) }). // Sort alphabetically
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Processed words: %v\n", processedWords)
	// Output: Processed words: [BROWN JUMPS LAZY OVER QUICK]
}

// Example_numbers demonstrates number processing.
func Example_numbers() {
	// Generate stream of numbers and process them
	stream := FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	defer func() { _ = stream.Close() }()

	// Find squares of even numbers, skip first 2, limit to 3
	result, err := stream.
		Filter(func(x int) bool { return x%2 == 0 }). // Even numbers: 2,4,6,8,10,12
		Map(func(x int) int { return x * x }).        // Squares: 4,16,36,64,100,144
		Skip(2).                                      // Skip first 2: 36,64,100,144
		Limit(3).                                     // Take 3: 36,64,100
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Result: %v\n", result)
	// Output: Result: [36 64 100]
}

// Example_collectToCustomType demonstrates collecting to custom types.
func Example_collectToCustomType() {
	words := []string{"hello", "world", "from", "stream", "api"}

	stream := FromSlice(words)
	defer func() { _ = stream.Close() }()

	// Collect into a custom string with separators
	result, err := stream.
		Filter(func(word string) bool { return len(word) > 3 }).
		Collect(
			context.Background(),
			func() interface{} { return &strings.Builder{} }, // Supplier
			func(acc interface{}, value string) { // Accumulator
				builder := acc.(*strings.Builder)
				if builder.Len() > 0 {
					builder.WriteString(", ")
				}
				builder.WriteString(value)
			},
			func(acc1, acc2 interface{}) interface{} { // Combiner (for parallel processing)
				b1 := acc1.(*strings.Builder)
				b2 := acc2.(*strings.Builder)
				b1.WriteString(", ")
				b1.WriteString(b2.String())
				return b1
			},
		)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	builder := result.(*strings.Builder)
	fmt.Printf("Collected: %s\n", builder.String())
	// Output: Collected: hello, world, from, stream
}

// Example_channels demonstrates creating streams from channels.
func Example_channels() {
	// Create a channel and send some data
	ch := make(chan string, 5)
	ch <- "apple"
	ch <- "banana"
	ch <- "cherry"
	ch <- "date"
	ch <- "elderberry"
	close(ch)

	stream := FromChannel(ch)
	defer func() { _ = stream.Close() }()

	// Process fruits: filter by length and convert to uppercase
	result, err := stream.
		Filter(func(fruit string) bool { return len(fruit) <= 5 }).
		Map(func(fruit string) string { return strings.ToUpper(fruit) }).
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Short fruits: %v\n", result)
	// Output: Short fruits: [APPLE DATE]
}

// Example_generator demonstrates infinite streams with generators.
func Example_generator() {
	counter := 0

	// Generate infinite stream of incrementing numbers
	stream := Generate(func() int {
		counter++
		return counter
	})
	defer func() { _ = stream.Close() }()

	// Take first 5 even squares
	result, err := stream.
		Filter(func(x int) bool { return x%2 == 0 }). // Even numbers
		Map(func(x int) int { return x * x }).        // Squares
		Limit(5).                                     // Limit to 5
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("First 5 even squares: %v\n", result)
	// Output: First 5 even squares: [4 16 36 64 100]
}

// Example_peek demonstrates debugging with peek.
// Peek allows you to observe elements as they flow through the pipeline
// without modifying them. Useful for debugging and logging.
func Example_peek() {
	stream := FromSlice([]int{1, 2, 3, 4, 5})
	defer func() { _ = stream.Close() }()

	// Use Peek to observe elements at different stages of the pipeline.
	// Note: Due to lazy evaluation, peek output order may vary.
	// We don't verify the debug output here, only the final result.
	result, err := stream.
		Peek(func(x int) { _ = x /* Observe original values */ }).
		Filter(func(x int) bool { return x%2 == 0 }).
		Peek(func(x int) { _ = x /* Observe filtered values */ }).
		Map(func(x int) int { return x * 10 }).
		Peek(func(x int) { _ = x /* Observe mapped values */ }).
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Result: %v\n", result)
	// Output: Result: [20 40]
}

// Example_flatMap demonstrates flattening nested structures.
func Example_flatMap() {
	// Each number generates a range from 1 to that number
	stream := FromSlice([]int{2, 3, 4})
	defer func() { _ = stream.Close() }()

	result, err := stream.
		FlatMap(func(n int) Stream[int] {
			// Create a range from 1 to n
			nums := make([]int, n)
			for i := 0; i < n; i++ {
				nums[i] = i + 1
			}
			return FromSlice(nums)
		}).
		ToSlice(context.Background())

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Flattened: %v\n", result)
	// Output: Flattened: [1 2 1 2 3 1 2 3 4]
}

// Example_wordCount demonstrates a word counting pipeline.
func Example_wordCount() {
	text := "the quick brown fox jumps over the lazy dog the fox is quick"
	words := strings.Fields(text)

	// Count word frequencies using collect
	stream := FromSlice(words)
	defer func() { _ = stream.Close() }()

	wordCount, err := stream.
		Map(func(word string) string { return strings.ToLower(word) }). // Normalize case
		Collect(
			context.Background(),
			func() interface{} { return make(map[string]int) }, // Create map
			func(acc interface{}, word string) { // Count words
				wordMap := acc.(map[string]int)
				wordMap[word]++
			},
			func(acc1, acc2 interface{}) interface{} { // Combine maps
				map1 := acc1.(map[string]int)
				map2 := acc2.(map[string]int)
				for word, count := range map2 {
					map1[word] += count
				}
				return map1
			},
		)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	wordMap := wordCount.(map[string]int)
	fmt.Printf("Word frequencies: %v\n", wordMap)
	// Output: Word frequencies: map[brown:1 dog:1 fox:2 is:1 jumps:1 lazy:1 over:1 quick:2 the:3]
}
