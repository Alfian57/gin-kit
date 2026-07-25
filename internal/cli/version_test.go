package cli

import "testing"

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name     string
		injected string
		module   string
		want     string
	}{
		{name: "release injection", injected: "0.2.0", module: "v0.1.0", want: "0.2.0"},
		{name: "module fallback", injected: "dev", module: "v0.1.1", want: "0.1.1"},
		{name: "development build", injected: "dev", module: "(devel)", want: "dev"},
		{name: "empty metadata", injected: "", module: "", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.injected, tt.module); got != tt.want {
				t.Fatalf("resolveVersion(%q, %q) = %q, want %q", tt.injected, tt.module, got, tt.want)
			}
		})
	}
}
