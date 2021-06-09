package ecspresso

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/private/protocol/json/jsonutil"
)

func marshalJSON(s interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	b, err := jsonutil.BuildJSON(s)
	if err != nil {
		return nil, err
	}
	json.Indent(&buf, b, "", "  ")
	buf.WriteString("\n")
	return &buf, nil
}

func MarshalJSON(s interface{}) ([]byte, error) {
	b, err := marshalJSON(s)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), err
}

func MarshalJSONString(s interface{}) string {
	b, _ := marshalJSON(s)
	return b.String()
}

func isLongArnFormat(a string) (bool, error) {
	an, err := arn.Parse(a)
	if err != nil {
		return false, err
	}
	rs := strings.Split(an.Resource, "/")
	switch rs[0] {
	case "container-instance", "service", "task":
		return len(rs) >= 3, nil
	default:
		return false, nil
	}
}
