// code.go renders tool output the way a markdown fenced block does: the
// small mono face on the code fill, one colour per token class.
//
// go-gui's Markdown widget is not used for it: every document draws a
// floating copy button and pulls its first block's right edge in for it,
// and a fenced block adds a second button in the same corner.

package agent

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"

	"github.com/ilaziness/vela/ui/layout"
)

// codeHighlightCap bounds the text handed to the tokenizer: a markdown
// document caches its parsed blocks, this view is re-tokenized every frame.
const codeHighlightCap = 16 << 10

// shellOutput renders command output as a shell fenced code block.
func shellOutput(text string) gui.View {
	style := gui.DefaultMarkdownStyle()
	base := style.CodeBlockText
	// Plain tokens fall through to the operator colour, the base a fenced
	// block's runs get.
	base.Color = style.CodeOperatorColor
	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Color:   style.CodeBlockBG,
		Radius:  gui.RadiusSmall,
		Padding: gui.PadVH(3, 5),
		// Wrap still overflows on an unbreakable token; keep it in the card.
		Clip: true,
		Content: []gui.View{
			gui.RTF(gui.RTFCfg{
				RichText: gui.RichText{Runs: codeRuns("shell", text, base, style)},
				Mode:     gui.TextModeWrap,
			}),
		},
	}))
}

// codeRuns tokenizes text and colours each token from the markdown palette.
// Over the cap it stays one plain run.
func codeRuns(lang, text string, base gui.TextStyle, style gui.MarkdownStyle) []gui.RichTextRun {
	if len(text) > codeHighlightCap {
		return []gui.RichTextRun{gui.RichRun(text, base)}
	}
	toks := highlight.Default().Tokenize(lang, text)
	runs := make([]gui.RichTextRun, len(toks))
	for i, tk := range toks {
		s := base
		s.Color = codeKindColor(tk.Kind, style)
		runs[i] = gui.RichTextRun{Text: tk.Text, Style: s}
	}
	return runs
}

// codeKindColor maps a token class onto the markdown code palette — the
// mapping a fenced block's runs get (the library's own is unexported).
func codeKindColor(k highlight.Kind, style gui.MarkdownStyle) gui.Color {
	switch k {
	case highlight.KindKeyword:
		return style.CodeKeywordColor
	case highlight.KindString:
		return style.CodeStringColor
	case highlight.KindNumber:
		return style.CodeNumberColor
	case highlight.KindComment:
		return style.CodeCommentColor
	case highlight.KindType:
		return style.CodeTypeColor
	case highlight.KindFunction:
		return style.CodeFunctionColor
	case highlight.KindBuiltin:
		return style.CodeBuiltinColor
	default:
		return style.CodeOperatorColor
	}
}
