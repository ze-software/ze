// Design: docs/architecture/cli/color-system.md — structured logging colors

package slogutil

import (
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

type logPalette struct {
	key, date, clock, identity string
	warning, failure, success  string
}

var ansiLogPalette = logPalette{
	key:      "\x1b[94m",
	date:     "\x1b[95m",
	clock:    "\x1b[96m",
	identity: "\x1b[36m",
	warning:  "\x1b[1;93m",
	failure:  "\x1b[1;91m",
	success:  "\x1b[92m",
}

func consoleLogPalette() logPalette {
	background := os.Getenv("COLORFGBG")
	if background == "" {
		return ansiLogPalette
	}
	return logPaletteFor(background, colorprofile.Env(os.Environ()))
}

func logPaletteFor(colorfgbg string, profile colorprofile.Profile) logPalette {
	// COLORFGBG conventionally ends with the terminal's background index.
	separator := strings.LastIndexByte(colorfgbg, ';')
	if separator < 0 || profile < colorprofile.ANSI256 {
		return ansiLogPalette
	}
	backgroundIndex, err := strconv.ParseUint(colorfgbg[separator+1:], 10, 8)
	if err != nil {
		return ansiLogPalette
	}
	background := logLuminance(ansi.IndexedColor(backgroundIndex))
	style := func(dark, light uint32, bold bool) string {
		value := dark
		if background > 0.179 {
			value = light
		}
		foreground := profile.Convert(ansi.RGBColor{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value)})
		luminance := logLuminance(foreground)
		result := ansi.NewStyle()
		if bold {
			result = result.Bold()
		}
		if (max(background, luminance)+0.05)/(min(background, luminance)+0.05) >= 4.5 {
			result = result.ForegroundColor(foreground)
		} else if !bold {
			return ""
		}
		return result.String()
	}

	return logPalette{
		key:      style(0x91b8fa, 0x2456a6, false),
		date:     style(0xc5b5f4, 0x59428c, false),
		clock:    style(0x62d9f4, 0x006174, false),
		identity: style(0x83d6c1, 0x006a55, false),
		warning:  style(0xffcb6b, 0x805000, true),
		failure:  style(0xff8d9a, 0xa42139, true),
		success:  style(0x9bdb8b, 0x326622, false),
	}
}

func logLuminance(c color.Color) float64 {
	linear := func(channel uint32) float64 {
		srgb := float64(channel) / 65535
		if srgb <= 0.04045 {
			return srgb / 12.92
		}
		return math.Pow((srgb+0.055)/1.055, 2.4)
	}
	r, g, b, _ := c.RGBA()
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}
