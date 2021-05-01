package ecspresso

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/fatih/color"
	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"
)

func diffServices(local, remote *ecs.Service) (string, error) {
	sortServiceDefinitionForDiff(local)
	sortServiceDefinitionForDiff(remote)

	newSvBytes, err := MarshalJSON(svToUpdateServiceInput(local))
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal new service definition")
	}
	if local.DesiredCount == nil {
		// ignore DesiredCount when it in local is not defined.
		remote.DesiredCount = nil
	}
	remoteSvBytes, err := MarshalJSON(svToUpdateServiceInput(remote))
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal remote service definition")
	}

	return diff.Diff(string(remoteSvBytes), string(newSvBytes)), nil
}

func diffTaskDefs(local, remote *TaskDefinitionInput) (string, error) {
	sortTaskDefinitionForDiff(local)
	sortTaskDefinitionForDiff(remote)

	newTdBytes, err := MarshalJSON(local)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal new task definition")
	}

	remoteTdBytes, err := MarshalJSON(remote)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal remote task definition")
	}

	return diff.Diff(string(remoteTdBytes), string(newTdBytes)), nil
}

func (d *App) Diff(opt DiffOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	// service definition
	newSv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load service definition")
	}
	remoteSv, err := d.DescribeService(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to describe service")
	}

	if ds, err := diffServices(newSv, remoteSv); err != nil {
		return err
	} else if ds != "" {
		fmt.Println(color.RedString("--- %s", *remoteSv.ServiceArn))
		fmt.Println(color.GreenString("+++ %s", d.config.ServiceDefinitionPath))
		fmt.Print(coloredDiff(ds))
	}

	// task definition
	newTd, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}
	remoteTd, err := d.DescribeTaskDefinition(ctx, *remoteSv.TaskDefinition)
	if err != nil {
		return errors.Wrap(err, "failed to describe task definition")
	}

	if ds, err := diffTaskDefs(newTd, remoteTd); err != nil {
		return err
	} else if ds != "" {
		fmt.Println(color.RedString("--- %s", *remoteSv.TaskDefinition))
		fmt.Println(color.GreenString("+++ %s", d.config.TaskDefinitionPath))
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

func tdToTaskDefinitionInput(td *TaskDefinition, tdTags []*ecs.Tag) *TaskDefinitionInput {
	return &TaskDefinitionInput{
		ContainerDefinitions:    td.ContainerDefinitions,
		Cpu:                     td.Cpu,
		EphemeralStorage:        td.EphemeralStorage,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		Family:                  td.Family,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		PlacementConstraints:    td.PlacementConstraints,
		RequiresCompatibilities: td.RequiresCompatibilities,
		TaskRoleArn:             td.TaskRoleArn,
		ProxyConfiguration:      td.ProxyConfiguration,
		Volumes:                 td.Volumes,
		Tags:                    tdTags,
	}
}

func tdInputToTaskDefinitionWithTags(td *TaskDefinitionInput) (*TaskDefinition, []*ecs.Tag) {
	return &ecs.TaskDefinition{
		ContainerDefinitions:    td.ContainerDefinitions,
		Cpu:                     td.Cpu,
		ExecutionRoleArn:        td.ExecutionRoleArn,
		Family:                  td.Family,
		Memory:                  td.Memory,
		NetworkMode:             td.NetworkMode,
		PlacementConstraints:    td.PlacementConstraints,
		RequiresCompatibilities: td.RequiresCompatibilities,
		TaskRoleArn:             td.TaskRoleArn,
		ProxyConfiguration:      td.ProxyConfiguration,
		Volumes:                 td.Volumes,
	}, td.Tags
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

func sortServiceDefinitionForDiff(sv *ecs.Service) {
	sortSlicesInDefinition(
		reflect.TypeOf(*sv), reflect.Indirect(reflect.ValueOf(sv)),
		"PlacementConstraints",
		"PlacementStrategy",
		"RequiresCompatibilities",
	)
	if equalString(sv.LaunchType, ecs.LaunchTypeFargate) && sv.PlatformVersion == nil {
		sv.PlatformVersion = aws.String("LATEST")
	}
	if sv.SchedulingStrategy == nil && sv.DeploymentConfiguration == nil {
		sv.DeploymentConfiguration = &ecs.DeploymentConfiguration{
			MaximumPercent:        aws.Int64(200),
			MinimumHealthyPercent: aws.Int64(100),
		}
	} else if equalString(sv.SchedulingStrategy, ecs.SchedulingStrategyDaemon) && sv.DeploymentConfiguration == nil {
		sv.DeploymentConfiguration = &ecs.DeploymentConfiguration{
			MaximumPercent:        aws.Int64(100),
			MinimumHealthyPercent: aws.Int64(0),
		}
	}

	if len(sv.LoadBalancers) > 0 && sv.HealthCheckGracePeriodSeconds == nil {
		sv.HealthCheckGracePeriodSeconds = aws.Int64(0)
	}
	if nc := sv.NetworkConfiguration; nc != nil {
		if ac := nc.AwsvpcConfiguration; ac != nil {
			if ac.AssignPublicIp == nil {
				ac.AssignPublicIp = aws.String(ecs.AssignPublicIpDisabled)
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
		if cd.Cpu == nil {
			cd.Cpu = aws.Int64(0)
		}
		sortSlicesInDefinition(
			reflect.TypeOf(*cd), reflect.Indirect(reflect.ValueOf(cd)),
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
