package ecspresso_test

import (
	"testing"

	"github.com/kayac/ecspresso"
)

var ecsArns = []struct {
	arnStr string
	isLong bool
}{
	{
		arnStr: "arn:aws:ecs:region:aws_account_id:container-instance/container-instance-id",
		isLong: false,
	},
	{
		arnStr: "arn:aws:ecs:region:aws_account_id:container-instance/cluster-name/container-instance-id",
		isLong: true,
	},
	{
		arnStr: "arn:aws:ecs:region:aws_account_id:service/service-name",
		isLong: false,
	},
	{
		arnStr: "arn:aws:ecs:region:aws_account_id:service/cluster-name/service-name",
		isLong: true,
	},
	{
		arnStr: "arn:aws:ecs:region:aws_account_id:task/task-id",
		isLong: false,
	},
	{
		arnStr: "arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id",
		isLong: true,
	},
}

func TestLongArnFormat(t *testing.T) {
	for _, ts := range ecsArns {
		b, err := ecspresso.IsLongArnFormat(ts.arnStr)
		if err != nil {
			t.Error(err)
		}
		if b != ts.isLong {
			t.Errorf("isLongArnFormat(%s) expected %v got %v", ts.arnStr, ts.isLong, b)
		}
	}
}
