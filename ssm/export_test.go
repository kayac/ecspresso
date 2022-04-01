package ssm

import (
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
)

func MockNew(ssm ssmiface.SSMAPI) *App {
	return &App{ssm: ssm}
}
