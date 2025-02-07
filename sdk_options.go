package ecspresso

import awsConfig "github.com/aws/aws-sdk-go-v2/config"

type SDKOptions struct {
	RetryMaxAttempts *int `yaml:"retry_max_attempts,omitempty" json:"retry_max_attempts,omitempty"`
}

func (s SDKOptions) ConfigOptions() []awsConfig.LoadOptionsFunc {
	res := make([]awsConfig.LoadOptionsFunc, 0)

	if s.RetryMaxAttempts != nil {
		res = append(res, awsConfig.WithRetryMaxAttempts(*s.RetryMaxAttempts))
	}

	return res
}

// TODO: Unmarshal JSON
