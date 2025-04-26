package mailer

import (
	"context"
	"errors" // Import errors package
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	ValidMaxWorkerCount = 100
	ValidMinWorkerCount = 1
)

type MailQueueError struct {
	Message string
}

func (e *MailQueueError) Error() string {
	return e.Message
}

type MailQueueService interface {
	Enqueue(content MailContent) error
}

type MailServiceConfig struct {
	Ctx         context.Context
	WorkerCount int
	Timeout     time.Duration
	Mailer      MailerService
}

type MailService struct {
	ctx         context.Context
	workerCount int
	content     chan MailContent
	mailer      MailerService
	wg          sync.WaitGroup
	mu          sync.RWMutex // Mutex to protect access to ctx
}

func NewMailService(conf *MailServiceConfig) (*MailService, error) {
	if conf == nil {
		return nil, &MailQueueError{Message: "MailServiceConfig cannot be nil"}
	}

	if conf.Mailer == nil {
		return nil, &MailQueueError{Message: "Mailer cannot be nil"}
	}

	if conf.WorkerCount < ValidMinWorkerCount || conf.WorkerCount > ValidMaxWorkerCount {
		return nil, &MailQueueError{Message: fmt.Sprintf("WorkerCount must be between %d and %d", ValidMinWorkerCount, ValidMaxWorkerCount)}
	}

	// Create a buffered channel with size equal to the worker count
	contentChan := make(chan MailContent, conf.WorkerCount)

	// Ensure the service has a non-nil context internally, even if Start isn't called.
	internalCtx := conf.Ctx
	if internalCtx == nil {
		internalCtx = context.Background()
	}

	return &MailService{
		ctx:         internalCtx, // Use the initialized context
		workerCount: conf.WorkerCount,
		content:     contentChan,
		mailer:      conf.Mailer,
		wg:          sync.WaitGroup{},
	}, nil
}

func (ref *MailService) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	ref.mu.Lock()
	ref.ctx = ctx
	ref.mu.Unlock()

	slog.Info("Starting mail service", "workerCount", ref.workerCount, "bufferSize", cap(ref.content))

	// Start worker goroutines
	for i := 1; i <= ref.workerCount; i++ {
		ref.wg.Add(1) // Increment WaitGroup for each worker we start
		// Launch worker as an anonymous goroutine
		go func(workerNum int) {
			defer ref.wg.Done() // Ensure WaitGroup is decremented when worker exits

			// Capture the context under read lock before starting the loop
			ref.mu.RLock()
			workerCtx := ref.ctx
			ref.mu.RUnlock()

			slog.Info("Worker started", "worker", workerNum)
			for {
				// Main select loop: prioritize context cancellation check using the captured context.
				select {
				case <-workerCtx.Done():
					// Context was cancelled, exit the worker loop.
					slog.Warn("Context done, worker stopping", "worker", workerNum)
					return
				case content, ok := <-ref.content:
					// Attempt to read from the content channel.
					if !ok {
						// Channel is closed, exit the worker loop.
						slog.Warn("Content channel closed, worker stopping", "worker", workerNum)
						return
					}

					// Process the mail content if read successfully.
					slog.Info("Sending email", "worker", workerNum)
					// Send should ideally respect the context internally as well. Use the captured context.
					err := ref.mailer.Send(workerCtx, content)
					if err != nil {
						// Log errors, checking specifically for context-related errors from Send. Use the captured context.
						if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
							slog.Warn("Email sending cancelled by context", "worker", workerNum, "error", err)
						} else {
							slog.Error("Error sending email", "worker", workerNum, "error", err)
						}
					}
					// Loop continues, will check ctx.Done again first in the next iteration.
				}
			}
		}(i)
	}
}

// Stop closes the content channel and waits for all workers to finish processing.
func (ref *MailService) Stop() {
	slog.Warn("Stopping mail service, closing content channel...")

	close(ref.content)

	slog.Warn("Waiting for workers to finish...")
	ref.wg.Wait() // Wait for all worker goroutines to call wg.Done()
	slog.Warn("All workers finished.")
}

// Wait blocks until all worker goroutines have exited.
// Useful if you cancel the context and want to ensure workers stopped
// without necessarily closing the channel via Stop().
func (ref *MailService) Wait() {
	ref.wg.Wait()
}

// Enqueue adds mail content to the queue. It returns an error if the service's context is cancelled during the attempt.
func (ref *MailService) Enqueue(content MailContent) error {
	slog.Info("Attempting to enqueue email")

	// Acquire read lock to safely access ctx
	ref.mu.RLock()
	ctxDone := ref.ctx.Done()
	ctxErr := ref.ctx.Err() // Capture error under lock too, though less critical
	ref.mu.RUnlock()

	select {
	case ref.content <- content:
		slog.Info("Email enqueued successfully")
		return nil // Successfully enqueued
	case <-ctxDone:
		// Context was cancelled while trying to enqueue (likely because buffer was full and workers were stopping).
		slog.Warn("Failed to enqueue email, context cancelled", "error", ctxErr)
		return ctxErr
	}
}
