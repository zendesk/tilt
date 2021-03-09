package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/tilt-dev/tilt/internal/analytics"
	engineanalytics "github.com/tilt-dev/tilt/internal/engine/analytics"
	"github.com/tilt-dev/tilt/internal/hud/prompt"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type runCmd struct {
	watch                bool
	fileName             string
	outputSnapshotOnExit string

	hud    bool
	legacy bool
	stream bool
	// whether hud/legacy/stream flags were explicitly set or just got the default value
	hudFlagExplicitlySet bool

	//whether watch was explicitly set in the cmdline
	watchFlagExplicitlySet bool
}

func (c *runCmd) name() model.TiltSubcommand { return "run" }

func (c *runCmd) register() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "run [<tilt flags>] [-- command to run]",
		DisableFlagsInUseLine: true,
	}

	addStartServerFlags(cmd)
	addDevServerFlags(cmd)
	cmd.Flags().StringVarP(&c.fileName, "file", "f", "adhoc.tilt", "Path to Tiltfile")
	addKubeContextFlag(cmd)

	return cmd
}

func (c *runCmd) run(ctx context.Context, args []string) error {
	a := analytics.Get(ctx)

	termMode := store.TerminalModePrompt

	cmdUpTags := engineanalytics.CmdTags(map[string]string{
		"update_mode": updateModeFlag, // before 7/8/20 this was just called "mode"
		"term_mode":   strconv.Itoa(int(termMode)),
	})
	a.Incr("cmd.run", cmdUpTags.AsMap())
	defer a.Flush(time.Second)

	deferred := logger.NewDeferredLogger(ctx)
	ctx = redirectLogs(ctx, deferred)

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	webHost := provideWebHost()
	webURL, _ := provideWebURL(webHost, provideWebPort())
	startLine := prompt.StartStatusLine(webURL, webHost)
	log.Print(startLine)
	log.Print(buildStamp())

	if ok, reason := analytics.IsAnalyticsDisabledFromEnv(); ok {
		log.Printf("Tilt analytics disabled: %s", reason)
	}

	err := updateAdHocTiltfile(c.fileName, args)
	if err != nil {
		return err
	}
	args = nil

	if isTiltRunning() {
		deferred.Infof("Tilt already running. Added new resource to existing Tilt.")
		return nil
	}

	cmdUpDeps, err := wireCmdUp(ctx, a, cmdUpTags, "run")
	if err != nil {
		deferred.SetOutput(deferred.Original())
		return err
	}

	upper := cmdUpDeps.Upper

	// Any logs that showed up during initialization, make sure they're
	// in the prompt.
	cmdUpDeps.Prompt.SetInitOutput(deferred.CopyBuffered(logger.InfoLvl))

	l := store.NewLogActionLogger(ctx, upper.Dispatch)
	deferred.SetOutput(l)
	ctx = redirectLogs(ctx, l)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	engineMode := store.EngineModeUp

	err = upper.Start(ctx, args, cmdUpDeps.TiltBuild, engineMode,
		c.fileName, termMode, a.UserOpt(), cmdUpDeps.Token, string(cmdUpDeps.CloudAddress))
	if err != context.Canceled {
		return err
	} else {
		return nil
	}
}

func isTiltRunning() bool {
	url := apiURL("view")

	client := http.Client{
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       2 * time.Second,
	}
	res, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK {
		return false
	}

	return true
}

func localResourceCode(resourceName string, command []string) string {
	// strings.Join on space can do bad things with shell, but not clear we have better options
	// use r''' to protect against `'`, `"`, and `\`.
	// XXX: handle `'''`
	return fmt.Sprintf(`
local_resource('%s',
  r'''%s''',
  # add files here to automatically update on file change
  # deps=['.'],
)
`, resourceName, strings.Join(command, " "))
}

func createAdHocTiltfile(filename string) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return errors.Wrapf(err, "error opening %s", filename)
	}
	defer func() {
		_ = f.Close()
	}()
	_, err = f.WriteString(`# uncomment this to load your team's Tiltfile
# load('Tiltfile')

`)
	if err != nil {
		return errors.Wrap(err, "error writing to file")
	}
	return nil
}

func updateAdHocTiltfile(filename string, args []string) error {
	// if it doesn't exist, create adhoc.tilt
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		err := createAdHocTiltfile(filename)
		if err != nil {
			return err
		}
	}

	// if there are any args, add a local resource to run that command
	if len(args) > 0 {
		f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return errors.Wrapf(err, "error opening %s", filename)
		}
		defer func() {
			_ = f.Close()
		}()

		_, err = f.WriteString(localResourceCode(args[0], args))
		if err != nil {
			return errors.Wrap(err, "error writing to file")
		}
	}

	return nil
}
