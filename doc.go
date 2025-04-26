/*
Package mailer provides a robust and concurrent email sending service for Go applications.

It features a queue-based system (`MailService`) that utilizes a pool of worker goroutines
to send emails asynchronously. This allows applications to enqueue emails quickly without
blocking on the actual sending process.

Key Features:

  - Concurrent email sending via a configurable worker pool.
  - Buffered queue for email content (`MailContent`).
  - Graceful shutdown using context cancellation and wait groups.
  - Pluggable backend architecture via the `MailerService` interface.
  - Includes a standard SMTP implementation (`MailerSMTP`).
  - Provides a builder (`MailContentBuilder`) for validated email content creation.

Usage typically involves:

 1. Configuring a `MailerService` implementation (e.g., `NewMailerSMTP`).
 2. Configuring the main `MailService` with the desired worker count and the chosen mailer backend.
 3. Starting the `MailService` with a context.
 4. Enqueuing `MailContent` instances using the `Enqueue` method.
 5. Stopping the service gracefully or relying on context cancellation for shutdown.

See the `ExampleMailService_Enqueue` function in the tests for a practical usage demonstration.
*/
package mailer
