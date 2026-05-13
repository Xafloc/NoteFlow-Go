package models

import "testing"

// Tests for the v1.4 per-section font-scale storage. Pins:
//   - GetFontScale returns the default when nothing has been set
//   - Set clamps to [Min, Max]
//   - Stored-on-disk out-of-range values are clamped on read (defensive)
//   - DefaultConfig populates all canonical sections

func TestConfig_GetFontScale_DefaultsWhenUnset(t *testing.T) {
	c := &Config{} // no FontScales map at all
	for _, section := range FontScaleSections {
		got := c.GetFontScale(section)
		if got != FontScaleDefault {
			t.Errorf("GetFontScale(%q) on empty config = %v, want default %v", section, got, FontScaleDefault)
		}
	}
}

func TestConfig_SetFontScale_ClampsToRange(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{0.5, FontScaleMin},   // below min
		{0.8, FontScaleMin},   // exactly min
		{1.0, 1.0},
		{1.6, FontScaleMax},   // exactly max
		{2.5, FontScaleMax},   // above max
		{-1.0, FontScaleMin},  // negative
	}
	for _, tt := range tests {
		c := DefaultConfig()
		c.SetFontScale("notes", tt.in)
		got := c.GetFontScale("notes")
		if got != tt.want {
			t.Errorf("SetFontScale(%v) → GetFontScale = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestConfig_GetFontScale_ClampsStoredValues(t *testing.T) {
	// Defensive: if someone hand-edited the config file to put a value
	// outside the allowed range, the API should still return something
	// safe rather than passing the garbage through.
	c := &Config{FontScales: map[string]float64{"notes": 5.0, "links": 0.1}}
	if got := c.GetFontScale("notes"); got != FontScaleMax {
		t.Errorf("oversized stored value not clamped on read: got %v, want %v", got, FontScaleMax)
	}
	if got := c.GetFontScale("links"); got != FontScaleMin {
		t.Errorf("undersized stored value not clamped on read: got %v, want %v", got, FontScaleMin)
	}
}

func TestDefaultConfig_PopulatesAllSections(t *testing.T) {
	c := DefaultConfig()
	if c.FontScales == nil {
		t.Fatalf("DefaultConfig.FontScales is nil")
	}
	for _, section := range FontScaleSections {
		if _, ok := c.FontScales[section]; !ok {
			t.Errorf("DefaultConfig missing section %q", section)
		}
		if c.FontScales[section] != FontScaleDefault {
			t.Errorf("DefaultConfig.FontScales[%q] = %v, want %v",
				section, c.FontScales[section], FontScaleDefault)
		}
	}
}
