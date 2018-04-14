package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ghetzel/cli"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger(`main`)

type OnQuitFunc func() // {}
var OnQuit OnQuitFunc

func main() {
	app := cli.NewApp()
	app.Name = `canfriend`
	app.Usage = `Your friendly friend in CAN-BUS protocol analysis and reverse engineering.`
	app.Version = `0.0.1`
	app.EnableBashCompletion = false

	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:   `log-level, L`,
			Usage:  `Level of log output verbosity`,
			EnvVar: `LOGLEVEL`,
		},
	}

	app.Before = func(c *cli.Context) error {
		var addlInfo string
		levels := append([]string{
			`debug`,
			`pivot/querylog:warning`,
		}, c.StringSlice(`log-level`)...)

		for _, levelspec := range levels {
			var levelName string
			var moduleName string

			if parts := strings.SplitN(levelspec, `:`, 2); len(parts) == 1 {
				levelName = parts[0]
			} else {
				moduleName = parts[0]
				levelName = parts[1]
			}

			if level, err := logging.LogLevel(levelName); err == nil {
				if level == logging.DEBUG {
					addlInfo = `%{module}: `
				}

				logging.SetLevel(level, moduleName)
			} else {
				return err
			}
		}

		logging.SetFormatter(logging.MustStringFormatter(
			fmt.Sprintf("%%{color}%%{level:.4s}%%{color:reset}[%%{id:04d}] %s%%{message}", addlInfo),
		))

		log.Infof("Starting %s %s", c.App.Name, c.App.Version)
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:      `analyze`,
			Usage:     `Begin analysis of the CAN-BUS traffic from a given interface.`,
			ArgsUsage: `DEVICE`,
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  `frame-summary-limit, l`,
					Usage: `The maximum number of frame summary items to keep in memory.`,
					Value: DefaultFrameSummaryLimit,
				},
				cli.DurationFlag{
					Name:  `refresh-interval, i`,
					Usage: `How often the UI will refresh to display new data.`,
					Value: DefaultSummaryRefreshInterval,
				},
			},
			Action: func(c *cli.Context) {
				analyzer := NewAnalyzer(c.Args().First())
				analyzer.FrameSummaryLimit = c.Int(`frame-summary-limit`)

				ui := NewAnalyzerUI(analyzer)
				ui.SummaryRefreshInterval = c.Duration(`refresh-interval`)

				go func() {
					if err := ui.Run(); err != nil {
						log.Fatal(err)
					}

					log.Debugf("UI exited")
					analyzer.Stop()
					os.Exit(0)
				}()

				if err := analyzer.Run(); err != nil {
					log.Fatalf("Analyzer exited: %v", err)
				}

				log.Debugf("Quitting")
			},
		},
	}

	app.Run(os.Args)
}
