package ecspresso_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	logsTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

func TestFormatLogEvent(t *testing.T) {
	event := logsTypes.OutputLogEvent{
		Timestamp: aws.Int64(1700000000000),
		Message:   aws.String("hello"),
	}
	for _, tc := range []struct {
		name       string
		taskID     string
		wantSuffix string
		wantTaskID string
	}{
		{name: "single task", taskID: "", wantSuffix: " hello"},
		{name: "multiple tasks", taskID: "abcdef", wantSuffix: " [abcdef] hello", wantTaskID: "abcdef"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, js := ecspresso.FormatLogEvent(event, tc.taskID)
			if !strings.HasSuffix(text, tc.wantSuffix) {
				t.Errorf("text %q does not end with %q", text, tc.wantSuffix)
			}
			if tc.taskID == "" && strings.Contains(text, "[") {
				t.Errorf("text %q must not contain a task ID", text)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(js), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"msg": "hello", "level": "INFO"}
			if tc.wantTaskID != "" {
				want["task_id"] = tc.wantTaskID
			}
			delete(got, "time")
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
