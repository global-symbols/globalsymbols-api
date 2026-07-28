package main

import "testing"

func TestExpandHas(t *testing.T) {
	tests := []struct {
		name   string
		expand string
		path   string
		want   bool
	}{
		{name: "empty", expand: "", path: expandPathPictoSymbolset, want: false},
		{name: "exact", expand: "picto.symbolset", path: expandPathPictoSymbolset, want: true},
		{name: "among many", expand: "foo picto.symbolset bar", path: expandPathPictoSymbolset, want: true},
		{name: "whitespace padded", expand: "  picto.symbolset  ", path: expandPathPictoSymbolset, want: true},
		{name: "unknown only", expand: "picto.licence other", path: expandPathPictoSymbolset, want: false},
		{name: "prefix not match", expand: "picto.symbolset.extra", path: expandPathPictoSymbolset, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandHas(tt.expand, tt.path); got != tt.want {
				t.Fatalf("expandHas(%q, %q) = %v, want %v", tt.expand, tt.path, got, tt.want)
			}
		})
	}
}
