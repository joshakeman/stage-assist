package domain

import (
	"reflect"
	"testing"
)

func eq(x, y string) bool { return x == y }

func TestLcsDiffIdentical(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "b", "c"}

	got := lcsDiff(a, b, eq)
	want := []opStep{
		{Op: OpEqual, AIdx: 0, BIdx: 0},
		{Op: OpEqual, AIdx: 1, BIdx: 1},
		{Op: OpEqual, AIdx: 2, BIdx: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lcsDiff(identical) = %+v, want %+v", got, want)
	}
}

func TestLcsDiffDisjoint(t *testing.T) {
	a := []string{"x", "y"}
	b := []string{"p", "q"}

	got := lcsDiff(a, b, eq)
	want := []opStep{
		{Op: OpDelete, AIdx: 0},
		{Op: OpDelete, AIdx: 1},
		{Op: OpInsert, BIdx: 0},
		{Op: OpInsert, BIdx: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lcsDiff(disjoint) = %+v, want %+v", got, want)
	}
}

func TestLcsDiffEmptySide(t *testing.T) {
	a := []string{"x", "y"}
	var b []string

	got := lcsDiff(a, b, eq)
	want := []opStep{{Op: OpDelete, AIdx: 0}, {Op: OpDelete, AIdx: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lcsDiff(a, empty) = %+v, want %+v", got, want)
	}

	got = lcsDiff(b, a, eq)
	want = []opStep{{Op: OpInsert, BIdx: 0}, {Op: OpInsert, BIdx: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lcsDiff(empty, a) = %+v, want %+v", got, want)
	}
}

func TestLcsDiffInterleaved(t *testing.T) {
	// LCS is [1, 2]: a leading element only in a, a trailing element only in b.
	a := []string{"x", "1", "2"}
	b := []string{"1", "2", "y"}

	got := lcsDiff(a, b, eq)
	want := []opStep{
		{Op: OpDelete, AIdx: 0},
		{Op: OpEqual, AIdx: 1, BIdx: 0},
		{Op: OpEqual, AIdx: 2, BIdx: 1},
		{Op: OpInsert, BIdx: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lcsDiff(interleaved) = %+v, want %+v", got, want)
	}
}
