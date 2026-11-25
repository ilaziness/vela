package layout

import "github.com/go-gui-org/go-gui/gui"

// Flush forces tiled chrome. Theme defaults otherwise inject
// PaddingMedium + radius + border on every colored container, which
// reads as floating cards with gaps around the panes.
func Flush(cfg gui.ContainerCfg) gui.ContainerCfg {
	if !cfg.Padding.IsSet() {
		cfg.Padding = gui.PaddingNone
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = gui.NoRadius
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = gui.NoBorder
	}
	if !cfg.Spacing.IsSet() {
		cfg.Spacing = gui.NoSpacing
	}
	return cfg
}

// VRule is a 1px vertical divider between flush panes.
func VRule() gui.View {
	th := gui.CurrentTheme()
	return gui.Column(Flush(gui.ContainerCfg{
		Sizing: gui.FixedFill,
		Width:  1,
		Color:  th.ColorBorder,
	}))
}
