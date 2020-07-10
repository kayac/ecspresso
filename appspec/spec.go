package appspec

import (
	"github.com/aws/aws-sdk-go/service/ecs"
	"gopkg.in/yaml.v2"
)

type AppSpec struct {
	Version   *string `yaml:"version"`
	Resources []*Resource
}

func (a *AppSpec) String() string {
	b, _ := yaml.Marshal(a)
	return string(b)
}

type Resource struct {
	TargetService *TargetService
}

type TargetService struct {
	Type       *string
	Properties *Properties
}

type Properties struct {
	TaskDefinition       *string
	LoadBalancerInfo     *LoadBalancerInfo
	PlatformVersion      *string
	NetworkConfiguration *ecs.NetworkConfiguration
}

type LoadBalancerInfo struct {
	ContainerName *string
	ContainerPort *int64
}
