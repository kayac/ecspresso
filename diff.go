package ecspresso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/mattn/go-shellwords"
)

type DiffOption struct {
	Unified  bool   `help:"unified diff format" default:"true" negatable:""`
	External string `help:"external command to format diff" env:"ECSPRESSO_DIFF_COMMAND"`

	w io.Writer `kong:"-"`
}

func (d *App) Diff(ctx context.Context, opt DiffOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()
	if opt.w == nil {
		opt.w = os.Stdout
	}

	var remoteTaskDefArn string
	// diff for services only when service defined
	if d.config.Service != "" {
		d.LogDebug("diff service compare with %s", d.config.Service)
		newSv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return fmt.Errorf("failed to load service definition: %w", err)
		}
		remoteSv, err := d.DescribeService(ctx)
		if err != nil {
			if errors.As(err, &errNotFound) {
				d.LogInfo("service not found, will create a new service")
			} else {
				return fmt.Errorf("failed to describe service: %w", err)
			}
		}
		if _, err := diffServices(ctx, newSv, remoteSv, d.config.ServiceDefinitionPath, &opt); err != nil {
			return err
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
				d.LogInfo("task definition not found, will register a new task definition")
			} else {
				return err
			}
		}
		remoteTaskDefArn = arn
	}
	var remoteTd *TaskDefinitionInput
	if remoteTaskDefArn != "" {
		d.LogDebug("diff task definition compare with %s", remoteTaskDefArn)
		remoteTd, err = d.DescribeTaskDefinition(ctx, remoteTaskDefArn)
		if err != nil {
			return err
		}
	}

	if _, err := diffTaskDefs(ctx, newTd, remoteTd, d.config.TaskDefinitionPath, remoteTaskDefArn, &opt); err != nil {
		return err
	}

	return nil
}

type ServiceForDiff struct {
	*ecs.UpdateServiceInput
	Tags []types.Tag
}

func diffServices(ctx context.Context, local, remote *Service, localPath string, opt *DiffOption) (bool, error) {
	var remoteArn string
	if remote != nil {
		remoteArn = aws.ToString(remote.ServiceArn)
	}

	localSvForDiff := ServiceDefinitionForDiff(local)
	remoteSvForDiff := ServiceDefinitionForDiff(remote)

	newSvBytes, err := MarshalJSONForAPI(localSvForDiff)
	if err != nil {
		return false, fmt.Errorf("failed to marshal new service definition: %w", err)
	}
	if local.DesiredCount == nil && remoteSvForDiff != nil {
		// ignore DesiredCount when it in local is not defined.
		remoteSvForDiff.UpdateServiceInput.DesiredCount = nil
	}
	remoteSvBytes, err := MarshalJSONForAPI(remoteSvForDiff)
	if err != nil {
		return false, fmt.Errorf("failed to marshal remote service definition: %w", err)
	}

	remoteSv := toDiffString(remoteSvBytes)
	newSv := toDiffString(newSvBytes)
	if remoteSv == newSv {
		return false, nil
	}

	switch {
	case opt.External != "":
		return true, diffExternal(ctx, opt.External, "service", remoteSv, newSv, opt)
	case opt.Unified:
		edits := myers.ComputeEdits(span.URIFromPath(remoteArn), remoteSv, newSv)
		ds := fmt.Sprint(gotextdiff.ToUnified(remoteArn, localPath, remoteSv, edits))
		fmt.Fprint(opt.w, coloredDiff(ds))
		return true, nil
	default:
		ds := diff.Diff(remoteSv, newSv)
		fmt.Fprint(opt.w, coloredDiff(fmt.Sprintf("--- %s\n+++ %s\n%s", remoteArn, localPath, ds)))
		return true, nil
	}
}

func diffTaskDefs(ctx context.Context, local, remote *TaskDefinitionInput, localPath, remoteArn string, opt *DiffOption) (bool, error) {
	sortTaskDefinition(local)
	sortTaskDefinition(remote)

	newTdBytes, err := MarshalJSONForAPI(local)
	if err != nil {
		return false, fmt.Errorf("failed to marshal new task definition: %w", err)
	}

	remoteTdBytes, err := MarshalJSONForAPI(remote)
	if err != nil {
		return false, fmt.Errorf("failed to marshal remote task definition: %w", err)
	}

	remoteTd := toDiffString(remoteTdBytes)
	newTd := toDiffString(newTdBytes)
	if remoteTd == newTd {
		return false, nil
	}

	switch {
	case opt.External != "":
		return true, diffExternal(ctx, opt.External, "taskdef", remoteTd, newTd, opt)
	case opt.Unified:
		edits := myers.ComputeEdits(span.URIFromPath(remoteArn), remoteTd, newTd)
		ds := fmt.Sprint(gotextdiff.ToUnified(remoteArn, localPath, remoteTd, edits))
		fmt.Fprint(opt.w, coloredDiff(ds))
		return true, nil
	default:
		ds := diff.Diff(remoteTd, newTd)
		fmt.Fprint(opt.w, coloredDiff(fmt.Sprintf("--- %s\n+++ %s\n%s", remoteArn, localPath, ds)))
		return true, nil
	}
}

func diffExternal(ctx context.Context, diffCmd string, target, remote, local string, opt *DiffOption) error {
	args, err := shellwords.Parse(diffCmd)
	if err != nil {
		return fmt.Errorf("failed to parse diff command: %w", err)
	}
	tmpDir, err := os.MkdirTemp("", "ecspresso-diff-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to getwd: %w", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		return fmt.Errorf("failed to chdir: %w", err)
	}
	defer os.Chdir(pwd)

	for _, name := range []string{"remote", "local"} {
		if err := os.Mkdir(name, 0755); err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
	}
	remoteFile := filepath.Join("remote", target+".json")
	localFile := filepath.Join("local", target+".json")
	if err := os.WriteFile(remoteFile, []byte(remote), 0644); err != nil {
		return fmt.Errorf("failed to write remote file: %w", err)
	}
	if err := os.WriteFile(localFile, []byte(local), 0644); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}
	args = append(args, remoteFile, localFile)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = opt.w
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run diff command: %w", err)
	}
	return nil
}

func coloredDiff(src string) string {
	if color.NoColor {
		// disable color
		return src
	}
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
		IpcMode:                 td.IpcMode,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		PlacementConstraints:    td.PlacementConstraints,
		PidMode:                 td.PidMode,
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

func jsonStr(v any) string {
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
	sort.SliceStable(sv.VolumeConfigurations, func(i, j int) bool {
		return aws.ToString(sv.VolumeConfigurations[i].Name) < aws.ToString(sv.VolumeConfigurations[j].Name)
	})
	sort.SliceStable(sv.VpcLatticeConfigurations, func(i, j int) bool {
		return aws.ToString(sv.VpcLatticeConfigurations[i].PortName) < aws.ToString(sv.VpcLatticeConfigurations[j].PortName)
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
