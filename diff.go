package ecspresso

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"
)

func diffServices(local, remote *Service, remoteArn string, localPath string, unified bool) (string, error) {
	sortServiceDefinitionForDiff(local)
	sortServiceDefinitionForDiff(remote)

	newSvBytes, err := MarshalJSONForAPI(svToUpdateServiceInput(local))
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal new service definition")
	}
	if local.DesiredCount == 0 {
		// ignore DesiredCount when it in local is not defined.
		remote.DesiredCount = 0
	}
	remoteSvBytes, err := MarshalJSONForAPI(svToUpdateServiceInput(remote))
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal remote service definition")
	}

	remoteSv := string(remoteSvBytes)
	newSv := string(newSvBytes)

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

func diffTaskDefs(local, remote *TaskDefinitionInput, remoteArn string, localPath string, unified bool) (string, error) {
	sortTaskDefinitionForDiff(local)
	sortTaskDefinitionForDiff(remote)

	newTdBytes, err := MarshalJSONForAPI(local)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal new task definition")
	}

	remoteTdBytes, err := MarshalJSONForAPI(remote)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal remote task definition")
	}

	remoteTd := string(remoteTdBytes)
	newTd := string(newTdBytes)

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

func (d *App) Diff(opt DiffOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	var taskDefArn string
	// diff for services only when service defined
	if d.config.Service != "" {
		newSv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
		if err != nil {
			return errors.Wrap(err, "failed to load service definition")
		}
		remoteSv, err := d.DescribeService(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to describe service")
		}

		if ds, err := diffServices(newSv, remoteSv, *remoteSv.ServiceArn, d.config.ServiceDefinitionPath, *opt.Unified); err != nil {
			return err
		} else if ds != "" {
			fmt.Print(coloredDiff(ds))
		}
		taskDefArn = *remoteSv.TaskDefinition
	}

	// task definition
	newTd, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}
	if taskDefArn == "" {
		arn, err := d.findLatestTaskDefinitionArn(ctx, *newTd.Family)
		if err != nil {
			return errors.Wrap(err, "failed to find latest task definition from family")
		}
		taskDefArn = arn
	}
	remoteTd, err := d.DescribeTaskDefinition(ctx, taskDefArn)
	if err != nil {
		return errors.Wrap(err, "failed to describe task definition")
	}

	if ds, err := diffTaskDefs(newTd, remoteTd, taskDefArn, d.config.TaskDefinitionPath, *opt.Unified); err != nil {
		return err
	} else if ds != "" {
		fmt.Print(coloredDiff(ds))
	}

	return nil
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

var stringerType = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()

func sortSlicesInDefinition(t reflect.Type, v reflect.Value, fieldNames ...string) {
	isSortableField := func(name string) bool {
		for _, n := range fieldNames {
			if n == name {
				return true
			}
		}
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		fv, field := v.Field(i), t.Field(i)
		if fv.Kind() != reflect.Slice || !fv.CanSet() {
			continue
		}
		if !isSortableField(field.Name) {
			continue
		}
		if size := fv.Len(); size == 0 {
			fv.Set(reflect.MakeSlice(fv.Type(), 0, 0))
		} else {
			slice := make([]reflect.Value, size, size)
			for i := 0; i < size; i++ {
				slice[i] = fv.Index(i)
			}
			sort.SliceStable(slice, func(i, j int) bool {
				iv, jv := reflect.Indirect(slice[i]), reflect.Indirect(slice[j])
				var is, js string
				if iv.Kind() == reflect.String && jv.Kind() == reflect.String {
					is, js = iv.Interface().(string), jv.Interface().(string)
				} else if iv.Type().Implements(stringerType) && jv.Type().Implements(stringerType) {
					is, js = iv.Interface().(fmt.Stringer).String(), jv.Interface().(fmt.Stringer).String()
				}
				return is < js
			})
			sorted := reflect.MakeSlice(fv.Type(), size, size)
			for i := 0; i < size; i++ {
				sorted.Index(i).Set(slice[i])
			}
			fv.Set(sorted)
		}
	}
}

func equalString(a *string, b string) bool {
	if a == nil {
		return b == ""
	}
	return *a == b
}

func sortServiceDefinitionForDiff(sv *Service) {
	sortSlicesInDefinition(
		reflect.TypeOf(*sv), reflect.Indirect(reflect.ValueOf(sv)),
		"PlacementConstraints",
		"PlacementStrategy",
		"RequiresCompatibilities",
	)
	if sv.LaunchType == types.LaunchTypeFargate && sv.PlatformVersion == nil {
		sv.PlatformVersion = aws.String("LATEST")
	}
	if sv.SchedulingStrategy == "" && sv.DeploymentConfiguration == nil {
		sv.DeploymentConfiguration = &types.DeploymentConfiguration{
			MaximumPercent:        aws.Int32(200),
			MinimumHealthyPercent: aws.Int32(100),
		}
	} else if sv.SchedulingStrategy == types.SchedulingStrategyDaemon && sv.DeploymentConfiguration == nil {
		sv.DeploymentConfiguration = &types.DeploymentConfiguration{
			MaximumPercent:        aws.Int32(100),
			MinimumHealthyPercent: aws.Int32(0),
		}
	}

	if len(sv.LoadBalancers) > 0 && sv.HealthCheckGracePeriodSeconds == nil {
		sv.HealthCheckGracePeriodSeconds = aws.Int32(0)
	}
	if nc := sv.NetworkConfiguration; nc != nil {
		if ac := nc.AwsvpcConfiguration; ac != nil {
			if ac.AssignPublicIp == "" {
				ac.AssignPublicIp = types.AssignPublicIpDisabled
			}
			sortSlicesInDefinition(
				reflect.TypeOf(*ac),
				reflect.Indirect(reflect.ValueOf(ac)),
				"SecurityGroups",
				"Subnets",
			)
		}
	}
}

func sortTaskDefinitionForDiff(td *TaskDefinitionInput) {
	for _, cd := range td.ContainerDefinitions {
		sortSlicesInDefinition(
			reflect.TypeOf(cd), reflect.Indirect(reflect.ValueOf(cd)),
			"Environment",
			"MountPoints",
			"PortMappings",
			"VolumesFrom",
			"Secrets",
		)
	}
	sortSlicesInDefinition(
		reflect.TypeOf(*td), reflect.Indirect(reflect.ValueOf(td)),
		"PlacementConstraints",
		"RequiresCompatibilities",
		"Volumes",
		"Tags",
	)
	// containerDefinitions are sorted by name
	sort.SliceStable(td.ContainerDefinitions, func(i, j int) bool {
		return *(td.ContainerDefinitions[i].Name) > *(td.ContainerDefinitions[j].Name)
	})

	if td.Cpu != nil {
		td.Cpu = toNumberCPU(*td.Cpu)
	}
	if td.Memory != nil {
		td.Memory = toNumberMemory(*td.Memory)
	}
	if td.ProxyConfiguration != nil && len(td.ProxyConfiguration.Properties) > 0 {
		sortSlicesInDefinition(
			reflect.TypeOf(*td.ProxyConfiguration), reflect.Indirect(reflect.ValueOf(td.ProxyConfiguration)),
			"Properties",
		)
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
