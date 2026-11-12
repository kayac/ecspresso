package ecspresso_test

import (
	"context"
	"testing"
	"time"

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

func TestMap2str(t *testing.T) {
	cases := []struct {
		in   map[string]string
		want string
	}{
		{map[string]string{"b": "2", "a": "1"}, "a=1,b=2"},
		{map[string]string{"foo": "bar", "baz": "qux", "quux": "corge"}, "baz=qux,foo=bar,quux=corge"},
		{map[string]string{}, ""},
	}

	for _, c := range cases {
		got := ecspresso.Map2str(c.in)
		if got != c.want {
			t.Errorf("map2str(%v) == %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCompareTags(t *testing.T) {
	testCases := []struct {
		name            string
		oldTags         []types.Tag
		newTags         []types.Tag
		expectedAdded   []types.Tag
		expectedUpdated []types.Tag
		expectedDeleted []types.Tag
	}{
		{
			name: "Test 1",
			oldTags: []types.Tag{
				{Key: new("key1"), Value: new("value1")},
				{Key: new("key2"), Value: new("value2")},
				{Key: new("key3"), Value: new("value3")},
			},
			newTags: []types.Tag{
				{Key: new("key1"), Value: new("value1_updated")},
				{Key: new("key2"), Value: new("value2")},
				{Key: new("key4"), Value: new("value4")},
			},
			expectedAdded: []types.Tag{
				{Key: new("key4"), Value: new("value4")},
			},
			expectedUpdated: []types.Tag{
				{Key: new("key1"), Value: new("value1_updated")},
			},
			expectedDeleted: []types.Tag{
				{Key: new("key3"), Value: new("value3")},
			},
		},
		// TODO Add more test cases
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			added, updated, deleted := ecspresso.CompareTags(tc.oldTags, tc.newTags)

			if diff := cmp.Diff(added, tc.expectedAdded, cmp.AllowUnexported(types.Tag{})); diff != "" {
				t.Errorf("Added mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(updated, tc.expectedUpdated, cmp.AllowUnexported(types.Tag{})); diff != "" {
				t.Errorf("Updated mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(deleted, tc.expectedDeleted, cmp.AllowUnexported(types.Tag{})); diff != "" {
				t.Errorf("Deleted mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSleepContext(t *testing.T) {
	t.Run("normal sleep", func(t *testing.T) {
		ctx := t.Context()
		start := time.Now()
		duration := 100 * time.Millisecond

		ecspresso.SleepContext(ctx, duration)

		elapsed := time.Since(start)
		if elapsed < duration {
			t.Errorf("Sleep duration was too short: %v, expected at least %v", elapsed, duration)
		}
	})

	t.Run("context canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		start := time.Now()
		duration := 1 * time.Second

		// Cancel the context after a short delay
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		// This should return as soon as the context is canceled
		ecspresso.SleepContext(ctx, duration)

		elapsed := time.Since(start)
		if elapsed >= duration {
			t.Errorf("Sleep did not respect context cancellation, duration: %v", elapsed)
		}
		if elapsed < 50*time.Millisecond {
			t.Errorf("Sleep returned too quickly, expected at least some delay: %v", elapsed)
		}
	})
}
