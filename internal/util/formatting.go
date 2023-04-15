package util

import (
	"bytes"
	"fmt"
	"sort"

	"golang.org/x/exp/constraints"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Title(s string) string {
	return cases.Title(language.English).String(s)
}

func FormatList[T constraints.Integer](list []T, sorted bool) string {
	var buf bytes.Buffer

	if sorted {
		sorted := append([]T{}, list...)
		sort.Sort(sortableSlice[T](sorted))
		list = sorted
	}

	for index, value := range list {
		if index != 0 {
			buf.WriteString(", ")
		}
		_, _ = fmt.Fprintf(&buf, "%v", value)
	}

	return buf.String()
}

type sortableSlice[T constraints.Integer] []T

func (s sortableSlice[T]) Len() int           { return len(s) }
func (s sortableSlice[T]) Less(i, j int) bool { return s[i] < s[j] }
func (s sortableSlice[T]) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
