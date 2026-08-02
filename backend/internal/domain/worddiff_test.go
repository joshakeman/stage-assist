package domain_test

import (
	"reflect"
	"testing"

	"github.com/joshakeman/stage-assist/backend/internal/domain"
)

func TestWordDiffIdentical(t *testing.T) {
	a := domain.Tokenize("To be or not to be")
	b := domain.Tokenize("To be or not to be")

	got := domain.WordDiff(a, b)
	want := []domain.WordDiffSpan{{Op: domain.OpEqual, Text: "To be or not to be"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WordDiff(identical) = %+v, want %+v", got, want)
	}
}

func TestWordDiffSubstitution(t *testing.T) {
	a := domain.Tokenize("Whether tis nobler")
	b := domain.Tokenize("Whether it's nobler")

	got := domain.WordDiff(a, b)
	want := []domain.WordDiffSpan{
		{Op: domain.OpEqual, Text: "Whether"},
		{Op: domain.OpDelete, Text: "tis"},
		{Op: domain.OpInsert, Text: "it's"},
		{Op: domain.OpEqual, Text: "nobler"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WordDiff(substitution) = %+v, want %+v", got, want)
	}
}
