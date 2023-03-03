package ecspresso_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso/v2"
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

type tagsTestSuite struct {
	src  string
	tags []types.Tag
	ok   bool
}

var tagsTestSuites = []tagsTestSuite{
	{
		src:  "",
		tags: []types.Tag{},
		ok:   true,
	},
	{
		src: "Foo=FOO",
		tags: []types.Tag{
			{Key: aws.String("Foo"), Value: aws.String("FOO")},
		},
		ok: true,
	},
	{
		src: "Foo=FOO,Bar=BAR",
		tags: []types.Tag{
			{Key: aws.String("Foo"), Value: aws.String("FOO")},
			{Key: aws.String("Bar"), Value: aws.String("BAR")},
		},
		ok: true,
	},
	{
		src: "Foo=,Bar=",
		tags: []types.Tag{
			{Key: aws.String("Foo"), Value: aws.String("")},
			{Key: aws.String("Bar"), Value: aws.String("")},
		},
		ok: true,
	},
	{
		src: "Foo=FOO,Bar=BAR,Baz=BAZ,",
		tags: []types.Tag{
			{Key: aws.String("Foo"), Value: aws.String("FOO")},
			{Key: aws.String("Bar"), Value: aws.String("BAR")},
			{Key: aws.String("Baz"), Value: aws.String("BAZ")},
		},
		ok: true,
	},
	{src: "Foo"},      // fail patterns
	{src: "Foo=,Bar"}, // fail patterns
	{src: "="},        // fail patterns
}

func TestParseTags(t *testing.T) {
	for _, ts := range tagsTestSuites {
		tags, err := ecspresso.ParseTags(ts.src)
		if ts.ok {
			if err != nil {
				t.Error(err)
				continue
			}
			opt := cmpopts.IgnoreUnexported(types.Tag{})
			if d := cmp.Diff(tags, ts.tags, opt); d != "" {
				t.Error(d)
			}
		} else {
			if err == nil {
				t.Errorf("must be failed %s", ts.src)
			}
		}
	}
}

func extractStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	org := os.Stdout
	defer func() {
		os.Stdout = org
	}()
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.Bytes()
}
