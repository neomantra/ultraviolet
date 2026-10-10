package uv

import (
	"fmt"
	"image/color"
	"io"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi/kitty"
)

// Prebuild frames to isolate renderer work from chart generation and parsing.
// Alternate every cell so both frames require real output on every iteration.
func BenchmarkProtocolColorFrame(b *testing.B) {
	for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256, colorprofile.Ascii} {
		for _, kind := range []string{"plain", "styled", "kitty"} {
			b.Run(profile.String()+"/"+kind, func(b *testing.B) {
				frames := [2]*RenderBuffer{NewRenderBuffer(80, 24), NewRenderBuffer(80, 24)}
				for f, frame := range frames {
					for y := range 24 {
						for x := range 80 {
							cell := Cell{Content: string(rune('a' + f)), Width: 1}
							if kind != "plain" {
								cell.Style.Fg = color.RGBA{R: 18, G: 52, B: uint8(86 + f), A: 255}
							}
							if kind == "kitty" {
								cell.Content = fmt.Sprintf("%c%c%c", kitty.Placeholder, kitty.Diacritic(y), kitty.Diacritic(x))
							}
							frame.SetCell(x, y, &cell)
						}
					}
				}
				r := NewTerminalRenderer(io.Discard, nil)
				r.SetColorProfile(profile)
				r.SetFullscreen(true)
				r.Resize(80, 24)
				r.Render(frames[0])
				if err := r.Flush(); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					frame := frames[(i+1)%2]
					for y := range 24 {
						frame.TouchLine(0, y, 80)
					}
					r.Render(frame)
					if err := r.Flush(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
