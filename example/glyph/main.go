// Command glyph-text-example renders styled text with
// github.com/go-gui-org/go-glyph (latest) into an offscreen image and
// writes it to glyph-example.png.
//
// go-glyph ships GPU backends (Metal/OpenGL/Web) but its text pipeline is
// pure Go: any DrawBackend implementation works. backend.go provides a
// software one over image.NRGBA, so the example needs no window.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"

	"github.com/go-gui-org/go-glyph"
)

const (
	imgW = 900
	imgH = 640
)

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, imgW, imgH))
	paintBackground(img)

	backend := newSoftBackend(img, 1.0)
	ts, err := glyph.NewTextSystem(backend)
	if err != nil {
		log.Fatal(err)
	}
	defer ts.Free()

	if err := renderDemo(ts); err != nil {
		log.Fatal(err)
	}
	ts.Commit()

	f, err := os.Create("glyph-example.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	fmt.Println("wrote glyph-example.png")
}

func paintBackground(img *image.NRGBA) {
	top := color.NRGBA{R: 24, G: 28, B: 46, A: 255}
	bot := color.NRGBA{R: 44, G: 52, B: 84, A: 255}
	for y := 0; y < imgH; y++ {
		t := float64(y) / float64(imgH-1)
		for x := 0; x < imgW; x++ {
			i := (y*imgW + x) * 4
			img.Pix[i+0] = uint8(float64(top.R)*(1-t) + float64(bot.R)*t)
			img.Pix[i+1] = uint8(float64(top.G)*(1-t) + float64(bot.G)*t)
			img.Pix[i+2] = uint8(float64(top.B)*(1-t) + float64(bot.B)*t)
			img.Pix[i+3] = 255
		}
	}
}

func renderDemo(ts *glyph.TextSystem) error {
	white := glyph.Color{R: 240, G: 242, B: 248, A: 255}

	// Plain text, a few sizes.
	if err := ts.DrawText(40, 90, "go-glyph text rendering", glyph.TextConfig{
		Style: glyph.TextStyle{FontName: "Sans 34", Color: white},
	}); err != nil {
		return err
	}
	if err := ts.DrawText(40, 140, "Bold + italic via Typeface override", glyph.TextConfig{
		Style: glyph.TextStyle{
			FontName: "Sans 20", Typeface: glyph.TypefaceBoldItalic, Color: white,
		},
	}); err != nil {
		return err
	}

	// Styled decorations: highlight, underline, strikethrough, letter spacing.
	if err := ts.DrawText(40, 200, "highlight · underline · strikethrough", glyph.TextConfig{
		Style: glyph.TextStyle{
			FontName:      "Sans 18",
			Color:         glyph.Color{R: 255, G: 255, B: 255, A: 255},
			BgColor:       glyph.Color{R: 80, G: 140, B: 255, A: 120},
			Underline:     true,
			Strikethrough: true,
			LetterSpacing: 1.5,
		},
	}); err != nil {
		return err
	}

	// CJK + emoji fallback through go-glyph's system font discovery.
	if err := ts.DrawText(40, 250, "你好，世界！こんにちは世界 🌍🚀", glyph.TextConfig{
		Style: glyph.TextStyle{FontName: "Sans 20", Color: white},
	}); err != nil {
		return err
	}

	// Gradient text.
	if err := ts.DrawText(40, 310, "Gradient fill", glyph.TextConfig{
		Style: glyph.TextStyle{FontName: "Sans 28", Color: white},
		Gradient: &glyph.GradientConfig{
			Direction: glyph.GradientHorizontal,
			Stops: []glyph.GradientStop{
				{Color: glyph.Color{R: 255, G: 80, B: 120, A: 255}, Position: 0},
				{Color: glyph.Color{R: 255, G: 200, B: 80, A: 255}, Position: 0.5},
				{Color: glyph.Color{R: 80, G: 200, B: 255, A: 255}, Position: 1},
			},
		},
	}); err != nil {
		return err
	}

	// Rich text: multiple runs in one shaped layout.
	rt := glyph.RichText{Runs: []glyph.StyleRun{
		{Text: "Rich ", Style: glyph.TextStyle{
			FontName: "Sans 22", Typeface: glyph.TypefaceBold,
			Color: glyph.Color{R: 120, G: 220, B: 160, A: 255},
		}},
		{Text: "text ", Style: glyph.TextStyle{
			FontName: "Sans 22", Typeface: glyph.TypefaceItalic,
			Color: glyph.Color{R: 255, G: 180, B: 80, A: 255},
		}},
		{Text: "runs 中文", Style: glyph.TextStyle{
			FontName: "Sans 22",
			Color:    glyph.Color{R: 200, G: 160, B: 255, A: 255},
		}},
	}}
	richLayout, err := ts.LayoutRichText(rt, glyph.TextConfig{
		Style: glyph.TextStyle{FontName: "Sans 22", Color: white},
	})
	if err != nil {
		return err
	}
	ts.DrawLayout(richLayout, 40, 380)

	// Wrapped paragraph in a fixed-width block, then drawn rotated.
	para := "Word wrapping, hanging indent and rotated layouts: " +
		"the same Layout value can be re-drawn every frame from the cache."
	paraLayout, err := ts.LayoutText(para, glyph.TextConfig{
		Style: glyph.TextStyle{FontName: "Sans 16", Color: white},
		Block: glyph.BlockStyle{
			Wrap:        glyph.WrapWord,
			Width:       380,
			Align:       glyph.AlignLeft,
			Indent:      -18,
			LineSpacing: 4,
		},
	})
	if err != nil {
		return err
	}
	ts.DrawLayout(paraLayout, 40, 430)

	// Rotated text reuses the paragraph layout through an affine transform.
	ts.DrawLayoutRotated(paraLayout, 470, 520, -0.35)

	// Markup mode: Pango-style <b>/<i>/<span> tags.
	if err := ts.DrawText(470, 180, "<b>Markup</b> <i>mode</i> "+
		"<span foreground='\"#7fd4ff\"'>colored</span> text",
		glyph.TextConfig{
			Style:     glyph.TextStyle{FontName: "Sans 20", Color: white},
			UseMarkup: true,
		}); err != nil {
		return err
	}

	return nil
}
