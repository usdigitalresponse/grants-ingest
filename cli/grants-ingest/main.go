package main

import (
	"os"

	"github.com/alecthomas/kong"
	kitLog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/posener/complete"
	"github.com/usdigitalresponse/grants-ingest/cli/grants-ingest/ffisImport"
	"github.com/usdigitalresponse/grants-ingest/internal/log"
	"github.com/willabides/kongplete"
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

	FFISImport ffisImport.Cmd `cmd:"ffis-import" help:"Import FFIS spreadsheets to S3."`

	Completion kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
}

func main() {
	cli := CLI{
		Globals: Globals{},
	}

	var logger log.Logger
	parser := kong.Must(&cli,
		kong.Name("grants-ingest"),
		kong.Description("CLI utility for the grants-ingest service."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Bind(&logger),
	)

	kongplete.Complete(parser,
		kongplete.WithPredictor("file", complete.PredictFiles("*")),
		kongplete.WithPredictor("dir", complete.PredictDirs("*")),
	)

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	if err := ctx.Run(&cli.Globals); err != nil {
		ctx.Exit(1)
	}
}
