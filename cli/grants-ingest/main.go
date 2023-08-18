package main

import (
	"github.com/alecthomas/kong"
	kitLog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/usdigitalresponse/grants-ingest/cli/grants-ingest/ffisImport"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
)

type Globals struct {
	Log struct {
		Level string `enum:"debug,info,warn,error" help:"Log level (debug|info|warn|error)" default:"info"`
		JSON  bool   `help:"Outputs JSON-formatted logs"`
	} `embed:"" prefix:"log-"`
}

func (g Globals) AfterApply(app *kong.Kong, logger *log.Logger) error {
	var newLogger log.Logger
	if g.Log.JSON {
		newLogger = kitLog.NewJSONLogger(app.Stderr)
	} else {
		newLogger = kitLog.NewLogfmtLogger(app.Stderr)
	}
	logLevel := level.ParseDefault(g.Log.Level, level.InfoValue())
	newLogger = level.NewFilter(newLogger, level.Allow(logLevel))
	*logger = newLogger
	return nil
}

type CLI struct {
	Globals

	FFISImport ffisImport.Cmd `cmd:"ffis-import" help:"Imports FFIS spreadsheets to S3."`
}

func main() {
	cli := CLI{
		Globals: Globals{},
	}

	var logger log.Logger
	ctx := kong.Parse(&cli,
		kong.Name("grants-ingest"),
		kong.Description("CLI utility for the grants-ingest service."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Bind(&logger),
	)
	if err := ctx.Run(&cli.Globals); err != nil {
		ctx.Exit(1)
	}
}
