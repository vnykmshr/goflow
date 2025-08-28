/*
Package stream provides a powerful and expressive API for processing sequences of data in Go.

The stream API is inspired by Java 8 Streams and provides a functional programming approach
to data processing with support for method chaining, lazy evaluation, and context-aware operations.

Core Concepts:

A Stream represents a sequence of elements supporting sequential operations. Streams are:
  - Lazy: computation is only performed when a terminal operation is initiated
  - Immutable: operations return new streams rather than modifying existing ones
  - Context-aware: all operations respect context cancellation and timeouts
  - Resource-managed: streams should be closed to release resources

Basic Usage:

	// Create a stream from a slice
	stream := stream.FromSlice([]int{1, 2, 3, 4, 5})
	defer stream.Close()

	// Chain operations and collect results
	result, err := stream.
		Filter(func(x int) bool { return x%2 == 0 }). // Keep even numbers
		Map(func(x int) int { return x * 2 }).         // Double them
		ToSlice(context.Background())                  // Collect to slice

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(result) // [4, 8]

Stream Creation:

Multiple ways to create streams:

	// From slice
	stream := stream.FromSlice([]string{"a", "b", "c"})

	// From channel
	ch := make(chan int, 3)
	ch <- 1; ch <- 2; ch <- 3; close(ch)
	stream := stream.FromChannel(ch)

	// From generator function (infinite)
	counter := 0
	stream := stream.Generate(func() int {
		counter++
		return counter
	})

	// Empty stream
	stream := stream.Empty[int]()

Intermediate Operations:

Intermediate operations are lazy and return new streams:

	// Filter elements based on predicate
	stream.Filter(func(x int) bool { return x > 0 })

	// Transform elements
	stream.Map(func(x int) int { return x * 2 })

	// Transform to different type
	stream.MapTo(func(x int) interface{} { return fmt.Sprintf("num-%d", x) })

	// Flatten nested streams
	stream.FlatMap(func(x int) Stream[int] {
		return stream.FromSlice([]int{x, x}) // Duplicate each element
	})

	// Remove duplicates
	stream.Distinct()

	// Sort elements
	stream.Sorted(func(a, b int) int {
		if a < b { return -1 }
		if a > b { return 1 }
		return 0
	})

	// Skip first n elements
	stream.Skip(5)

	// Limit to n elements
	stream.Limit(10)

	// Peek at elements (for debugging)
	stream.Peek(func(x int) { log.Printf("Processing: %d", x) })

Terminal Operations:

Terminal operations consume the stream and produce results:

	// Iterate over elements
	stream.ForEach(ctx, func(x int) {
		fmt.Println(x)
	})

	// Reduce to single value
	sum, err := stream.Reduce(ctx, 0, func(acc, x int) int {
		return acc + x
	})

	// Collect to custom type
	result, err := stream.Collect(
		ctx,
		func() interface{} { return &strings.Builder{} },
		func(acc interface{}, x string) {
			acc.(*strings.Builder).WriteString(x)
		},
		func(acc1, acc2 interface{}) interface{} { return acc1 },
	)

	// Convert to slice
	slice, err := stream.ToSlice(ctx)

	// Count elements
	count, err := stream.Count(ctx)

	// Find elements
	first, found, err := stream.FindFirst(ctx)
	any, found, err := stream.FindAny(ctx)

	// Test predicates
	hasAny, err := stream.AnyMatch(ctx, predicate)
	hasAll, err := stream.AllMatch(ctx, predicate)
	hasNone, err := stream.NoneMatch(ctx, predicate)

	// Find extremes
	min, found, err := stream.Min(ctx, comparator)
	max, found, err := stream.Max(ctx, comparator)

Data Processing Patterns:

Filter-Map-Reduce Pattern:

	result, err := stream.FromSlice(numbers).
		Filter(func(x int) bool { return x > 0 }).      // Keep positive
		Map(func(x int) int { return x * x }).           // Square them
		Reduce(ctx, 0, func(acc, x int) int {           // Sum squares
			return acc + x
		})

Data Transformation Pipeline:

	emails, err := stream.FromSlice(usernames).
		Filter(func(name string) bool { return len(name) > 3 }).
		Map(func(name string) string {
			return strings.ToLower(name) + "@company.com"
		}).
		ToSlice(ctx)

Aggregation and Statistics:

	numbers := stream.FromSlice([]int{1, 2, 3, 4, 5})

	count, _ := numbers.Count(ctx)
	sum, _ := numbers.Reduce(ctx, 0, func(a, b int) int { return a + b })
	min, _, _ := numbers.Min(ctx, intComparator)
	max, _, _ := numbers.Max(ctx, intComparator)

Text Processing:

	words, err := stream.FromSlice(strings.Fields(text)).
		Filter(func(word string) bool { return len(word) > 2 }).
		Map(func(word string) string { return strings.ToLower(word) }).
		Distinct().
		Sorted(func(a, b string) int { return strings.Compare(a, b) }).
		ToSlice(ctx)

Infinite Streams:

	// Generate fibonacci sequence
	a, b := 0, 1
	fibonacci := stream.Generate(func() int {
		a, b = b, a+b
		return a
	})

	first10, _ := fibonacci.Limit(10).ToSlice(ctx)

Error Handling:

All terminal operations return errors that should be checked:

	result, err := stream.ToSlice(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println("Operation timed out")
		} else if errors.Is(err, stream.ErrStreamClosed) {
			log.Println("Stream was closed")
		} else {
			log.Printf("Stream error: %v", err)
		}
		return
	}

Context and Cancellation:

All terminal operations accept a context for cancellation and timeouts:

	// With timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := stream.ToSlice(ctx)

	// With cancellation
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(time.Second)
		cancel() // Cancel after 1 second
	}()

	err := stream.ForEach(ctx, processFunction)

Resource Management:

Streams should be closed to release resources:

	stream := stream.FromSlice(data)
	defer stream.Close() // Always close streams

	// Or use in function scope
	func processData(data []int) error {
		stream := stream.FromSlice(data)
		defer stream.Close()

		return stream.ForEach(ctx, process)
	}

Performance Characteristics:

  - Lazy evaluation: operations are only executed when terminal operations are called
  - Memory efficient: elements are processed one at a time in most cases
  - Some operations (like Sort) require collecting all elements
  - Parallel operations: future versions may support parallel processing
  - Overhead: functional style adds some overhead compared to imperative loops

Common Patterns:

Data Validation Pipeline:

	validData, err := stream.FromSlice(rawData).
		Filter(func(item DataItem) bool { return item.IsValid() }).
		Map(func(item DataItem) DataItem { return item.Normalize() }).
		Filter(func(item DataItem) bool { return item.PassesBusinessRules() }).
		ToSlice(ctx)

ETL (Extract, Transform, Load):

	err := stream.FromChannel(inputChannel).
		Map(func(raw RawData) ProcessedData { return transform(raw) }).
		Filter(func(data ProcessedData) bool { return data.IsComplete() }).
		ForEach(ctx, func(data ProcessedData) {
			database.Save(data)
		})

Statistical Analysis:

	stats := Stats{}
	stream.FromSlice(measurements).ForEach(ctx, func(m Measurement) {
		stats.Add(m)
	})

	mean := stats.Mean()
	stddev := stats.StandardDeviation()

Word Frequency Analysis:

	frequencies, err := stream.FromSlice(strings.Fields(text)).
		Map(func(word string) string { return strings.ToLower(word) }).
		Collect(ctx,
			func() interface{} { return make(map[string]int) },
			func(acc interface{}, word string) {
				acc.(map[string]int)[word]++
			},
			func(acc1, acc2 interface{}) interface{} {
				// Combine maps for parallel processing
				return combineMaps(acc1, acc2)
			},
		)

Best Practices:

 1. Always close streams to release resources:
    defer stream.Close()

 2. Check errors from terminal operations:
    result, err := stream.ToSlice(ctx)
    if err != nil {
    // handle error
    }

 3. Use appropriate context timeouts:
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)

 4. Chain operations efficiently:
    // Good: single chain
    result := stream.Filter(p1).Map(f1).Filter(p2).ToSlice(ctx)

    // Avoid: multiple intermediate variables
    s1 := stream.Filter(p1)
    s2 := s1.Map(f1)
    s3 := s2.Filter(p2)

 5. Use Peek for debugging:
    stream.Peek(func(x T) { log.Printf("Debug: %v", x) })

 6. Consider memory implications of operations:
    // Memory-efficient (streaming)
    stream.Filter(predicate).ForEach(ctx, action)

    // Memory-intensive (collects all)
    stream.Sorted(comparator).ToSlice(ctx)

 7. Handle infinite streams carefully:
    stream.Generate(generator).Limit(1000).ToSlice(ctx) // Always limit

Comparison with Alternatives:

vs Traditional Loops:
  - More expressive and readable
  - Built-in error handling and context support
  - Functional composition
  - Some performance overhead

vs Channels:
  - Higher-level abstraction
  - Rich set of operations
  - Better error handling
  - Less flexible for complex flow control

vs Iterator Pattern:
  - Lazy evaluation
  - Method chaining
  - Context-aware operations
  - Resource management

Thread Safety:

Individual stream instances are not thread-safe and should not be shared between goroutines.
However, you can create multiple streams from the same source safely:

	// Safe: separate streams
	go processStream(stream.FromSlice(data).Filter(predicate1))
	go processStream(stream.FromSlice(data).Filter(predicate2))

	// Unsafe: shared stream
	s := stream.FromSlice(data)
	go s.Filter(predicate1).ForEach(ctx1, action1) // Don't do this
	go s.Filter(predicate2).ForEach(ctx2, action2) // Don't do this

Integration with Other Packages:

The stream package integrates well with other goflow components:

	// With worker pool for parallel processing
	pool := workerpool.New(4, 100)
	err := stream.FromSlice(tasks).ForEach(ctx, func(task Task) {
		pool.Submit(workerpool.TaskFunc(func(ctx context.Context) error {
			return processTask(task)
		}))
	})

	// With pipeline for complex processing
	pipeline := pipeline.New().
		AddStageFunc("extract", extractData).
		AddStageFunc("transform", func(ctx context.Context, input interface{}) (interface{}, error) {
			data := input.([]DataItem)
			processed, err := stream.FromSlice(data).
				Filter(isValid).
				Map(normalize).
				ToSlice(ctx)
			return processed, err
		}).
		AddStageFunc("load", loadData)

Future Enhancements:

Planned features for future versions:
  - Parallel stream processing
  - More built-in collectors
  - Stream splitting and merging
  - Integration with Go generics improvements
  - Performance optimizations
*/
package stream
