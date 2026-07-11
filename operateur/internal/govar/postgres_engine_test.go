package govar

import (
	"context"
	"testing"
)

func TestPostgresEngineRejectsEmptyURL(t *testing.T) {
	if _, err := NewPostgresEngine(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty database url")
	}
}
