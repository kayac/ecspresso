package appspec

import (
	"github.com/aws/aws-sdk-go/service/ecs"
	"gopkg.in/yaml.v2"
)

var Version = "0.0"

type AppSpec struct {
	Version   *string     `yaml:"version"`
	Resources []*Resource `yaml:"Resources"`
}

func New() *AppSpec {
	return &AppSpec{
		Version: &Version,
	}
}

func (a *AppSpec) String() string {
	b, _ := yaml.Marshal(a)
	return string(b)
}

type Resource struct {
	TargetService *TargetService `yaml:"TargetService"`
}

type TargetService struct {
	Type       *string     `yaml:"Type"`
	Properties *Properties `yaml:"Properties"`
}

type Properties struct {
	TaskDefinition       *string                   `yaml:"TaskDefinition"`
	LoadBalancerInfo     *LoadBalancerInfo         `yaml:"LoadBalancerInfo"`
	PlatformVersion      *string                   `yaml:"PlatformVersion"`
	NetworkConfiguration *ecs.NetworkConfiguration `yaml:"NetworkConfiguration"`
}

type LoadBalancerInfo struct {
	ContainerName *string `yaml:"ContainerName"`
	ContainerPort *int64  `yaml:"ContainerPort"`
}
