// Design: plan/spec-install-0-umbrella.md — ze install remote: config gen + fork

package install

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
)

func runRemote(args []string) int {
	fs := flag.NewFlagSet("install remote", flag.ContinueOnError)

	iface := fs.String("interface", "", "Network interface for provisioning")
	network := fs.String("network", "", "Provisioning network CIDR (e.g. 10.0.0.0/24)")
	image := fs.String("image", "", "Path to gokrazy disk image")
	sshUser := fs.String("ssh-username", "", "Admin username for installed target")
	sshPass := fs.String("ssh-password", "", "Admin password for installed target (bcrypt-hashed before use)")
	address := fs.String("address", "", "Override server IP (default: first IPv4 on interface)")

	fs.Usage = func() { remoteUsage() }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if errs := validateFlags(*iface, *network, *image, *sshUser, *sshPass); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "error: %s\n", e)
		}
		return 1
	}

	serverIP, ipErr := resolveServerIP(*iface, *address)
	if ipErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", ipErr)
		return 1
	}

	hash, hashErr := hashPassword(*sshPass)
	if hashErr != nil {
		fmt.Fprintf(os.Stderr, "error: hashing password: %v\n", hashErr)
		return 1
	}

	cfg := generateConfig(configParams{
		iface:       *iface,
		network:     *network,
		image:       *image,
		serverIP:    serverIP,
		sshUsername: *sshUser,
		sshPassHash: hash,
	})

	return forkAndServe(cfg)
}

func remoteUsage() {
	p := helpfmt.Page{
		Command: "ze install remote",
		Summary: "Start DHCP+PXE, TFTP, and HTTP provisioning servers (requires root)",
		Usage:   []string{"ze install remote --interface <name> --network <cidr> --image <path> --ssh-username <user> --ssh-password <pass>"},
		Sections: []helpfmt.HelpSection{
			{Title: "Required flags", Entries: []helpfmt.HelpEntry{
				{Name: "--interface", Desc: "Network interface for provisioning"},
				{Name: "--network", Desc: "Provisioning network CIDR (e.g. 10.0.0.0/24)"},
				{Name: "--image", Desc: "Path to gokrazy disk image"},
				{Name: "--ssh-username", Desc: "Admin username for installed target"},
				{Name: "--ssh-password", Desc: "Admin password (bcrypt-hashed before embedding in config)"},
			}},
			{Title: "Optional flags", Entries: []helpfmt.HelpEntry{
				{Name: "--address", Desc: "Override server IP (default: first IPv4 on --interface)"},
			}},
		},
	}
	p.Write()
}
