package termcolor

import (
	"os"
	"testing"
)

func TestParseHex(t *testing.T) {
	tests := []struct {
		input   string
		want    Color
		wantErr bool
	}{
		{"#ff0000", Color{255, 0, 0}, false},
		{"ff0000", Color{255, 0, 0}, false},
		{"#00ff00", Color{0, 255, 0}, false},
		{"#0000ff", Color{0, 0, 255}, false},
		{"#ffffff", Color{255, 255, 255}, false},
		{"#000000", Color{0, 0, 0}, false},
		{"#abc", Color{170, 187, 204}, false}, // Shorthand
		{"invalid", Color{}, true},
		{"#gg0000", Color{}, true},
		{"#ff00", Color{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseHex(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHex(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseHex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestColorTo256(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		want  uint8
	}{
		// Pure colors should map to cube
		{"red", Color{255, 0, 0}, 196},  // cube index for max red
		{"green", Color{0, 255, 0}, 46}, // cube index for max green
		{"blue", Color{0, 0, 255}, 21},  // cube index for max blue
		{"white", Color{255, 255, 255}, 231},
		{"black", Color{0, 0, 0}, 16},
		// Grays should map to grayscale ramp
		{"gray128", Color{128, 128, 128}, 244}, // Should be in grayscale ramp
		{"gray50", Color{50, 50, 50}, 236},     // Dark gray
		// Saturated colors should stay in cube
		{"cyan", Color{0, 255, 255}, 51},
		{"magenta", Color{255, 0, 255}, 201},
		{"yellow", Color{255, 255, 0}, 226},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.color.To256()
			if got != tt.want {
				t.Errorf("Color%v.To256() = %d, want %d", tt.color, got, tt.want)
			}
		})
	}
}

func TestStylerFg(t *testing.T) {
	// Test with 256-color mode
	s := NewStyler(ColorMode256)
	red := Color{255, 0, 0}
	result := s.Fg(red, "test")
	// Should contain 256-color escape and reset
	if result != "\x1b[38;5;196mtest\x1b[39m" {
		t.Errorf("Fg 256 = %q, want escape code with 196", result)
	}

	// Test with truecolor mode
	s = NewStyler(ColorModeTruecolor)
	result = s.Fg(red, "test")
	if result != "\x1b[38;2;255;0;0mtest\x1b[39m" {
		t.Errorf("Fg truecolor = %q, want RGB escape", result)
	}
}

func TestStylerBg(t *testing.T) {
	s := NewStyler(ColorModeTruecolor)
	green := Color{0, 255, 0}
	result := s.Bg(green, "test")
	if result != "\x1b[48;2;0;255;0mtest\x1b[49m" {
		t.Errorf("Bg = %q, want background escape", result)
	}
}

func TestStylerFgHex(t *testing.T) {
	s := NewStyler(ColorModeTruecolor)
	result := s.FgHex("#00ff00", "test")
	if result != "\x1b[38;2;0;255;0mtest\x1b[39m" {
		t.Errorf("FgHex = %q", result)
	}

	// Invalid hex should return plain text
	result = s.FgHex("invalid", "test")
	if result != "test" {
		t.Errorf("FgHex invalid = %q, want plain text", result)
	}
}

func TestColorHex(t *testing.T) {
	c := Color{255, 128, 64}
	if c.Hex() != "#ff8040" {
		t.Errorf("Hex() = %q, want #ff8040", c.Hex())
	}
}

func TestStylerNoColor(t *testing.T) {
	s := NewStyler(ColorModeNone)
	red := Color{255, 0, 0}

	// All styling methods should return plain text
	if s.Fg(red, "test") != "test" {
		t.Errorf("Fg with ColorModeNone should return plain text")
	}
	if s.Bg(red, "test") != "test" {
		t.Errorf("Bg with ColorModeNone should return plain text")
	}
	if s.FgBg(red, red, "test") != "test" {
		t.Errorf("FgBg with ColorModeNone should return plain text")
	}
	if s.Bold("test") != "test" {
		t.Errorf("Bold with ColorModeNone should return plain text")
	}
	if s.Dim("test") != "test" {
		t.Errorf("Dim with ColorModeNone should return plain text")
	}
	if s.Italic("test") != "test" {
		t.Errorf("Italic with ColorModeNone should return plain text")
	}
	if s.Underline("test") != "test" {
		t.Errorf("Underline with ColorModeNone should return plain text")
	}
	if s.Strikethrough("test") != "test" {
		t.Errorf("Strikethrough with ColorModeNone should return plain text")
	}
	if s.FgStrikethrough(red, "test") != "test" {
		t.Errorf("FgStrikethrough with ColorModeNone should return plain text")
	}
	if s.FgUnderline(red, "test") != "test" {
		t.Errorf("FgUnderline with ColorModeNone should return plain text")
	}
	if s.Reset() != "" {
		t.Errorf("Reset with ColorModeNone should return empty string")
	}
	if s.FgHex("#ff0000", "test") != "test" {
		t.Errorf("FgHex with ColorModeNone should return plain text")
	}
	if s.BgHex("#ff0000", "test") != "test" {
		t.Errorf("BgHex with ColorModeNone should return plain text")
	}
}

func TestDetectColorModeNoColor(t *testing.T) {
	// Save original env
	origNoColor, hadNoColor := os.LookupEnv("NO_COLOR")
	origForceColor := os.Getenv("FORCE_COLOR")
	defer func() {
		if hadNoColor {
			os.Setenv("NO_COLOR", origNoColor)
		} else {
			os.Unsetenv("NO_COLOR")
		}
		if origForceColor != "" {
			os.Setenv("FORCE_COLOR", origForceColor)
		} else {
			os.Unsetenv("FORCE_COLOR")
		}
	}()

	// Test NO_COLOR with empty value
	os.Setenv("NO_COLOR", "")
	os.Unsetenv("FORCE_COLOR")
	if DetectColorMode() != ColorModeNone {
		t.Errorf("NO_COLOR='' should return ColorModeNone")
	}

	// Test NO_COLOR with any value
	os.Setenv("NO_COLOR", "1")
	if DetectColorMode() != ColorModeNone {
		t.Errorf("NO_COLOR=1 should return ColorModeNone")
	}

	// Test NO_COLOR takes precedence over FORCE_COLOR
	os.Setenv("FORCE_COLOR", "1")
	if DetectColorMode() != ColorModeNone {
		t.Errorf("NO_COLOR should take precedence over FORCE_COLOR")
	}

	// Test FORCE_COLOR without NO_COLOR
	os.Unsetenv("NO_COLOR")
	os.Setenv("FORCE_COLOR", "1")
	mode := DetectColorMode()
	if mode == ColorModeNone {
		t.Errorf("FORCE_COLOR=1 should not return ColorModeNone")
	}

	// Test FORCE_COLOR=0 is ignored
	os.Setenv("FORCE_COLOR", "0")
	// This should fall through to normal detection, not force colors
	// The result depends on environment, but shouldn't be forced
}
