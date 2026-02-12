package io

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestPrint_Output(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Print("hello")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	expected := fmt.Sprintln("[SDLC]", "hello")
	if got != expected {
		t.Errorf("Print(\"hello\") output = %q, want %q", got, expected)
	}
}

func TestPrint_EmptyMessage(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Print("")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	expected := fmt.Sprintln("[SDLC]", "")
	if got != expected {
		t.Errorf("Print(\"\") output = %q, want %q", got, expected)
	}
}

func TestPrint_SpecialCharacters(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Print("error: file not found!")

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	expected := fmt.Sprintln("[SDLC]", "error: file not found!")
	if got != expected {
		t.Errorf("Print(\"error: file not found!\") output = %q, want %q", got, expected)
	}
}
