package ecspresso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/kylelemons/godebug/diff"
)

type DiffOption struct {
	Unified bool `help:"unified diff format" default:"true" negatable:""`
}

func (d *App) Diff(ctx context.Context, opt DiffOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	var remoteTaskDefArn string
	// diff for services only when service defined
	if d.config.Service != "" {
		d.Log("[DEBUG] diff service compare with %s", d.config.Service)
		newSv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return fmt.Errorf("failed to load service definition: %w", err)
		}
		remoteSv, err := d.DescribeService(ctx)
		if err != nil {
			if errors.As(err, &errNotFound) {
				d.Log("[INFO] service not found, will create a new service")
			} else {
				return fmt.Errorf("failed to describe service: %w", err)
			}
		}
		if ds, err := diffServices(newSv, remoteSv, d.config.ServiceDefinitionPath, opt.Unified); err != nil {
			return err
		} else if ds != "" {
			fmt.Print(coloredDiff(ds))
		}
		if remoteSv != nil {
			remoteTaskDefArn = *remoteSv.TaskDefinition
		}
	}

	// task definition
	newTd, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	if remoteTaskDefArn == "" {
		arn, err := d.findLatestTaskDefinitionArn(ctx, *newTd.Family)
		if err != nil {
			if errors.As(err, &errNotFound) {
				d.Log("[INFO] task definition not found, will register a new task definition")
			} else {
				return err
			}
		}
		remoteTaskDefArn = arn
	}
	var remoteTd *ecs.RegisterTaskDefinitionInput
	if remoteTaskDefArn != "" {
		d.Log("[DEBUG] diff task definition compare with %s", remoteTaskDefArn)
		remoteTd, err = d.DescribeTaskDefinition(ctx, remoteTaskDefArn)
		if err != nil {
			return err
		}
	}

	if ds, err := diffTaskDefs(newTd, remoteTd, d.config.TaskDefinitionPath, remoteTaskDefArn, opt.Unified); err != nil {
		return err
	} else if ds != "" {
		fmt.Print(coloredDiff(ds))
	}

	return nil
}

type ServiceForDiff struct {
	*ecs.UpdateServiceInput
	Tags []types.Tag
}

func diffServices(local, remote *Service, localPath string, unified bool) (string, error) {
	var remoteArn string
	if remote != nil {
		remoteArn = aws.ToString(remote.ServiceArn)
	}

	localSvForDiff := ServiceDefinitionForDiff(local)
	remoteSvForDiff := ServiceDefinitionForDiff(remote)

	newSvBytes, err := MarshalJSONForAPI(localSvForDiff)
	if err != nil {
		return "", fmt.Errorf("failed to marshal new service definition: %w", err)
	}
	if local.DesiredCount == nil && remoteSvForDiff != nil {
		// ignore DesiredCount when it in local is not defined.
		remoteSvForDiff.UpdateServiceInput.DesiredCount = nil
	}
	remoteSvBytes, err := MarshalJSONForAPI(remoteSvForDiff)
	if err != nil {
		return "", fmt.Errorf("failed to marshal remote service definition: %w", err)
	}

	remoteSv := toDiffString(remoteSvBytes)
	newSv := toDiffString(newSvBytes)

	if unified {
		edits := myers.ComputeEdits(span.URIFromPath(remoteArn), remoteSv, newSv)
		return fmt.Sprint(gotextdiff.ToUnified(remoteArn, localPath, remoteSv, edits)), nil
	}

	ds := diff.Diff(remoteSv, newSv)
	if ds == "" {
		return ds, nil
	}
	return fmt.Sprintf("--- %s\n+++ %s\n%s", remoteArn, localPath, ds), nil
}

func diffTaskDefs(local, remote *TaskDefinitionInput, localPath, remoteArn string, unified bool) (string, error) {
	sortTaskDefinition(local)
	sortTaskDefinition(remote)

	newTdBytes, err := MarshalJSONForAPI(local)
	if err != nil {
		return "", fmt.Errorf("failed to marshal new task definition: %w", err)
	}

	remoteTdBytes, err := MarshalJSONForAPI(remote)
	if err != nil {
		return "", fmt.Errorf("failed to marshal remote task definition: %w", err)
	}

	remoteTd := toDiffString(remoteTdBytes)
	newTd := toDiffString(newTdBytes)

	if unified {
		edits := myers.ComputeEdits(span.URIFromPath(remoteArn), remoteTd, newTd)
		return fmt.Sprint(gotextdiff.ToUnified(remoteArn, localPath, remoteTd, edits)), nil
	}

	ds := diff.Diff(remoteTd, newTd)
	if ds == "" {
		return ds, nil
	}
	return fmt.Sprintf("--- %s\n+++ %s\n%s", remoteArn, localPath, ds), nil
}

func coloredDiff(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "-") {
			b.WriteString(color.RedString(line) + "\n")
		} else if strings.HasPrefix(line, "+") {
			b.WriteString(color.GreenString(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func tdToTaskDefinitionInput(td *TaskDefinition, tdTags []types.Tag) *TaskDefinitionInput {
	tdi := &TaskDefinitionInput{
		ContainerDefinitions:    td.ContainerDefinitions,
		Cpu:                     td.Cpu,
		EphemeralStorage:        td.EphemeralStorage,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		Family:                  td.Family,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		PlacementConstraints:    td.PlacementConstraints,
		RequiresCompatibilities: td.RequiresCompatibilities,
		RuntimePlatform:         td.RuntimePlatform,
		TaskRoleArn:             td.TaskRoleArn,
		ProxyConfiguration:      td.ProxyConfiguration,
		Volumes:                 td.Volumes,
	}
	if len(tdTags) > 0 {
		tdi.Tags = tdTags
	}
	return tdi
}

func jsonStr(v interface{}) string {
	s, _ := json.Marshal(v)
	return string(s)
}

func ServiceDefinitionForDiff(sv *Service) *ServiceForDiff {
	if sv == nil {
		return nil
	}
	sort.SliceStable(sv.PlacementConstraints, func(i, j int) bool {
		return jsonStr(sv.PlacementConstraints[i]) < jsonStr(sv.PlacementConstraints[j])
	})
	sort.SliceStable(sv.PlacementStrategy, func(i, j int) bool {
		return jsonStr(sv.PlacementStrategy[i]) < jsonStr(sv.PlacementStrategy[j])
	})
	sort.SliceStable(sv.Tags, func(i, j int) bool {
		return aws.ToString(sv.Tags[i].Key) < aws.ToString(sv.Tags[j].Key)
	})
	if sv.LaunchType == types.LaunchTypeFargate && sv.PlatformVersion == nil {
		sv.PlatformVersion = aws.String("LATEST")
	}
	if sv.SchedulingStrategy == "" || sv.SchedulingStrategy == types.SchedulingStrategyReplica {
		sv.SchedulingStrategy = types.SchedulingStrategyReplica
		if sv.DeploymentConfiguration == nil {
			sv.DeploymentConfiguration = &types.DeploymentConfiguration{
				DeploymentCircuitBreaker: &types.DeploymentCircuitBreaker{
					Enable:   false,
					Rollback: false,
				},
				MaximumPercent:        aws.Int32(200),
				MinimumHealthyPercent: aws.Int32(100),
			}
		} else if sv.DeploymentConfiguration.DeploymentCircuitBreaker == nil {
			sv.DeploymentConfiguration.DeploymentCircuitBreaker = &types.DeploymentCircuitBreaker{
				Enable:   false,
				Rollback: false,
			}
		}
	} else if sv.SchedulingStrategy == types.SchedulingStrategyDaemon && sv.DeploymentConfiguration == nil {
		sv.DeploymentConfiguration = &types.DeploymentConfiguration{
			MaximumPercent:        aws.Int32(100),
			MinimumHealthyPercent: aws.Int32(0),
		}
	}

	if nc := sv.NetworkConfiguration; nc != nil {
		if ac := nc.AwsvpcConfiguration; ac != nil {
			if ac.AssignPublicIp == "" {
				ac.AssignPublicIp = types.AssignPublicIpDisabled
			}
			sort.SliceStable(ac.SecurityGroups, func(i, j int) bool {
				return ac.SecurityGroups[i] < ac.SecurityGroups[j]
			})
			sort.SliceStable(ac.Subnets, func(i, j int) bool {
				return ac.Subnets[i] < ac.Subnets[j]
			})
		}
	}
	return &ServiceForDiff{
		UpdateServiceInput: svToUpdateServiceInput(sv),
		Tags:               sv.Tags,
	}
}

func sortTaskDefinition(td *TaskDefinitionInput) {
	if td == nil {
		return
	}
	for i, cd := range td.ContainerDefinitions {
		sort.SliceStable(cd.Environment, func(i, j int) bool {
			return aws.ToString(cd.Environment[i].Name) < aws.ToString(cd.Environment[j].Name)
		})
		sort.SliceStable(cd.MountPoints, func(i, j int) bool {
			return jsonStr(cd.MountPoints[i]) < jsonStr(cd.MountPoints[j])
		})
		sort.SliceStable(cd.PortMappings, func(i, j int) bool {
			return jsonStr(cd.PortMappings[i]) < jsonStr(cd.PortMappings[j])
		})
		// fill hostPort only when networkMode is awsvpc
		if td.NetworkMode == types.NetworkModeAwsvpc {
			for i, pm := range cd.PortMappings {
				if pm.HostPort == nil {
					pm.HostPort = pm.ContainerPort
				}
				cd.PortMappings[i] = pm
			}
		}
		sort.SliceStable(cd.VolumesFrom, func(i, j int) bool {
			return jsonStr(cd.VolumesFrom[i]) < jsonStr(cd.VolumesFrom[j])
		})
		sort.SliceStable(cd.Secrets, func(i, j int) bool {
			return aws.ToString(cd.Secrets[i].Name) < aws.ToString(cd.Secrets[j].Name)
		})
		td.ContainerDefinitions[i] = cd // set sorted value
	}
	sort.SliceStable(td.PlacementConstraints, func(i, j int) bool {
		return jsonStr(td.PlacementConstraints[i]) < jsonStr(td.PlacementConstraints[j])
	})
	sort.SliceStable(td.RequiresCompatibilities, func(i, j int) bool {
		return td.RequiresCompatibilities[i] < td.RequiresCompatibilities[j]
	})
	sort.SliceStable(td.Volumes, func(i, j int) bool {
		return jsonStr(td.Volumes[i]) < jsonStr(td.Volumes[j])
	})
	sort.SliceStable(td.Tags, func(i, j int) bool {
		return aws.ToString(td.Tags[i].Key) < aws.ToString(td.Tags[j].Key)
	})
	// containerDefinitions are sorted by name
	sort.SliceStable(td.ContainerDefinitions, func(i, j int) bool {
		return aws.ToString(td.ContainerDefinitions[i].Name) < aws.ToString(td.ContainerDefinitions[j].Name)
	})

	if td.Cpu != nil {
		td.Cpu = toNumberCPU(*td.Cpu)
	}
	if td.Memory != nil {
		td.Memory = toNumberMemory(*td.Memory)
	}
	if td.ProxyConfiguration != nil && len(td.ProxyConfiguration.Properties) > 0 {
		p := td.ProxyConfiguration.Properties
		sort.SliceStable(p, func(i, j int) bool {
			return aws.ToString(p[i].Name) < aws.ToString(p[j].Name)
		})
	}
}

func toNumberCPU(cpu string) *string {
	if i := strings.Index(strings.ToLower(cpu), "vcpu"); i > 0 {
		if ns, err := strconv.ParseFloat(strings.Trim(cpu[0:i], " "), 64); err != nil {
			return nil
		} else {
			nn := fmt.Sprintf("%d", int(ns*1024))
			return &nn
		}
	}
	return &cpu
}

func toNumberMemory(memory string) *string {
	if i := strings.Index(memory, "GB"); i > 0 {
		if ns, err := strconv.ParseFloat(strings.Trim(memory[0:i], " "), 64); err != nil {
			return nil
		} else {
			nn := fmt.Sprintf("%d", int(ns*1024))
			return &nn
		}
	}
	return &memory
}

func toDiffString(b []byte) string {
	if bytes.Equal(b, []byte("null")) || bytes.Equal(b, []byte("null\n")) {
		return ""
	}
	return string(b)
}
