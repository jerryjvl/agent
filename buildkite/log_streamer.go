package buildkite

import (
	"github.com/buildkite/agent/buildkite/logger"
	"github.com/goinggo/work"
	"math"
	"time"
)

type LogStreamer struct {
	// The client we should use to stream the logs
	Client Client

	// How many log streamer workers are running at any one time
	Concurrency int

	// The streaming work queue
	Queue *work.Work

	// Total size in bytes of the log
	bytes int

	// The chunks of the log
	chunks []*LogStreamerChunk

	// Each chunk is assigned an order
	order int
}

func workLoggingFunction(message string) {
	// logger.Debug("Worker: %s", message)
}

func NewLogStreamer(client *Client) (*LogStreamer, error) {
	// Create a new log streamer and default the concurrency to 5, seems
	// like a good number?
	streamer := new(LogStreamer)
	streamer.Concurrency = 5

	return streamer, nil
}

func (streamer *LogStreamer) Start() error {
	// Create a new work queue
	w, err := work.New(streamer.Concurrency, time.Second, workLoggingFunction)
	if err != nil {
		return err
	}
	streamer.Queue = w

	return nil
}

// Takes the full process output, grabs the portion we don't have, and adds it
// to the stream queue
func (streamer *LogStreamer) Process(output string) error {
	bytes := len(output)

	if streamer.bytes != bytes {
		maximumBlobSize := 100000

		// Grab the part of the log that we haven't seen yet
		blob := output[streamer.bytes:bytes]

		// How many 100kb chunks are in the blob?
		numberOfChunks := int(math.Ceil(float64(len(blob)) / float64(maximumBlobSize)))

		for i := 0; i < numberOfChunks; i++ {
			// Find the upper limit of the blob
			upperLimit := (i + 1) * maximumBlobSize
			if upperLimit > len(blob) {
				upperLimit = len(blob)
			}

			// Grab the 100kb section of the blob
			partialBlob := blob[i*maximumBlobSize : upperLimit]

			// Increment the order
			streamer.order += 1

			logger.Debug("Creating %d byte chunk", len(partialBlob))

			// Create the chunk and append it to our list
			chunk := LogStreamerChunk{
				Order: streamer.order,
				Blob:  partialBlob,
				Bytes: len(partialBlob),
			}

			// Append the chunk to our list
			streamer.chunks = append(streamer.chunks, &chunk)

			// Create the worker and run it
			worker := LogStreamerWorker{Chunk: &chunk}
			go func() {
				// Add the chunk worker to the queue. Run will block until it
				// is successfully added to the queue.
				streamer.Queue.Run(&worker)
			}()
		}

		// Save the new amount of bytes
		streamer.bytes = bytes
	}

	return nil
}

func (streamer *LogStreamer) Stop() error {
	logger.Debug("Waiting for the log streaming workers to finish")

	streamer.Queue.Shutdown()

	return nil
}
