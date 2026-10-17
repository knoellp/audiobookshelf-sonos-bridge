package main

import (
	"html/template"
	"os"
	"path/filepath"
	"testing"
)

// chdirRepoRoot switches to the repository root so that the relative
// template paths used by the server resolve.
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// TestLoadTemplates makes sure the shared template set parses, so a broken
// template or a missing template function fails here instead of at startup.
func TestLoadTemplates(t *testing.T) {
	chdirRepoRoot(t)
	if _, err := loadTemplates(); err != nil {
		t.Fatalf("loadTemplates: %v", err)
	}
}

// TestPageTemplatesParse parses every page template together with the layout
// and partials, mirroring how the handlers render full pages.
func TestPageTemplatesParse(t *testing.T) {
	chdirRepoRoot(t)
	pages, err := filepath.Glob("web/templates/*.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("no page templates found")
	}
	for _, page := range pages {
		if filepath.Base(page) == "layout.html" {
			continue
		}
		tmpl, err := template.New("").Funcs(getFuncMap()).ParseGlob("web/templates/layout.html")
		if err != nil {
			t.Fatalf("layout: %v", err)
		}
		if tmpl, err = tmpl.ParseGlob("web/templates/partials/*.html"); err != nil {
			t.Fatalf("partials: %v", err)
		}
		if _, err = tmpl.ParseFiles(page); err != nil {
			t.Errorf("%s: %v", page, err)
		}
	}
}
