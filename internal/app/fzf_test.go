package app

import (
	"reflect"
	"testing"
)

func TestWithBackOption(t *testing.T) {
	tests := []struct {
		name  string
		in    []string
		want  []string
	}{
		{
			name: "empty list unchanged",
			in:   []string{},
			want: []string{},
		},
		{
			name: "back option prepended",
			in:   []string{"one", "two"},
			want: []string{fzfBackOption, "one", "two"},
		},
	}

	for _, tc := range tests {
		got := withBackOption(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
