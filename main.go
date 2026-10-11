// Command bolt-navi is a Go port of the Python "hybrid" bolt-tui concept: a
// search-first palette that opens into a single-page run wizard for Puppet
// Bolt tasks & plans.
//
// Run it from a Bolt project root, or install it system-wide and point it at a
// project with --project:
//
//	bolt-navi                       # use the current directory as the project
//	bolt-navi --project C:\ops\bolt # run against a project from anywhere
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/phr34k/bolt-navi/internal/bolt"
	"github.com/phr34k/bolt-navi/internal/ui"
)

func main() {
	var project, targets string
	flag.StringVar(&project, "project", "",
		"Bolt project directory; forwarded to bolt as --project and used to "+
			"locate its caches/inventory (defaults to the current directory)")
	flag.StringVar(&targets, "targets", "",
		"Comma-separated targets or groups to pre-populate the targets field "+
			"when the run wizard opens (e.g. --targets web1,web2 or --targets all)")
	flag.Parse()

	if _, err := exec.LookPath("bolt"); err != nil {
		fmt.Fprintln(os.Stderr, "error: 'bolt' not found on PATH.")
		os.Exit(1)
	}

	// root is where bolt-navi reads the on-disk caches and inventory.yaml; when
	// --project is given it is also forwarded to every bolt call.
	root := project
	if root == "" {
		root = "."
	}
	bolt.SetProject(project)

	p := tea.NewProgram(ui.New(root, targets), tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "bolt-navi:", err)
		os.Exit(1)
	}
}
