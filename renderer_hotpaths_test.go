package uv

import (
	"crypto/sha256"
	"fmt"
	"image/color"
	"io"
	"reflect"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func TestKittyDriftFastPath(t *testing.T) {
	check := func(content string) {
		t.Helper()
		for _, m := range []ansi.Method{ansi.WcWidth, ansi.GraphemeWidth} {
			for _, width := range []int{0, 1, 2} {
				cell := Cell{Content: content, Width: width}
				want := width != 0 && (width > 1 || m.StringWidth(content) != ansi.StringWidth(content))
				if got := lineHasDrift(m, Line{cell}); got != want {
					t.Fatalf("content=%q method=%v width=%d: drift=%v, want %v", content, m, width, got, want)
				}
			}
		}
	}
	for r := range kittyDiacritics {
		for n := 0; n <= 3; n++ {
			content := string(kitty.Placeholder)
			for range n {
				content += string(r)
			}
			check(content)
		}
	}
	for _, suffix := range []string{"x", "\ufe0f", "\u200d👩", "\xff", "\u0305\u0305\u0305\u0305"} {
		content := string(kitty.Placeholder) + suffix
		if standardKittyPlaceholder(content) {
			t.Fatalf("unexpected fast path for %q", content)
		}
		check(content)
	}
}

func TestStyleConversionCache(t *testing.T) {
	r := NewTerminalRenderer(io.Discard, nil)
	styles := []Style{
		{},
		{Fg: color.RGBA{R: 18, G: 52, B: 86, A: 255}},
		{Fg: ansi.IndexedColor(1), Bg: ansi.BasicColor(2), Attrs: AttrBold},
		{Fg: ansi.BasicColor(1)},
		{Fg: ansi.IndexedColor(1)},
		{Fg: color.RGBA{R: 128, A: 255}},
		{Fg: ansi.RGBColor{R: 18, G: 52, B: 86}, UnderlineColor: color.RGBA{G: 255, A: 255}},
	}
	for _, p := range []colorprofile.Profile{colorprofile.ANSI256, colorprofile.ANSI, colorprofile.Ascii, colorprofile.TrueColor, colorprofile.NoTTY, colorprofile.ANSI256} {
		r.SetColorProfile(p)
		for _, style := range styles {
			for _, content := range []string{"A", "B", string(kitty.Placeholder), "C"} {
				cell := Cell{Content: content, Width: 1, Style: style}
				want := ConvertStyle(style, p)
				if content == string(kitty.Placeholder) && p != colorprofile.NoTTY {
					want.Fg, want.UnderlineColor = style.Fg, style.UnderlineColor
				}
				got := r.cellStyle(&cell)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("profile=%v content=%q: got %#v, want %#v", p, content, got, want)
				}
			}
		}
	}
	// Unknown mutable implementations must never become cache keys.
	mutable := &color.RGBA{R: 255, A: 255}
	if cacheableColor(mutable) {
		t.Fatal("mutable color accepted as a cache key")
	}
}

func TestCursorPositionLen(t *testing.T) {
	for _, row := range []int{-1, 0, 1, 2, 9, 10, 99, 100, 999, 1000, 1000000} {
		for _, col := range []int{-1, 0, 1, 2, 9, 10, 99, 100, 999, 1000, 1000000} {
			if got, want := cursorPositionLen(col, row), len(ansi.CursorPosition(col, row)); got != want {
				t.Errorf("col=%d row=%d: length=%d, want %d", col, row, got, want)
			}
		}
	}
}

func TestCursorSelectionCompatibility(t *testing.T) {
	// Fingerprint the byte-for-byte choices of the pre-optimization renderer,
	// including tie-breaking, across capabilities, modes and screen content.
	hash := sha256.New()
	buf := NewRenderBuffer(20, 4)
	for y := range 4 {
		for x := range 20 {
			cell := Cell{Content: "x", Width: 1}
			if x%3 == 0 {
				cell.Style.Fg = color.RGBA{R: 255, A: 255}
			}
			buf.SetCell(x, y, &cell)
		}
	}
	for _, caps := range []capabilities{noCaps, capHPA | capVPA, capCHA | capBS, capHPA | capVPA | capHT | capCBT | capBS} {
		for _, flags := range []tFlag{0, tFullscreen, tRelativeCursor, tMapNewline, tFullscreen | tMapNewline} {
			r := NewTerminalRenderer(io.Discard, nil)
			r.caps, r.flags, r.tabs = caps, flags, DefaultTabStops(20)
			for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.Ascii} {
				r.SetColorProfile(profile)
				for _, fx := range []int{-1, 0, 1, 7, 8, 19} {
					for _, fy := range []int{-1, 0, 1, 3} {
						r.cur.X, r.cur.Y = fx, fy
						for tx := range 20 {
							for ty := range 4 {
								for _, overwrite := range []bool{false, true} {
									fmt.Fprintf(hash, "%q\n", moveCursor(r, buf, tx, ty, overwrite))
								}
							}
						}
					}
				}
			}
		}
	}
	const want = "ca7e58f997d48e809f92cd470c7ebf92def6f7d87be8d81d910d4c549c5012e0"
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != want {
		t.Fatalf("cursor sequence fingerprint = %s, want %s", got, want)
	}
}
