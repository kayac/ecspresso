package ecspresso_test

import (
	"testing"
	"time"

	"github.com/kayac/ecspresso"
)

func TestDeploymentDefinition(t *testing.T) {
	path := "tests/deployment.json"
	c := &ecspresso.Config{
		Region:                   "ap-northeast-1",
		Timeout:                  300 * time.Second,
		Service:                  "test",
		Cluster:                  "default",
		TaskDefinitionPath:       "tests/td.json",
		DeploymentDefinitionPath: "tests/deployment.json",
	}
	app, err := ecspresso.NewApp(c)
	if err != nil {
		t.Error(err)
	}
	dc, err := app.LoadDeploymentDefinition(path)
	if err != nil || dc == nil {
		t.Errorf("%s load failed: %s", path, err)
	}

	if *dc.ApplicationName != "AppECS-default-test" ||
		*dc.DeploymentGroupName != "DgpECS-default-test" ||
		*dc.DeploymentConfigName != "CodeDeployDefault.ECSAllAtOnce" ||
		*dc.AutoRollbackConfiguration.Enabled != true ||
		*dc.AutoRollbackConfiguration.Events[0] != "DEPLOYMENT_FAILURE" {
		t.Errorf("unexpected deployment definition %s", dc.String())
	}
}
