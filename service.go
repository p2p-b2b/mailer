package mailer

import (
	"context"
	"errors"
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
	Ctx context.Context

	// WorkerCount is the number of concurrent workers to process the mail queue.
	// It must be between 1 and 100.
	WorkerCount int

	// Timeout is the duration for which the service will wait for a worker to finish processing before timing out.
	// This is currently not used in the core service logic but can be implemented in the future.
	Timeout time.Duration

	// Mailer is the service responsible for sending emails.
	// It must not be nil.
	Mailer MailerService
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
		return nil, &MailQueueError{
			Message: "MailServiceConfig cannot be nil",
		}
	}

	if conf.Mailer == nil {
		return nil, &MailQueueError{
			Message: "Mailer cannot be nil",
		}
	}

	if conf.WorkerCount < ValidMinWorkerCount || conf.WorkerCount > ValidMaxWorkerCount {
		return nil, &MailQueueError{
			Message: fmt.Sprintf("WorkerCount must be between %d and %d", ValidMinWorkerCount, ValidMaxWorkerCount),
		}
	}

	contentChan := make(chan MailContent, conf.WorkerCount)

	internalCtx := conf.Ctx
	if internalCtx == nil {
		internalCtx = context.Background()
	}

	return &MailService{
		ctx:         internalCtx,
		workerCount: conf.WorkerCount,
		content:     contentChan,
		mailer:      conf.Mailer,
		wg:          sync.WaitGroup{},
	}, nil
}

// Start initializes the worker goroutines that will process the mail queue.
// Each worker will listen on the content channel for new mail content to process.
// The workers will run concurrently, and the number of workers is determined by the WorkerCount field.
// The context provided in the configuration is used to manage the lifecycle of the workers.
// If the context is cancelled, all workers will stop processing and exit gracefully.
// The workers will log their status and any errors encountered during the email sending process.
func (ref *MailService) Start() {
	for i := 1; i <= ref.workerCount; i++ {
		ref.wg.Add(1)

		go func(workerNum int) {
			defer ref.wg.Done()

			ref.mu.RLock()
			workerCtx := ref.ctx
			ref.mu.RUnlock()

			slog.Info("Worker started", "worker", workerNum)
			for {
				select {
				case <-workerCtx.Done():
					slog.Warn("Context done, worker stopping", "worker", workerNum)
					return
				case content, ok := <-ref.content:
					if !ok {
						slog.Warn("Content channel closed, worker stopping", "worker", workerNum)
						return
					}

					slog.Info("Sending email", "worker", workerNum)
					err := ref.mailer.Send(workerCtx, content)
					if err != nil {
						if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
							slog.Warn("Email sending cancelled by context", "worker", workerNum, "error", err)
						} else {
							slog.Error("Error sending email", "worker", workerNum, "error", err)
						}
					}
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
	ref.wg.Wait()
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
	ctxErr := ref.ctx.Err()
	ref.mu.RUnlock()

	select {
	case ref.content <- content:
		slog.Info("Email enqueued successfully")

		return nil
	case <-ctxDone:
		slog.Warn("Failed to enqueue email, context cancelled", "error", ctxErr)
		return ctxErr
	}
}
