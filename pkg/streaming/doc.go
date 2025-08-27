/*
Package streaming offers a new take on streaming data, providing higher-level
abstractions than standard Go readers and writers.

This package provides three main streaming components:

  - stream: Data stream API for creating and consuming data streams
  - writer: Asynchronous writer that buffers data and writes in background
  - backpressure: Backpressure-aware channels to prevent resource exhaustion

Basic usage:

	// Create an asynchronous writer
	writer := writer.New(underlyingWriter, bufferSize)
	defer writer.Close()

	// Write data asynchronously
	writer.Write(data)

All streaming components support common operations like filtering, mapping,
and reducing, with built-in error handling and cancellation support.
*/
package streaming
