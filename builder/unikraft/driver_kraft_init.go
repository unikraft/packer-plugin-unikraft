package unikraft

import (
	"context"
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/manifest"
	"kraftkit.sh/oci"
	"kraftkit.sh/packmanager"
)

type Kraft struct{}

func (k *Kraft) Run(ctx context.Context, args []string) error {
	return fmt.Errorf("kraft command should not be called")
}

// KraftCommandContext returns a context with the Kraft commands registered.
// It needs to initialise the commands to ensure that internal context functions are called.
func KraftCommandContext(ui packersdk.Ui, logLevel string) context.Context {
	ctx := signals.SetupSignalContext()

	cfg, err := config.NewDefaultKraftKitConfig()
	if err != nil {
		panic(err)
	}
	cfg.NoPrompt = true

	cfgm, err := config.NewConfigManager(
		cfg,
		config.WithFile[config.KraftKit](config.DefaultConfigFile(), true),
	)
	if err != nil {
		panic(err)
	}

	ctx = config.WithConfigManager(ctx, cfgm)

	// Set up a default logger based on the internal TextFormatter
	logger := logrus.New()
	formatter := new(log.TextFormatter)
	formatter.FullTimestamp = true
	formatter.DisableTimestamp = true
	logger.Formatter = formatter

	switch logLevel {
	case "trace":
		logger.Level = logrus.TraceLevel
	case "debug":
		logger.Level = logrus.DebugLevel
	case "info":
		logger.Level = logrus.InfoLevel
	case "warn":
		logger.Level = logrus.WarnLevel
	case "error":
		logger.Level = logrus.ErrorLevel
	case "fatal":
		logger.Level = logrus.FatalLevel
	case "panic":
		logger.Level = logrus.PanicLevel
	default:
		logger.Level = logrus.InfoLevel
	}

	logger.SetOutput(&LoggerWriter{
		ui: &ui,
	})

	ctx = log.WithLogger(ctx, logger)

	managerConstructors := []func(u *packmanager.UmbrellaManager) error{
		oci.RegisterPackageManager(),
		manifest.RegisterPackageManager(),
	}

	err = packmanager.InitUmbrellaManager(ctx, managerConstructors)
	if err != nil {
		panic(err)
	}

	ctx, err = packmanager.WithDefaultUmbrellaManagerInContext(ctx)
	if err != nil {
		panic(err)
	}
	return ctx
}

// Logger writer that implements the writer interface
type LoggerWriter struct {
	ui *packersdk.Ui
}

func (l *LoggerWriter) Write(p []byte) (n int, err error) {
	(*l.ui).Message(string(p[:len(p)-1]))
	return len(p), nil
}
