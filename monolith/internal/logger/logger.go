package logger

import (
	"context"
	"log/slog"
	"os"
)

// A global logger instance that can be used throughout the application.
var Log *slog.Logger

// What information do we need?
const CorrelationIDKey = "correlation_id"

// What is the scope?
// The correlation ID is request-scoped, so it should not be stored globally.
// Instead, we attach the correlation ID to the context, and functions that
// need it should receive that context.
//
// Our application therefore needs:
// 1. A logger.
// 2. A context containing the correlation ID.
//
// Go provides the log/slog package for structured logging.

// InitLogger creates a JSON logger and sends its output to stdout.
//
// Example:
// logger.Info(ctx, "user logged in", "user_id", userID)
//
// This produces a structured JSON log.
//
// Someone has to create the logger using slog.New().
func InitLogger() {
	// Give us Go's built-in handler that:
	// 1. Formats logs as JSON.
	// 2. Writes the output to stdout.
	// 3. Uses HandlerOptions to configure additional logging behavior.
	//
	// NewJSONHandler creates a Handler.
	//
	// A Handler's job is to:
	// 1. Take a log record.
	// 2. Format the record.
	// 3. Write the formatted record to its destination.
	//
	// There are different types of handlers, such as JSON, text, console, etc.
	// We are using the JSON handler here.
	//
	// The first argument is an io.Writer.
	// os.Stdout represents the program's standard output stream.
	//
	// NewJSONHandler receives an io.Writer, and os.Stdout is the destination
	// to which the handler writes the formatted log output.
	//
	// HandlerOptions is where we configure the behavior of the handler.
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	})

	// Pass the handler to slog to create a logger instance.
	Log = slog.New(handler)

	// Make this exact logger the default logger used by the slog package.
	slog.SetDefault(Log)
}

// getLogArgs is a helper function that automatically takes the correlation ID
// out of the context and adds it to the log arguments.
//
// Whenever somebody logs something, the correlation ID from the request
// context is automatically attached to the log.
//
// slog logging arguments can contain different types, for example:
//
// Log.Info(
//
//	"user created",
//	"user_id", 123,              // int
//	"email", "test@example.com", // string
//	"premium", true,             // bool
//
// )
func getLogArgs(ctx context.Context, args []any) []any {
	if ctx != nil {
		//context.Value() returns any but i expect this to contain string: this is type assertion
		if cid, ok := ctx.Value(CorrelationIDKey).(string); ok {
			return append(args, slog.String("correlation_id", cid))
		}
	}
	return args
}

// These functions are wrappers around the slog logger.
//
// The purpose of these wrappers is to automatically attach the correlation ID
// from the context to the log arguments.
//
// Without these wrappers, we would repeatedly have to:
// 1. Get the correlation ID from the context.
// 2. Add the correlation ID to the log arguments.
// 3. Call the slog logger.
//
// We avoid that repetition by using getLogArgs() inside these wrapper functions.
func Info(ctx context.Context, msg string, args ...any) {
	argsWithCID := getLogArgs(ctx, args)
	Log.InfoContext(ctx, msg, argsWithCID...)
}

func Error(ctx context.Context, msg string, args ...any) {
	argsWithCID := getLogArgs(ctx, args)
	Log.ErrorContext(ctx, msg, argsWithCID...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	argsWithCID := getLogArgs(ctx, args)
	Log.WarnContext(ctx, msg, argsWithCID...)
}
