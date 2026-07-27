package cli

import (
	"testing"
	"time"
)

func TestDebouncerCoalesces(t *testing.T) {
	d := newDebouncer(100 * time.Millisecond)
	for i := 0; i < 5; i++ {
		d.Trigger()
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-d.C():
	case <-time.After(2 * time.Second):
		t.Fatal("debouncer did not fire after coalesced triggers")
	}
	select {
	case <-d.C():
		t.Fatal("debouncer fired more than once for a coalesced burst")
	case <-time.After(300 * time.Millisecond):
	}

	d.Trigger()
	select {
	case <-d.C():
	case <-time.After(2 * time.Second):
		t.Fatal("debouncer did not fire after a later trigger")
	}
}

func TestDevShouldRebuild(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		uiMode bool
		want   bool
	}{
		{name: "go file", path: "internal/app/app.go", uiMode: false, want: true},
		{name: "go file in ui mode", path: "main.go", uiMode: true, want: true},
		{name: "go.mod", path: "go.mod", uiMode: false, want: true},
		{name: "go.sum", path: "go.sum", uiMode: false, want: true},
		{name: "dotenv", path: ".env", uiMode: false, want: true},
		{name: "nested dotenv", path: "config/.env", uiMode: false, want: true},
		{name: "html in api mode", path: "web/templates/home.html", uiMode: false, want: false},
		{name: "html in ui mode", path: "web/templates/home.html", uiMode: true, want: true},
		{name: "css in api mode", path: "web/static/app.css", uiMode: false, want: false},
		{name: "css in ui mode", path: "web/static/app.css", uiMode: true, want: true},
		{name: "js in api mode", path: "web/static/app.js", uiMode: false, want: false},
		{name: "js in ui mode", path: "web/static/app.js", uiMode: true, want: true},
		{name: "markdown", path: "README.md", uiMode: true, want: false},
		{name: "yaml", path: ".gin-kit.yaml", uiMode: true, want: false},
		{name: "sql migration", path: "migrations/00001_init.sql", uiMode: false, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := devShouldRebuild(tt.path, tt.uiMode); got != tt.want {
				t.Fatalf("devShouldRebuild(%q, %v) = %v, want %v", tt.path, tt.uiMode, got, tt.want)
			}
		})
	}
}

func TestDevIgnoreDir(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: ".git", want: true},
		{name: ".idea", want: true},
		{name: "node_modules", want: true},
		{name: "vendor", want: true},
		{name: "bin", want: true},
		{name: "tmp", want: true},
		{name: "dist", want: true},
		{name: "internal", want: false},
		{name: "cmd", want: false},
		{name: "web", want: false},
		{name: "migrations", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := devIgnoreDir(tt.name); got != tt.want {
				t.Fatalf("devIgnoreDir(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
