package tray

import (
	"context"
	"time"
	"warp-server/pkg/log"

	"github.com/dominicletz/genserver"
	"github.com/getlantern/systray"
)

const (
	VPNConnectedMenuItemName     = "VPN connected"
	PFDisabledMenuItemName       = "PF Disabled"
	SSHKeysInstalledMenuItemName = "SSH Keys Installed"
	TunnelEnabledMenuItemName    = "Tunnel Enabled"
)

type Menu struct {
	QuitOrig *systray.MenuItem
	// Condition statuses
	VPN        *systray.MenuItem
	VPNState   *systray.MenuItem
	VPNReason  *systray.MenuItem
	VPNMessage *systray.MenuItem
	PF         *systray.MenuItem
	SSHKeys    *systray.MenuItem
	Tunnel     *systray.MenuItem
}

// System tray agent definition.
type SystemTrayAgent struct {
	gen *genserver.GenServer

	// Actions
	QuitOrig *systray.MenuItem
	Restart  *systray.MenuItem

	// Condition statuses
	VPNConnected     *systray.MenuItem
	PFDisabled       *systray.MenuItem
	SSHKeysInstalled *systray.MenuItem
	TunnelEnabled    *systray.MenuItem
}

// New runs the new system tray agent instance.
func New() *SystemTrayAgent {
	agent := &SystemTrayAgent{
		gen: genserver.New("Systray"),
	}

	agent.QuitOrig = systray.AddMenuItem("Stop", "Stop the WaRp/Server")
	agent.Restart = systray.AddMenuItemCheckbox("Restart", "Restart the WaRp/Server", false)
	systray.AddSeparator()
	agent.VPNConnected = systray.AddMenuItemCheckbox("VPN connected", "VPN connected condition", false)
	agent.PFDisabled = systray.AddMenuItemCheckbox("PF Disabled", "PF disabled condition", false)
	agent.SSHKeysInstalled = systray.AddMenuItemCheckbox("SSH Keys Installed", "SSH keys installed condition", false)
	agent.TunnelEnabled = systray.AddMenuItemCheckbox("Tunnel Enabled", "Tunnel enabled condition", false)

	return agent
}

// Cast is a non-blocking send
func (x *SystemTrayAgent) HandleQuit(ctx context.Context) {
	if x.gen.Cast(func() {
		go func() {
			select {
			case <-x.QuitOrig.ClickedCh:
				log.Info().Msg("Main", "Requesting quit...")
				systray.Quit()
				log.Info().Msg("Main", "Finished quitting")

			case <-ctx.Done():
				log.Info().Msg("Main", "Requesting quit...")
				systray.Quit()
				log.Info().Msg("Main", "Finished quitting")
			}

		}()
	}) == nil {
	} else {
		log.Error().Msg("Main", "Cannot run Exit handler")
	}
}

// Await uses the genserver.Terminate callback to block
// until the goroutine has finished
func (x *SystemTrayAgent) Start(onReady func(), onExit func()) {
	systray.Run(onReady, onExit)

	x.gen.Call(func() {
		x.gen.Terminate = func() {
			log.Info().Msg("Main", "Exiting goroutine...")
		}
	})

	// Shutodwn(linger) will keep the counter running
	// for linger (5 seconds here) during which
	// all calls and casts are still being worked on
	log.Info().Msg("Main", "Lingering for 5 seconds")
	x.gen.Shutdown(5 * time.Second)

	log.Info().Msg("Main", "Stopped goroutine")
}
