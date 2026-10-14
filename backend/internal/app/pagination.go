package app

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// Page est une page de résultats triés du plus récent au plus ancien.
type Page[T any] struct {
	Items      []T
	NextCursor string
}

var errCursor = invalid("curseur-invalide", "Le curseur de pagination est invalide.")

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return defaultPageSize
	case n > maxPageSize:
		return maxPageSize
	}
	return n
}

// cursor repère la dernière ligne vue : (date, id), pour un tri stable.
type cursor struct {
	At time.Time
	ID uuid.UUID
}

func (c cursor) encode() string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.At.Format(time.RFC3339Nano) + "|" + c.ID.String()))
}

// decodeCursor renvoie nil pour un curseur vide.
func decodeCursor(s string) (*cursor, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, errCursor
	}
	at, id, ok := strings.Cut(string(b), "|")
	if !ok {
		return nil, errCursor
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return nil, errCursor
	}
	u, err := uuid.Parse(id)
	if err != nil {
		return nil, errCursor
	}
	return &cursor{At: t, ID: u}, nil
}

// cursorArgs renvoie les paramètres SQL d'un curseur (NULL si absent).
func cursorArgs(c *cursor) (*time.Time, *uuid.UUID) {
	if c == nil {
		return nil, nil
	}
	return &c.At, &c.ID
}
