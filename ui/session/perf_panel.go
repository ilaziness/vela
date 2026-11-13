package session

import (
	"fmt"
	"strconv"
	"time"

	"github.com/go-gui-org/go-charts/axis"
	"github.com/go-gui-org/go-charts/chart"
	"github.com/go-gui-org/go-charts/series"
	"github.com/go-gui-org/go-charts/theme"
	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/i18n"
	"github.com/ilaziness/vela/ui/app"
	"github.com/ilaziness/vela/ui/layout"
)

const (
	perfPanelWidth  float32 = 360
	perfChartHeight float32 = 140
	// perfChartPadLeft reserves room for y tick labels inside the
	// panel card.
	perfChartPadLeft float32 = 46
)

// go-charts series colors (Tableau 10 accents; gui.Color works directly).
var (
	perfColorCPU       = gui.Hex(0x4E79A7)
	perfColorMem       = gui.Hex(0xB8860B)
	perfColorPrimary   = gui.Hex(0x59A14F)
	perfColorSecondary = gui.Hex(0xE15759)
)

// perfChartTheme is the compact chart theme. go-charts' default padding
// (60 left / 40 right / 60 bottom) is sized for full-window charts; in a
// panel card it leaves almost no plot area, so the curves and labels
// collapse onto each other.
func perfChartTheme() *theme.Theme {
	th := gui.CurrentTheme()
	t := theme.Default()
	t.Background = gui.ColorTransparent // let the panel color show through
	t.PaddingTop = 6
	t.PaddingRight = 6
	t.PaddingBottom = 18
	t.PaddingLeft = perfChartPadLeft
	t.TickStyle = th.TextStyleBody
	t.LabelStyle = th.TextStyleBody
	t.Legend.TextStyle = th.TextStyleBody
	t.Legend.SwatchSize = 8
	t.Legend.Padding = 2
	return t
}

// pctTick renders a percentage y tick without decimals.
func pctTick(v float64) string { return strconv.Itoa(int(v)) + "%" }

// bytesTick renders a throughput y tick compactly ("14K", "1.2M") so the
// label fits the panel's y-label gutter.
func bytesTick(v float64) string { return compactBytes(v) }

// floorAxis is a y axis pinned to a zero floor. go-charts hands the raw
// data range straight to the axis, which for a 59–60% memory curve
// produces three identical "59%" labels and an area fill that spills
// below the plot. Here the floor is always 0 and the top is either
// fixed (percentages) or rounded up from the data (throughput).
type floorAxis struct {
	*axis.Linear
	top float64 // > 0 pins the top; 0 derives it from the data
}

func newFloorAxis(top float64, format axis.TickFormat) *floorAxis {
	return &floorAxis{
		Linear: axis.NewLinear(axis.LinearCfg{TickTarget: 3, TickFormat: format}),
		top:    top,
	}
}

// SetRange keeps the zero floor and caps the top.
func (a *floorAxis) SetRange(_, max float64) {
	top := a.top
	if top <= 0 {
		top = niceCeil(max)
	}
	if top <= 0 {
		top = 1
	}
	a.Linear.SetRange(0, top)
}

// niceCeil rounds a byte rate up to 1/2/5×10ⁿ of its 1024-based unit, so
// a throughput axis tops out on a label that reads round ("100K" rather
// than "98K").
func niceCeil(v float64) float64 {
	if v <= 0 {
		return 0
	}
	unit := 1.0
	for v/unit >= 1024 {
		unit *= 1024
	}
	n := v / unit
	step := 1.0
	for step*10 <= n {
		step *= 10
	}
	switch {
	case n <= step:
		return step * unit
	case n <= step*2:
		return step * 2 * unit
	case n <= step*5:
		return step * 5 * unit
	}
	return step * 10 * unit
}

// PerfPanel is the performance column shown between the terminal and
// the tool rail, rendered top-to-bottom: header, load/latency, then
// CPU / memory / root-FS / per-NIC / per-disk chart cards.
func PerfPanel(ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	samples, lastErr := ctrl.PerfSnapshot(tab.SessID)

	header := gui.Row(layout.Flush(gui.ContainerCfg{
		ID:     "perf-header-" + tab.ID,
		Sizing: gui.FillFit,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			// The title fills the row and pushes the close chip right.
			gui.Text(gui.TextCfg{
				Text:      i18n.Tf("shell.perf.title", perfTitle(tab, samples)),
				TextStyle: th.TextStyleBody,
				Sizing:    gui.FillFit,
			}),
			closeBtn(ctrl, tab),
		},
	}))

	if len(samples) == 0 {
		body := gui.Label(i18n.T("shell.perf.sampling"), th.TextStyleSecondary)
		if lastErr != "" {
			body = gui.Label(lastErr, th.TextStyleWarning)
		}
		return perfColumn(tab, header, body)
	}

	last := samples[len(samples)-1]

	overview := gui.Row(layout.Flush(gui.ContainerCfg{
		ID:      "perf-overview-" + tab.ID,
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			compactStat(th, i18n.T("shell.perf.load"), strconv.FormatFloat(last.Load1, 'f', 2, 64)),
			compactStat(th, i18n.T("shell.perf.latency"), strconv.FormatInt(last.LatencyMs, 10)+" ms"),
		},
	}))

	rootFS := gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "perf-fs-" + tab.ID,
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			statRow(th,
				i18n.T("shell.perf.root_fs"),
				formatBytes(last.DiskUsed)+" / "+formatBytes(last.DiskTotal)),
			gui.ProgressBar(gui.ProgressBarCfg{
				ID:      "perf-fs-bar-" + tab.ID,
				Percent: pct(last.DiskUsed, last.DiskTotal),
				Sizing:  gui.FillFixed,
				Height:  6,
			}),
		},
	}))

	cards := []gui.View{
		rootFS,
		chartCard(tab.ID, "cpu", i18n.T("shell.perf.cpu"),
			strconv.Itoa(int(last.CPUPercent))+"%",
			lineChart(tab.ID, "cpu",
				[]series.XY{xy(i18n.T("shell.perf.series.cpu"), perfColorCPU,
					chartPoints(samples, cpuPct))},
				newFloorAxis(100, pctTick), i18n.T("shell.perf.y_pct"))),
		chartCard(tab.ID, "mem", i18n.T("shell.perf.mem"),
			formatBytes(last.MemUsed)+" / "+formatBytes(last.MemTotal),
			lineChart(tab.ID, "mem",
				[]series.XY{xy(i18n.T("shell.perf.series.mem"), perfColorMem,
					chartPoints(samples, memPct))},
				newFloorAxis(100, pctTick), i18n.T("shell.perf.y_pct"))),
	}

	for _, iface := range last.Net {
		cards = append(cards, chartCard(tab.ID, "net-"+iface.Name,
			i18n.Tf("shell.perf.net", iface.Name),
			i18n.Tf("shell.perf.rx", formatRate(iface.RxBps))+" · "+
				i18n.Tf("shell.perf.tx", formatRate(iface.TxBps)),
			lineChart(tab.ID, "net-"+iface.Name, []series.XY{
				xy(i18n.T("shell.perf.series.rx"), perfColorPrimary,
					chartPoints(samples, netRate(iface.Name, true))),
				xy(i18n.T("shell.perf.series.tx"), perfColorSecondary,
					chartPoints(samples, netRate(iface.Name, false))),
			}, newFloorAxis(0, bytesTick), i18n.T("shell.perf.y_rate"))))
	}
	for _, disk := range last.DiskIO {
		cards = append(cards, chartCard(tab.ID, "io-"+disk.Name,
			i18n.Tf("shell.perf.disk_io", disk.Name),
			i18n.Tf("shell.perf.read", formatRate(disk.ReadBps))+" · "+
				i18n.Tf("shell.perf.write", formatRate(disk.WriteBps)),
			lineChart(tab.ID, "io-"+disk.Name, []series.XY{
				xy(i18n.T("shell.perf.series.read"), perfColorPrimary,
					chartPoints(samples, diskRate(disk.Name, true))),
				xy(i18n.T("shell.perf.series.write"), perfColorSecondary,
					chartPoints(samples, diskRate(disk.Name, false))),
			}, newFloorAxis(0, bytesTick), i18n.T("shell.perf.y_rate"))))
	}

	return perfColumn(tab, header, append([]gui.View{overview}, cards...)...)
}

// perfColumn wraps the panel content in a scrollable fixed-width column.
func perfColumn(tab *app.Tab, header gui.View, rest ...gui.View) gui.View {
	th := gui.CurrentTheme()
	content := append([]gui.View{header}, rest...)
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "perf-panel-" + tab.ID,
		Sizing:  gui.FixedFill,
		Width:   perfPanelWidth,
		Color:   th.ColorPanel,
		Padding: gui.NewPadding(6, 8, 6, 6),
		// One scrollable column: cards flow top-to-bottom.
		Scrollable: true,
		ScrollMode: gui.ScrollVerticalOnly,
		Spacing:    gui.SpacingMedium,
		Content:    content,
	}))
}

func closeBtn(ctrl *app.Controller, tab *app.Tab) gui.View {
	th := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		ID:            "perf-close-" + tab.ID,
		Variant:       gui.ButtonGhost,
		FocusDisabled: true,
		Padding:       gui.PaddingXSmall,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      gui.IconClose,
				TextStyle: th.TextStyleIconSmall,
			}),
		},
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			st := gui.State[app.State](ctx.Window)
			if t := st.TabByID(tab.ID); t != nil {
				t.PerfOpen = false
			}
			ctrl.StopPerf(tab.SessID)
		},
	})
}

// perfTitle prefers the remote hostname once known.
func perfTitle(tab *app.Tab, samples []app.PerfSample) string {
	if n := len(samples); n > 0 && samples[n-1].Hostname != "" {
		return samples[n-1].Hostname
	}
	return tab.Title
}

// compactStat is one "label value" pair in the overview row.
func compactStat(th gui.Theme, label, value string) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FitFit,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label + " ", TextStyle: th.TextStyleSecondary}),
			gui.Text(gui.TextCfg{Text: value, TextStyle: th.TextStyleBody}),
		},
	}))
}

// statRow is a full-width "label ......... value" row; the value box
// fills the remaining width and right-aligns its text.
func statRow(th gui.Theme, label, value string) gui.View {
	return gui.Row(layout.Flush(gui.ContainerCfg{
		Sizing: gui.FillFit,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: th.TextStyleSecondary}),
			valueText(th, value),
		},
	}))
}

// valueText right-aligns a value inside a stretching text box.
func valueText(th gui.Theme, value string) gui.View {
	style := th.TextStyleBody
	style.Align = gui.TextAlignRight
	return gui.Text(gui.TextCfg{
		Text:      value,
		TextStyle: style,
		Sizing:    gui.FillFit,
	})
}

// chartCard is a titled chart section: header (title | value) + chart.
func chartCard(tabID, metric, title, value string, v gui.View) gui.View {
	th := gui.CurrentTheme()
	return gui.Column(layout.Flush(gui.ContainerCfg{
		ID:      "perf-card-" + tabID + "-" + metric,
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingTight,
		Content: []gui.View{
			statRow(th, title, value),
			v,
		},
	}))
}

// chartPoints maps the sample history to (unix-seconds, value) points.
func chartPoints(
	samples []app.PerfSample,
	pick func(app.PerfSample) float64,
) []series.Point {
	pts := make([]series.Point, 0, len(samples))
	for _, s := range samples {
		pts = append(pts, series.Point{X: float64(s.At.Unix()), Y: pick(s)})
	}
	return pts
}

// xy builds one named, colored data series.
func xy(name string, color gui.Color, pts []series.Point) series.XY {
	return series.NewXY(series.XYCfg{Name: name, Color: color, Points: pts})
}

// lineChart wires one go-charts line chart inside a compact theme.
// A single series renders as a filled area without a legend; two
// series get a compact bottom legend to tell the colors apart.
func lineChart(tabID, metric string, xs []series.XY, yAxis axis.Axis, yName string) gui.View {
	cfg := chart.LineCfg{
		BaseCfg: chart.BaseCfg{
			ID:            "perf-chart-" + tabID + "-" + metric,
			Sizing:        gui.FillFixed,
			Height:        perfChartHeight,
			Theme:         perfChartTheme(),
			TooltipXLabel: i18n.T("shell.perf.tip_x"),
			TooltipYLabel: yName,
		},
		Series:   xs,
		XAxis:    timeXAxis(),
		YAxis:    yAxis,
		ShowArea: len(xs) == 1,
	}
	legend := theme.LegendNone
	if len(xs) > 1 {
		legend = theme.LegendBottom
	}
	cfg.LegendPosition = &legend
	return chart.Line(cfg)
}

// timeXAxis formats unix seconds as wall-clock ticks and keeps the tick
// count low: the library's time axis always targets ~8 ticks, which
// overlap in a panel card.
func timeXAxis() axis.Axis {
	return axis.NewLinear(axis.LinearCfg{
		TickTarget: 3,
		TickFormat: func(sec float64) string {
			return time.Unix(int64(sec), 0).Format("15:04")
		},
	})
}

func cpuPct(s app.PerfSample) float64 { return s.CPUPercent }

func memPct(s app.PerfSample) float64 {
	if s.MemTotal <= 0 {
		return 0
	}
	return float64(s.MemUsed) / float64(s.MemTotal) * 100
}

func netRate(name string, rx bool) func(app.PerfSample) float64 {
	return func(s app.PerfSample) float64 {
		for _, f := range s.Net {
			if f.Name == name {
				if rx {
					return float64(f.RxBps)
				}
				return float64(f.TxBps)
			}
		}
		return 0
	}
}

func diskRate(name string, read bool) func(app.PerfSample) float64 {
	return func(s app.PerfSample) float64 {
		for _, d := range s.DiskIO {
			if d.Name == name {
				if read {
					return float64(d.ReadBps)
				}
				return float64(d.WriteBps)
			}
		}
		return 0
	}
}

func pct(used, total int64) float32 {
	if total <= 0 {
		return 0
	}
	return float32(float64(used) / float64(total))
}

// formatBytes renders a byte count the way the legacy panel did
// (B/KB/MB/GB/TB, two decimals above B).
func formatBytes(b int64) string {
	if b < 1024 {
		return strconv.FormatInt(b, 10) + " B"
	}
	div, exp := int64(1024), 0
	for n := b / 1024; n >= 1024 && exp < 4; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatRate(bps int64) string {
	return formatBytes(bps) + "/s"
}

// compactBytes renders a byte rate without a unit suffix beyond the
// magnitude ("0", "441", "14K", "1.2M"), so a y tick label fits the
// narrow gutter.
func compactBytes(v float64) string {
	if v < 1024 {
		// An idle axis tops out at 1 B/s, whose mid tick is 0.5 — keep
		// the fraction so the labels stay distinct.
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	units := "KMGTPE"
	div, exp := 1024.0, 0
	for n := v / 1024; n >= 1024 && exp < len(units)-1; n /= 1024 {
		div *= 1024
		exp++
	}
	n := v / div
	if n < 10 {
		return fmt.Sprintf("%.1f%c", n, units[exp])
	}
	return fmt.Sprintf("%.0f%c", n, units[exp])
}
