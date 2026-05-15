// Package logging re-exports request logging interfaces for SDK consumers.
package logging

import internallogging "github.com/router-for-me/CLIProxyAPI/v7/internal/logging"

// RequestLogger defines the interface for logging HTTP requests and responses.
type RequestLogger = internallogging.RequestLogger

// StreamingLogWriter handles real-time logging of streaming response chunks.
type StreamingLogWriter = internallogging.StreamingLogWriter
