package serialui

import (
	"strconv"

	"github.com/go-gui-org/go-gui/gui"

	"github.com/ilaziness/vela/internal/serial"
	"github.com/ilaziness/vela/ui/layout"
	"github.com/ilaziness/vela/ui/protocol"
)

var baudRates = []string{
	"9600", "19200", "38400", "57600", "115200", "230400", "460800", "921600",
}

func bindForm(mgr *serial.Manager) protocol.FormFactory {
	return func(bus protocol.Bus, draft *protocol.Draft) gui.View {
		return Form(bus, draft, mgr)
	}
}

// Form renders basic serial fields (device + baud). Advanced line settings
// belong in bookmarks later.
func Form(bus protocol.Bus, draft *protocol.Draft, mgr *serial.Manager) gui.View {
	p := ensurePayload(draft)
	th := gui.CurrentTheme()
	busy, busyOp := bus.Busy()
	ports, _ := mgr.ListPorts()

	return gui.Column(layout.Flush(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		HAlign:  gui.HAlignStart,
		Content: []gui.View{
			gui.Combobox(gui.ComboboxCfg{
				ID:          "serial-device",
				Label:       "串口设备*",
				Value:       p.Device,
				Items:       ports,
				Placeholder: "选择串口",
				Sizing:      gui.FillFit,
				Disabled:    busy,
				OnSelect: func(v string, ctx gui.EventCtx) {
					p.Device = v
					ctx.Consume()
				},
			}),
			gui.Select(gui.SelectCfg{
				ID:       "serial-baud",
				Label:    "波特率*",
				Items:    baudRates,
				Selected: []string{strconv.Itoa(p.BaudRate)},
				Sizing:   gui.FillFit,
				Disabled: busy,
				OnSelect: func(vals []string, ctx gui.EventCtx) {
					if len(vals) == 0 {
						return
					}
					n, err := strconv.Atoi(vals[0])
					if err == nil && n > 0 {
						p.BaudRate = n
					}
					ctx.Consume()
				},
			}),
			gui.Row(layout.Flush(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingMedium,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					protocol.FormActionBtn(th, "serial-test", "测试连接", "测试中…", "test",
						gui.ButtonSecondary, busy, busyOp, func(ctx gui.EventCtx) {
							ctx.Consume()
							bus.Test(*draft)
						}),
					protocol.FormActionBtn(th, "serial-connect", "连接", "连接中…", "connect",
						gui.ButtonPrimary, busy, busyOp, func(ctx gui.EventCtx) {
							ctx.Consume()
							bus.Connect(*draft)
						}),
					gui.Button(gui.ButtonCfg{
						ID:       "serial-save",
						Variant:  gui.ButtonGhost,
						Label:    "保存",
						Disabled: busy,
						OnClick: func(ctx gui.EventCtx) {
							ctx.Consume()
							ctx.Window.Toast(gui.ToastCfg{Body: "保存书签（占位）", Severity: gui.ToastInfo})
						},
					}),
				},
			})),
		},
	}))
}

func ensurePayload(draft *protocol.Draft) *Payload {
	if draft == nil {
		p := DefaultPayload()
		return &p
	}
	switch v := draft.Payload.(type) {
	case *Payload:
		if v.BaudRate <= 0 {
			v.BaudRate = 115200
		}
		return v
	case Payload:
		p := v
		if p.BaudRate <= 0 {
			p.BaudRate = 115200
		}
		draft.Payload = &p
		return &p
	default:
		p := DefaultPayload()
		draft.Payload = &p
		return &p
	}
}
