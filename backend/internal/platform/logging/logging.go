// Package logging configure le journal JSON du service.
package logging

import (
	"io"
	"log/slog"
)

// New renvoie un logger JSON au niveau demandé, avec le nom du service.
func New(w io.Writer, level slog.Level, service string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	return slog.New(h).With(slog.String("service", service))
}
