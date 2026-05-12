package themes

import (
	"regexp"
	"strings"
	"testing"
)

// The themes package is essentially a static map of theme name → color
// palette. These tests guard against drift in the things that downstream
// CSS templating relies on:
//   - every advertised theme exists and exposes the same key set
//   - every color is a syntactically valid CSS color literal
//   - there are no surprise duplicates or empty values

var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{3}([0-9A-Fa-f]{3})?$`)

func TestAvailableThemes_NonEmpty(t *testing.T) {
	if len(AvailableThemes) == 0 {
		t.Fatalf("AvailableThemes is empty")
	}
	// The product brief promises these three themes today; if one is renamed
	// or removed, the CSS in web/static/css/styles.css needs to follow.
	for _, name := range []string{"dark-orange", "dark-blue", "light-blue"} {
		if _, ok := AvailableThemes[name]; !ok {
			t.Errorf("expected theme %q to be defined", name)
		}
	}
}

func TestAvailableThemes_ConsistentKeySet(t *testing.T) {
	// Every theme should expose the same colour keys. A missing key in one
	// theme would leave a `{{.key}}` literal in the rendered CSS for that
	// theme but not others — a silent bug.
	var reference map[string]bool
	var referenceName string
	for name, theme := range AvailableThemes {
		keys := map[string]bool{}
		for k := range theme.Colors {
			keys[k] = true
		}
		if reference == nil {
			reference = keys
			referenceName = name
			continue
		}
		for k := range reference {
			if !keys[k] {
				t.Errorf("theme %q is missing key %q present in %q", name, k, referenceName)
			}
		}
		for k := range keys {
			if !reference[k] {
				t.Errorf("theme %q has extra key %q not present in %q", name, k, referenceName)
			}
		}
	}
}

func TestAvailableThemes_ValuesAreCSSColors(t *testing.T) {
	// Most palette keys are colors expressed as hex literals. A few keys are
	// not colors (e.g. code_style names the highlight scheme). The allowlist
	// below captures those known exceptions so the rest must validate.
	nonColorKeys := map[string]bool{
		"code_style": true,
	}
	for name, theme := range AvailableThemes {
		for key, value := range theme.Colors {
			if nonColorKeys[key] {
				continue
			}
			if strings.TrimSpace(value) == "" {
				t.Errorf("theme %q key %q is empty", name, key)
				continue
			}
			if !colorPattern.MatchString(value) {
				t.Errorf("theme %q key %q value %q is not a valid #rgb/#rrggbb color", name, key, value)
			}
		}
	}
}

func TestAvailableThemes_NameMatchesKey(t *testing.T) {
	// The Theme struct has a Name field. We ship the same name as the map
	// key so consumers can look up a theme either way.
	for key, theme := range AvailableThemes {
		if theme.Name != key {
			t.Errorf("theme map key %q has Theme.Name = %q (should match)", key, theme.Name)
		}
	}
}
