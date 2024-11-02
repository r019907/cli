package root

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cli/cli/v2/pkg/extensions"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
)

type ExternalCommandExitError struct {
	*exec.ExitError
}

type extensionReleaseInfo struct {
	CurrentVersion string
	LatestVersion  string
	Pinned         bool
	URL            string
}

func NewCmdExtension(io *iostreams.IOStreams, em extensions.ExtensionManager, ext extensions.Extension) *cobra.Command {
	// Setup channel containing information about potential latest release info
	updateMessageChan := make(chan *extensionReleaseInfo)

	return &cobra.Command{
		Use:   ext.Name(),
		Short: fmt.Sprintf("Extension %s", ext.Name()),
		// PreRun handles looking up whether extension has a latest version only when the command is ran.
		PreRun: func(c *cobra.Command, args []string) {
			go func() {
				if ext.UpdateAvailable() {
					updateMessageChan <- &extensionReleaseInfo{
						CurrentVersion: ext.CurrentVersion(),
						LatestVersion:  ext.LatestVersion(),
						Pinned:         ext.IsPinned(),
						URL:            ext.URL(),
					}
				}
			}()
		},
		RunE: func(c *cobra.Command, args []string) error {
			args = append([]string{ext.Name()}, args...)
			if _, err := em.Dispatch(args, io.In, io.Out, io.ErrOut); err != nil {
				var execError *exec.ExitError
				if errors.As(err, &execError) {
					return &ExternalCommandExitError{execError}
				}
				return fmt.Errorf("failed to run extension: %w\n", err)
			}
			return nil
		},
		// PostRun handles communicating extension release information if found
		PostRun: func(c *cobra.Command, args []string) {
			releaseInfo := <-updateMessageChan
			if releaseInfo != nil {
				stderr := io.ErrOut
				fmt.Fprintf(stderr, "\n\n%s %s → %s\n",
					ansi.Color(fmt.Sprintf("A new release of %s is available:", ext.Name()), "yellow"),
					ansi.Color(strings.TrimPrefix(releaseInfo.CurrentVersion, "v"), "cyan"),
					ansi.Color(strings.TrimPrefix(releaseInfo.LatestVersion, "v"), "cyan"))
				if releaseInfo.Pinned {
					fmt.Fprintf(stderr, "To upgrade, run: gh extension upgrade %s --force\n", ext.Name())
				} else {
					fmt.Fprintf(stderr, "To upgrade, run: gh extension upgrade %s\n", ext.Name())
				}
				fmt.Fprintf(stderr, "%s\n\n",
					ansi.Color(releaseInfo.URL, "yellow"))
			}
		},
		GroupID: "extension",
		Annotations: map[string]string{
			"skipAuthCheck": "true",
		},
		DisableFlagParsing: true,
	}
}
