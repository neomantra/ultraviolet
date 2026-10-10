package uv

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func TestRendererKittyPlaceholderColors(t *testing.T) {
	profiles := []colorprofile.Profile{
		colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.ANSI,
		colorprofile.Env([]string{"TERM=xterm-kitty", "COLORTERM=truecolor", "NO_COLOR=1"}),
		colorprofile.NoTTY,
	}
	for _, profile := range profiles {
		t.Run(profile.String(), func(t *testing.T) {
			var out bytes.Buffer
			r := NewTerminalRenderer(&out, []string{"TERM=xterm-kitty"})
			r.SetColorProfile(profile)
			r.Resize(6, 1)
			buf := NewRenderBuffer(6, 1)
			placeholder := fmt.Sprintf("%c%c%c", kitty.Placeholder, kitty.Diacritic(0), kitty.Diacritic(0))
			secondPlaceholder := fmt.Sprintf("%c%c%c", kitty.Placeholder, kitty.Diacritic(0), kitty.Diacritic(1))
			style := Style{
				Fg:             color.RGBA{R: 18, G: 52, B: 86, A: 255},
				Bg:             color.RGBA{R: 86, G: 52, B: 18, A: 255},
				UnderlineColor: color.RGBA{R: 17, G: 34, B: 51, A: 255},
				Attrs:          AttrBold,
			}
			// Ordinary text and placeholders deliberately share source colors.
			// Their emitted colors must still differ under reduced profiles.
			text := style.String() + "A" + placeholder + secondPlaceholder + "B" + ansi.ResetStyle
			line := NewStyledString(text).Lines(ansi.WcWidth)[0]
			imageFg := color.RGBA{R: 18, G: 52, B: 87, A: 255}
			line[2].Style.Fg = imageFg
			for x := range line {
				buf.SetCell(x, 0, &line[x])
			}
			check := func(wantPlaceholders int) {
				t.Helper()
				r.Render(buf)
				if err := r.Flush(); err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, emitted := range NewStyledString(out.String()).Lines(ansi.WcWidth) {
					for _, cell := range emitted {
						want := ConvertStyle(style, profile)
						if strings.HasPrefix(cell.Content, string(kitty.Placeholder)) {
							count++
							if profile != colorprofile.NoTTY {
								want.Fg = style.Fg
								if cell.Content == secondPlaceholder {
									want.Fg = imageFg
								}
								want.UnderlineColor = style.UnderlineColor
							}
						} else if cell.Content != "A" && cell.Content != "B" {
							continue
						}
						if !cell.Style.Equal(&want) {
							t.Errorf("cell %q style = %#v, want %#v; output %q", cell.Content, cell.Style, want, out.String())
						}
					}
				}
				if count != wantPlaceholders {
					t.Errorf("emitted %d placeholders, want %d; output %q", count, wantPlaceholders, out.String())
				}
				out.Reset()
			}
			check(2)
			// Change only the second image ID: the renderer must repaint it even
			// when both IDs map to the same reduced-profile color.
			imageFg = color.RGBA{R: 18, G: 52, B: 88, A: 255}
			changed := line[2]
			changed.Style.Fg = imageFg
			buf.SetCell(2, 0, &changed)
			check(1)
			r.Erase()
			check(2)
			buf.SetCell(2, 0, &line[3])
			check(0)
		})
	}
}

func TestRendererKittyPlaceholderPassThrough(t *testing.T) {
	placeholder := string(kitty.Placeholder)
	for _, tc := range []struct {
		name    string
		content string
		wantID  bool
	}{
		{"bare", placeholder, true},
		{"APC prefix", "\x1b_Ga=q,i=1;\x1b\\" + placeholder, true},
		{"DCS prefix", "\x1bPdata\x1b\\" + placeholder, true},
		{"ordinary text with APC prefix", "\x1b_Ga=q,i=1;\x1b\\X", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			r := NewTerminalRenderer(&out, nil)
			r.SetColorProfile(colorprofile.Ascii)
			cell := Cell{Content: tc.content, Width: 1, Style: Style{Fg: color.RGBA{R: 18, G: 52, B: 86, A: 255}}}
			r.updatePen(&cell)
			if err := r.Flush(); err != nil {
				t.Fatal(err)
			}
			want := ""
			if tc.wantID {
				want = "\x1b[38;2;18;52;86m"
			}
			if out.String() != want {
				t.Errorf("output = %q, want %q", out.String(), want)
			}
		})
	}
}
