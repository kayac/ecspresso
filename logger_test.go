package ecspresso_test

import (
	"bytes"
	"testing"

	"github.com/kayac/ecspresso"
)

var logLevels = []string{"DEBUG", "INFO", "WARNING", "ERROR"}

func TestCommonLogger(t *testing.T) {
	for _, level := range logLevels {
		b := new(bytes.Buffer)
		logger := ecspresso.NewLogger()
		logger.SetOutput(ecspresso.NewLogFilter(b, level))
		ecspresso.SetLogger(logger)

		ecspresso.Log("test %s", level)
		ecspresso.Log("[DEBUG] test %s", level)
		ecspresso.Log("[INFO] test %s", level)
		ecspresso.Log("[WARNING] test %s", level)
		ecspresso.Log("[ERROR] test %s", level)
		t.Log(b.String())
	}
}

func TestLogger(t *testing.T) {
	app, err := ecspresso.New(&ecspresso.Config{
		Cluster: "testcluster",
		Service: "testservice",
	}, &ecspresso.Option{})
	if err != nil {
		t.Error(err)
	}
	for _, level := range logLevels {
		b := new(bytes.Buffer)
		logger := ecspresso.NewLogger()
		logger.SetOutput(ecspresso.NewLogFilter(b, level))
		app.SetLogger(logger)

		app.Log("test %s", "test")
		app.Log("[DEBUG] test %s", "test")
		app.Log("[INFO] test %s", "test")
		app.Log("[WARNING] test %s", "test")
		app.Log("[ERROR] test %s", "test")
		t.Log(b.String())
	}
}
