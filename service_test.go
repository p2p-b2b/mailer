package mailer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// Helper to create valid MailContent using the builder
func createTestMail(subject string) MailContent {
	// Use the builder and provide valid minimal values for required fields
	builder := MailContentBuilder{}
	// Note: Assuming fromAddress validation requires a basic format. Adjust if needed.
	content, err := builder.
		WithFromName("Test Sender").
		WithFromAddress("sender@example.com").
		WithToName("Test Recipient").
		WithToAddress("recipient@example.com").
		WithMimeType("text/plain"). // Use a valid MIME type from constants
		WithSubject(subject).
		WithBody("Test body content that is long enough."). // Ensure body meets min length
		Build()
	if err != nil {
		// In a test helper, panicking is often acceptable if creation MUST succeed
		// for the test setup.
		panic(fmt.Sprintf("createTestMail failed for subject '%s': %v", subject, err))
	}
	return content
}

// MockMailerService is a mock implementation of MailerService for testing.
type MockMailerService struct {
	mu       sync.Mutex
	SentMail []MailContent
	SendFunc func(ctx context.Context, content MailContent) error
	SendErr  error // Predefined error to return from Send
}

// Send records the mail content and returns a predefined error or the result of SendFunc.
func (m *MockMailerService) Send(ctx context.Context, content MailContent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate some work
	time.Sleep(5 * time.Millisecond)

	m.SentMail = append(m.SentMail, content)

	if m.SendFunc != nil {
		return m.SendFunc(ctx, content)
	}
	return m.SendErr
}

// Reset clears the recorded sent mail.
func (m *MockMailerService) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentMail = nil
	m.SendErr = nil
	m.SendFunc = nil
}

// Count returns the number of emails "sent".
func (m *MockMailerService) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.SentMail)
}

// --- Test Cases ---

func TestNewMailService(t *testing.T) {
	mockMailer := &MockMailerService{}
	ctx := context.Background()

	tests := []struct {
		name        string
		config      *MailServiceConfig
		expectError bool
		checkFunc   func(*testing.T, *MailService) // Optional check on the created service
	}{
		{
			name: "Valid config",
			config: &MailServiceConfig{
				Ctx:         ctx,
				WorkerCount: 5,
				Timeout:     10 * time.Second,
				Mailer:      mockMailer,
			},
			expectError: false,
			checkFunc: func(t *testing.T, s *MailService) {
				if s == nil {
					t.Fatal("Expected non-nil MailService")
				}
				if s.workerCount != 5 {
					t.Errorf("Expected workerCount 5, got %d", s.workerCount)
				}
				if cap(s.content) != 5 {
					t.Errorf("Expected channel capacity 5, got %d", cap(s.content))
				}
				if s.mailer != mockMailer {
					t.Error("Mailer not set correctly")
				}
			},
		},
		{
			name:        "Nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "Nil mailer",
			config: &MailServiceConfig{
				Ctx:         ctx,
				WorkerCount: 1,
				Mailer:      nil,
			},
			expectError: true,
		},
		{
			name: "Worker count too low",
			config: &MailServiceConfig{
				Ctx:         ctx,
				WorkerCount: 0,
				Mailer:      mockMailer,
			},
			expectError: true,
		},
		{
			name: "Worker count too high",
			config: &MailServiceConfig{
				Ctx:         ctx,
				WorkerCount: ValidMaxWorkerCount + 1,
				Mailer:      mockMailer,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewMailService(tt.config)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
				// Check if it's the specific error type if needed
				if _, ok := err.(*MailQueueError); !ok && err != nil {
					t.Errorf("Expected error of type *MailQueueError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error, but got: %v", err)
				}
				if tt.checkFunc != nil {
					tt.checkFunc(t, service)
				}
			}
		})
	}
}

func TestMailService_StartStopEnqueue(t *testing.T) {
	mockMailer := &MockMailerService{}
	config := &MailServiceConfig{
		WorkerCount: 3,
		Mailer:      mockMailer,
	}
	service, err := NewMailService(config)
	if err != nil {
		t.Fatalf("Failed to create MailService: %v", err)
	}

	service.Start()

	// Enqueue some emails
	numEmails := 10
	for i := range numEmails {
		service.Enqueue(createTestMail(fmt.Sprintf("Test %d", i)))
	}

	// Stop the service. This will close the channel and wait (using the internal wg)
	// for all workers to finish processing the enqueued items.
	service.Stop() // This now blocks until workers are done.

	// Final verification - workers should have processed all items before Stop returned.
	if mockMailer.Count() != numEmails {
		t.Errorf("Expected %d emails to be sent, but final count is %d", numEmails, mockMailer.Count())
	}
}

func TestMailService_ContextCancellation(t *testing.T) {
	mockMailer := &MockMailerService{}
	processedCount := 0
	var countMu sync.Mutex

	// Simplified SendFunc: Simulate work but respect context cancellation.
	mockMailer.SendFunc = func(ctx context.Context, content MailContent) error {
		// Simulate work that can be interrupted by context cancellation
		select {
		case <-time.After(25 * time.Millisecond): // Simulate slightly longer work
			// Work completed without cancellation
			t.Logf("Send completed work for mail content")
		case <-ctx.Done():
			// Work interrupted by cancellation
			t.Logf("Send cancelled during work for mail content")
			return ctx.Err() // Return context error if cancelled
		}

		// If work completed, record the send
		mockMailer.mu.Lock()
		mockMailer.SentMail = append(mockMailer.SentMail, content)
		countMu.Lock()
		processedCount++
		countMu.Unlock()
		mockMailer.mu.Unlock()
		t.Logf("Send completed for mail content") // Removed content.Subject
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	config := &MailServiceConfig{
		Ctx:         ctx,
		WorkerCount: 2, // Fewer workers than emails
		Mailer:      mockMailer,
	}
	service, err := NewMailService(config)
	if err != nil {
		t.Fatalf("Failed to create MailService: %v", err)
	}

	// We need a separate context for the enqueue goroutine that we don't cancel immediately
	// Or simply let the enqueue goroutine run until it blocks or finishes.
	service.Start() // Workers use the cancellable context

	// Run enqueue loop in a separate goroutine.
	var enqueueWg sync.WaitGroup
	enqueueWg.Add(1)
	go func() {
		defer enqueueWg.Done()
		const numEmails = 5 // Define numEmails inside the goroutine
		enqueuedCount := 0
		for i := range numEmails {
			// Use the service's context for enqueueing, so it respects cancellation.
			err := service.Enqueue(createTestMail(fmt.Sprintf("CtxTest %d", i)))
			if err != nil {
				// Log expected errors due to cancellation/blocking
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					t.Logf("Enqueue goroutine: Enqueue failed as expected due to context cancellation: %v", err)
				} else {
					// Log unexpected errors but don't fail the test here, let main check handle final state
					t.Logf("Enqueue goroutine: Unexpected error during enqueue %d: %v", i, err)
				}
				break // Stop trying to enqueue more emails if an error occurs
			}
			enqueuedCount++
		}
		t.Logf("Enqueue goroutine: Finished attempting to enqueue %d emails.", enqueuedCount)
	}()

	// Wait a tiny bit for workers to pick up initial tasks AND for enqueue goroutine to potentially block
	time.Sleep(20 * time.Millisecond)

	// Cancel the context
	t.Log("Cancelling context...")
	cancel()
	t.Log("Context cancelled.")

	// No blocker channel to close anymore.

	// Wait for workers to exit due to context cancellation using the service's WaitGroup.
	// This ensures we check the state *after* workers have reacted to the cancellation.
	service.Wait() // Wait for workers to finish without closing the channel.
	t.Log("Workers finished after context cancellation.")

	// Check that not all emails were processed
	// Note: We can't use numEmails here as it's defined in the goroutine.
	// The assertion should focus on whether *some* processing happened but was likely cut short.
	// A more robust check might involve knowing exactly how many *should* have completed before cancellation.
	// For now, we check if the count is less than the intended total.
	const expectedTotalEmails = 5 // Re-declare for assertion clarity
	countMu.Lock()
	finalSentCount := processedCount // Use the counter updated within SendFunc
	countMu.Unlock()

	if finalSentCount == expectedTotalEmails {
		t.Errorf("Expected fewer than %d emails sent due to context cancellation, but got %d", expectedTotalEmails, finalSentCount)
	}
	if finalSentCount == 0 && expectedTotalEmails > 0 {
		t.Logf("Warning: 0 emails sent, cancellation might have been very quick or blocking logic needs review.")
	} else if finalSentCount > 0 {
		t.Logf("Successfully processed %d emails before/during context cancellation.", finalSentCount)
	} else {
		t.Logf("Processed count is %d", finalSentCount)
	}
}

// Test for enqueue blocking when buffer is full
func TestMailService_EnqueueBlocksWhenFull(t *testing.T) {
	mockMailer := &MockMailerService{}
	workerCount := 1 // Use a small buffer size (equal to worker count)
	config := &MailServiceConfig{
		Ctx:         t.Context(),
		WorkerCount: workerCount,
		Mailer:      mockMailer,
	}
	service, err := NewMailService(config)
	if err != nil {
		t.Fatalf("Failed to create MailService: %v", err)
	}

	// DO NOT START the service, so workers don't consume from the channel.
	// This ensures the channel buffer can be filled.

	// Fill the buffer completely
	t.Logf("Filling buffer with capacity %d", workerCount)
	for i := range workerCount {
		mail := createTestMail(fmt.Sprintf("Fill %d", i))
		service.Enqueue(mail)
		t.Logf("Enqueued mail %d", i) // Removed mail.Subject
	}
	t.Logf("Buffer should now be full.")

	// Try to enqueue one more item. This should block because the buffer is full
	// and no workers are running to consume items.
	enqueueDone := make(chan bool)
	t.Logf("Attempting to enqueue one more item (expected to block)...")
	go func() {
		mail := createTestMail("Blocking")
		service.Enqueue(mail)
		// Cannot log mail.Subject here as it's unexported
		t.Logf("Enqueued blocking mail (THIS SHOULD NOT HAPPEN if blocking works)")
		close(enqueueDone) // Signal completion ONLY if Enqueue returns (which it shouldn't)
	}()

	// Wait for a short period. If enqueueDone is closed, it means Enqueue didn't block.
	// If the timeout expires, it means Enqueue blocked as expected.
	select {
	case <-enqueueDone:
		t.Errorf("Enqueue did not block when the buffer was full and no workers were running")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior: Enqueue blocked.
		t.Log("Enqueue correctly blocked as expected.")
	}

	// Cleanup: Start and stop the service to drain the channel and allow the blocked goroutine to potentially finish.
	// This prevents the test leaving a goroutine blocked on the channel send.
	t.Log("Cleaning up: Starting service to drain channel...")
	service.Start()
	// Allow time for the worker to potentially process the items
	time.Sleep(50 * time.Millisecond)
	t.Log("Cleaning up: Stopping service...")
	service.Stop()
	// Allow time for worker shutdown
	time.Sleep(50 * time.Millisecond)
	t.Log("Cleanup complete.")
}

// ExampleMailService_Enqueue demonstrates how to create a MailService,
// enqueue an email, and have it processed by a mock mailer.
func ExampleMailService_Enqueue() {
	// Use a mock mailer for demonstration purposes
	mockMailer := &MockMailerService{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is cancelled eventually

	// Configure the mail service
	config := &MailServiceConfig{
		Ctx:         ctx,
		WorkerCount: 1, // One worker is sufficient for the example
		Mailer:      mockMailer,
	}
	service, err := NewMailService(config)
	if err != nil {
		fmt.Printf("Error creating service: %v\n", err)
		return
	}

	service.Start()

	// Create email content using the builder
	content, err := (&MailContentBuilder{}).
		WithFromName("Example Sender").
		WithFromAddress("sender@example.net").
		WithToName("Example Recipient").
		WithToAddress("recipient@example.net").
		WithMimeType("text/plain").
		WithSubject("Example Subject").
		WithBody("This is the example email body.").
		Build()
	if err != nil {
		fmt.Printf("Error building content: %v\n", err)
		return
	}

	// Enqueue the email
	err = service.Enqueue(content)
	if err != nil {
		fmt.Printf("Error enqueuing email: %v\n", err)
		// Don't return here, allow Stop to run for cleanup
	}

	// Stop the service gracefully. This ensures the enqueued item is processed
	// by the worker before the example finishes.
	service.Stop()

	// Check if the mock mailer received the email
	if mockMailer.Count() > 0 {
		// Accessing unexported fields like subject directly isn't possible here.
		// We'll print a confirmation message based on the count.
		// In a real test, you might check mockMailer.SentMail[0].subject if it were exported
		// or add a method to MockMailerService to get sent subjects.
		fmt.Printf("Mock mailer received %d email(s).\n", mockMailer.Count())
	} else {
		fmt.Println("Mock mailer did not receive the email.")
	}

	// Output:
	// Mock mailer received 1 email(s).
}
