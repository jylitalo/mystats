package data //nolint:testpackage

import (
	"iter"
	"slices"
	"testing"
)

func TestIntersectionStrings(t *testing.T) {
	values := []struct {
		name     string
		inputs   [][]string
		expected []string
	}{
		{
			name:     "alphabets",
			inputs:   [][]string{{"a", "b", "c", "d"}, {"b", "c"}},
			expected: []string{"b", "c"},
		},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			iters := []iter.Seq[string]{}
			for _, v := range value.inputs {
				iters = append(iters, slices.Values(v))
			}
			inter := Intersection(iters...)
			slices.Sort(inter)
			if slices.Compare(inter, value.expected) != 0 {
				t.Errorf("intersection doesn't match with expected (%#v vs. %#v)", inter, value.expected)
			}
		})
	}
}

func TestIntersectionInts(t *testing.T) {
	values := []struct {
		name     string
		inputs   [][]int
		expected []int
	}{
		{
			name:     "numbers",
			inputs:   [][]int{{1, 2, 3, 4}, {2, 3}},
			expected: []int{2, 3},
		},
	}
	for _, value := range values {
		t.Run(value.name, func(t *testing.T) {
			iters := []iter.Seq[int]{}
			for _, v := range value.inputs {
				iters = append(iters, slices.Values(v))
			}
			inter := Intersection(iters...)
			slices.Sort(inter)
			if slices.Compare(inter, value.expected) != 0 {
				t.Errorf("intersection doesn't match with expected (%#v vs. %#v)", inter, value.expected)
			}
		})
	}
}
