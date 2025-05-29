package data

import (
	"iter"
	"slices"
)

func Coalesce[T comparable](args ...T) T {
	var def T
	for _, arg := range args {
		if arg != def {
			return arg
		}
	}
	return def
}

func Intersection[T comparable](arrays ...iter.Seq[T]) []T {
	count := map[T]int{}
	total := len(arrays)
	for _, array := range arrays {
		for arg := range array {
			count[arg]++
		}
	}
	intersect := []T{}
	for key, value := range count {
		if value == total {
			intersect = append(intersect, key)
		}
	}
	return intersect
}

func Reduce[T comparable](full, removals []T) []T {
	left := []T{}
	for _, item := range full {
		if !slices.Contains(removals, item) {
			left = append(left, item)
		}
	}
	return left
}
