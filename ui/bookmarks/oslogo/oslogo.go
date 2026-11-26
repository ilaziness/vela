// Package oslogo embeds a curated set of official OS/distro logo SVGs for
// bookmark icons. Logos are trademarks of their respective owners; they are
// used here only to identify the target system of a connection (nominative
// use), same as terminal tools like Tabby/FinalShell.
//
// Sources: walkxcode/dashboard-icons and selfhst/icons (MIT), Lucide (ISC).
package oslogo

import _ "embed"

// Entry is one selectable OS logo. Token is stored in bookmark.Icon.
type Entry struct {
	Token string
	Name  string
	SVG   string
}

// DefaultSVG is the neutral "no logo" icon shown for empty/unknown tokens.
//
//go:embed default.svg
var DefaultSVG string

//go:embed ubuntu.svg
var ubuntuSVG string

//go:embed debian.svg
var debianSVG string

//go:embed arch-linux.svg
var archLinuxSVG string

//go:embed fedora.svg
var fedoraSVG string

//go:embed centos.svg
var centosSVG string

//go:embed rocky-linux.svg
var rockyLinuxSVG string

//go:embed manjaro-linux.svg
var manjaroSVG string

//go:embed opensuse.svg
var opensuseSVG string

//go:embed alpine.svg
var alpineSVG string

//go:embed linux-mint.svg
var linuxMintSVG string

//go:embed oracle.svg
var oracleSVG string

//go:embed amazon.svg
var amazonSVG string

//go:embed proxmox.svg
var proxmoxSVG string

//go:embed nixos.svg
var nixosSVG string

//go:embed openwrt.svg
var openwrtSVG string

//go:embed android.svg
var androidSVG string

//go:embed apple.svg
var appleSVG string

//go:embed windows-10.svg
var windowsSVG string

//go:embed raspberrypi.svg
var raspberryPiSVG string

//go:embed tux.svg
var tuxSVG string

//go:embed unraid.svg
var unraidSVG string

//go:embed truenas.svg
var truenasSVG string

//go:embed openmediavault.svg
var omvSVG string

//go:embed vmware-esxi.svg
var esxiSVG string

//go:embed synology.svg
var synologySVG string

//go:embed harvester.svg
var harvesterSVG string

// Catalog lists selectable OS logos in display order.
var Catalog = []Entry{
	{Token: "os:ubuntu", Name: "Ubuntu", SVG: ubuntuSVG},
	{Token: "os:debian", Name: "Debian", SVG: debianSVG},
	{Token: "os:fedora", Name: "Fedora", SVG: fedoraSVG},
	{Token: "os:centos", Name: "CentOS", SVG: centosSVG},
	{Token: "os:rocky", Name: "Rocky Linux", SVG: rockyLinuxSVG},
	{Token: "os:arch", Name: "Arch Linux", SVG: archLinuxSVG},
	{Token: "os:manjaro", Name: "Manjaro", SVG: manjaroSVG},
	{Token: "os:opensuse", Name: "openSUSE", SVG: opensuseSVG},
	{Token: "os:alpine", Name: "Alpine Linux", SVG: alpineSVG},
	{Token: "os:linuxmint", Name: "Linux Mint", SVG: linuxMintSVG},
	{Token: "os:oracle", Name: "Oracle Linux", SVG: oracleSVG},
	{Token: "os:amazon", Name: "Amazon Linux", SVG: amazonSVG},
	{Token: "os:proxmox", Name: "Proxmox VE", SVG: proxmoxSVG},
	{Token: "os:nixos", Name: "NixOS", SVG: nixosSVG},
	{Token: "os:openwrt", Name: "OpenWrt", SVG: openwrtSVG},
	{Token: "os:windows", Name: "Windows", SVG: windowsSVG},
	{Token: "os:macos", Name: "macOS", SVG: appleSVG},
	{Token: "os:android", Name: "Android", SVG: androidSVG},
	{Token: "os:raspberrypi", Name: "Raspberry Pi OS", SVG: raspberryPiSVG},
	{Token: "os:linux", Name: "Linux", SVG: tuxSVG},
	{Token: "os:unraid", Name: "Unraid", SVG: unraidSVG},
	{Token: "os:truenas", Name: "TrueNAS", SVG: truenasSVG},
	{Token: "os:omv", Name: "OpenMediaVault", SVG: omvSVG},
	{Token: "os:esxi", Name: "VMware ESXi", SVG: esxiSVG},
	{Token: "os:synology", Name: "Synology DSM", SVG: synologySVG},
	{Token: "os:harvester", Name: "Harvester", SVG: harvesterSVG},
}

// Lookup returns the catalog entry for token.
func Lookup(token string) (Entry, bool) {
	for _, e := range Catalog {
		if e.Token == token {
			return e, true
		}
	}
	return Entry{}, false
}
