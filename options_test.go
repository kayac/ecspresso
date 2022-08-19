package ecspresso_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kayac/ecspresso"
)

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
