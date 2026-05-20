package ecspresso

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

type ExpressGatewayService struct {
	*ecs.CreateExpressGatewayServiceInput
	src *types.ECSExpressGatewayService
}

func (e *ExpressGatewayService) SetTags(tags []types.Tag) {
	e.Tags = tags
}

func (e *ExpressGatewayService) GetTags() []types.Tag {
	return e.Tags
}

func (d *App) initExpressGatewayService(ctx context.Context, sv *Service, opt InitOption) (*ExpressGatewayService, *Service, error) {
	conf := d.config
	d.LogInfo("initializing express definition", "service", aws.ToString(sv.ServiceName))

	ex, err := d.DescribeExpressGatewayService(ctx, sv)
	if err != nil {
		return nil, nil, err
	}

	// remove fields in definition that will be set by config
	ex.ServiceName = nil
	ex.Cluster = nil

	b, err := MarshalJSONForAPI(ex)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal express gateway service to JSON: %w", err)
	}
	if opt.Jsonnet {
		out, err := toJsonnetString(string(b), conf.ExpressDefinitionPath)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to format express gateway service as Jsonnet: %w", err)
		}
		b = []byte(out)
	}
	d.LogInfo("saving express gateway service", "service", aws.ToString(ex.ServiceName), "path", conf.ExpressDefinitionPath)
	if err := d.saveFile(conf.ExpressDefinitionPath, b, CreateFileMode, opt.ForceOverwrite); err != nil {
		return nil, nil, err
	}

	return ex, sv, nil
}

func (d *App) DescribeExpressGatewayService(ctx context.Context, sv *Service) (*ExpressGatewayService, error) {
	if !sv.isExpressMode() {
		return nil, fmt.Errorf("service %s is not an express mode", aws.ToString(sv.ServiceName))
	}
	res, err := d.ecs.DescribeExpressGatewayService(ctx, &ecs.DescribeExpressGatewayServiceInput{
		ServiceArn: sv.ServiceArn,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe express gateway service: %w", err)
	}
	rex := res.Service
	if res.Service == nil {
		return nil, ErrNotFound("express gateway service is not found")
	}
	if len(rex.ActiveConfigurations) == 0 {
		return nil, fmt.Errorf("express gateway service %s has no active configuration", aws.ToString(rex.ServiceName))
	}
	ac := rex.ActiveConfigurations[0]
	in := &ecs.CreateExpressGatewayServiceInput{
		Cluster:               aws.String(d.config.Cluster),
		ServiceName:           rex.ServiceName,
		InfrastructureRoleArn: rex.InfrastructureRoleArn,
		ExecutionRoleArn:      ac.ExecutionRoleArn,
		Cpu:                   ac.Cpu,
		Memory:                ac.Memory,
		HealthCheckPath:       ac.HealthCheckPath,
		NetworkConfiguration:  ac.NetworkConfiguration,
		ScalingTarget:         ac.ScalingTarget,
		TaskRoleArn:           ac.TaskRoleArn,
		PrimaryContainer:      ac.PrimaryContainer,
		Tags:                  sv.Tags,
	}
	return &ExpressGatewayService{
		CreateExpressGatewayServiceInput: in,
		src:                              rex,
	}, nil
}

func (d *App) createExpressGatewayService(ctx context.Context, opt DeployOption) error {
	ex, err := d.LoadExpressDefinition(d.config.ExpressDefinitionPath)
	if err != nil {
		return err
	}
	d.LogInfo("creating express gateway service", withDryRun(opt.DryRun, "service", aws.ToString(ex.ServiceName))...)
	if opt.DryRun {
		OutputJSONForAPI(os.Stdout, ex)
		return nil
	}
	res, err := d.ecs.CreateExpressGatewayService(ctx, ex.CreateExpressGatewayServiceInput)
	if err != nil {
		return fmt.Errorf("failed to create express gateway service: %w", err)
	}
	d.LogInfo("Express gateway service is created")
	d.LogJSON(res.Service)

	if !opt.Wait {
		return nil
	}
	sleepContext(ctx, delayForServiceChanged) // wait for service created
	sv, err := d.DescribeService(ctx)
	if err != nil {
		return err
	}
	return d.WaitServiceDeployCompleted(ctx, sv)
}

func (d *App) LoadExpressDefinition(path string) (*ExpressGatewayService, error) {
	if path == "" {
		return nil, fmt.Errorf("express_definition is not defined")
	}

	var ex ExpressGatewayService
	src, err := d.readDefinitionFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load express definition %s: %w", path, err)
	}
	if err := unmarshalJSON(src, &ex, path); err != nil {
		return nil, fmt.Errorf("failed to load express definition %s: %w", path, err)
	}

	ex.ServiceName = aws.String(d.config.Service)
	ex.Cluster = &d.config.Cluster

	if err := d.config.Ignore.Apply(&ex); err != nil {
		return nil, fmt.Errorf("failed to apply ignore: %w", err)
	}
	return &ex, nil
}

func diffExpressGatewayServices(ctx context.Context, local, remote *ExpressGatewayService, localPath string, opt *DiffOption) (bool, error) {
	var remoteArn string
	if remote != nil {
		remoteArn = aws.ToString(remote.ServiceName)
	}

	sortExpressDefinition(local)
	sortExpressDefinition(remote)
	ignores := buildIgnoreQueryForExpressDefinition(local)

	newSvBytes, err := MarshalJSONForAPI(local, ignores...)
	if err != nil {
		return false, fmt.Errorf("failed to marshal new express gateway service definition: %w", err)
	}
	remoteSvBytes, err := MarshalJSONForAPI(remote, ignores...)
	if err != nil {
		return false, fmt.Errorf("failed to marshal remote express gateway service definition: %w", err)
	}

	remoteSv := toDiffString(remoteSvBytes)
	newSv := toDiffString(newSvBytes)
	if remoteSv == newSv {
		return false, nil
	}

	if opt.External != "" {
		return true, diffExternal(ctx, opt.External, "service", remoteSv, newSv, opt)
	}
	edits := myers.ComputeEdits(span.URIFromPath(remoteArn), remoteSv, newSv)
	ds := fmt.Sprint(gotextdiff.ToUnified(remoteArn, localPath, remoteSv, edits))
	fmt.Fprint(opt.w, coloredDiff(ds))
	return true, nil
}

func exToUpdateExpressGatewayServiceInput(ex *ExpressGatewayService, sv *Service) *ecs.UpdateExpressGatewayServiceInput {
	in := &ecs.UpdateExpressGatewayServiceInput{
		ServiceArn:           sv.ServiceArn,
		Cpu:                  ex.Cpu,
		Memory:               ex.Memory,
		HealthCheckPath:      ex.HealthCheckPath,
		NetworkConfiguration: ex.NetworkConfiguration,
		ScalingTarget:        ex.ScalingTarget,
		ExecutionRoleArn:     ex.ExecutionRoleArn,
		TaskRoleArn:          ex.TaskRoleArn,
		PrimaryContainer:     ex.PrimaryContainer,
	}

	// explicitly set empty values to reset them
	if pc := ex.PrimaryContainer; pc != nil {
		if len(pc.Environment) == 0 {
			pc.Environment = []types.KeyValuePair{}
		}
		if len(pc.Secrets) == 0 {
			pc.Secrets = []types.Secret{}
		}
	}

	return in
}

func (d *App) DeployExpressGatewayService(ctx context.Context, sv *Service, opt DeployOption) error {
	if !sv.isExpressMode() {
		return fmt.Errorf("service %s is not an express mode", aws.ToString(sv.ServiceName))
	}
	if !opt.UpdateService {
		return fmt.Errorf("express gateway service does not support this operation")
	}

	ex, err := d.LoadExpressDefinition(d.config.ExpressDefinitionPath)
	if err != nil {
		return err
	}
	d.LogInfo("updating express gateway service", withDryRun(opt.DryRun, "service", aws.ToString(sv.ServiceName))...)

	in := exToUpdateExpressGatewayServiceInput(ex, sv)

	addedTags, updatedTags, deletedTags := CompareTags(sv.Tags, ex.Tags)
	doUpdateTags := func() error {
		return d.UpdateServiceTags(ctx, sv, addedTags, updatedTags, deletedTags, opt)
	}

	if opt.DryRun {
		OutputJSONForAPI(os.Stdout, in)
		return doUpdateTags()
	}
	res, err := d.ecs.UpdateExpressGatewayService(ctx, in)
	if err != nil {
		return fmt.Errorf("failed to update express gateway service: %w", err)
	}
	d.LogInfo("Express gateway service is updated")
	d.LogJSON(res.Service)

	// update tags
	if err := doUpdateTags(); err != nil {
		return err
	}

	if !opt.Wait {
		return nil
	}
	sleepContext(ctx, delayForServiceChanged) // wait for service updated
	// wait for service deployed
	return d.WaitServiceDeployCompleted(ctx, sv)
}

func (d *App) deleteExpressGatewayService(ctx context.Context, sv *Service, _ DeleteOption) error {
	in := &ecs.DeleteExpressGatewayServiceInput{
		ServiceArn: sv.ServiceArn,
	}
	if _, err := d.ecs.DeleteExpressGatewayService(ctx, in); err != nil {
		return fmt.Errorf("failed to delete express gateway service: %w", err)
	}
	d.LogInfo("Service is deleted")
	return nil
}

// build ignore query
// ECS Express sets some default values, so ignore them in diff when the local definition doesn't set them.
func buildIgnoreQueryForExpressDefinition(ex *ExpressGatewayService) []string {
	ignores := []string{}

	if ex.NetworkConfiguration == nil {
		ignores = append(ignores, "del(.networkConfiguration)")
	}
	if ex.ScalingTarget == nil {
		ignores = append(ignores, "del(.scalingTarget)")
	}
	if ex.PrimaryContainer != nil && ex.PrimaryContainer.AwsLogsConfiguration == nil {
		ignores = append(ignores, "del(.primaryContainer.awsLogsConfiguration)")
	}

	return ignores
}

func sortExpressDefinition(ex *ExpressGatewayService) {
	if ex == nil {
		return
	}
	if nc := ex.NetworkConfiguration; nc != nil {
		sort.Strings(nc.SecurityGroups)
		sort.Strings(nc.Subnets)
	}
	sort.SliceStable(ex.Tags, func(i, j int) bool {
		return aws.ToString(ex.Tags[i].Key) < aws.ToString(ex.Tags[j].Key)
	})

	pc := ex.PrimaryContainer
	sort.SliceStable(pc.Environment, func(i, j int) bool {
		return aws.ToString(pc.Environment[i].Name) < aws.ToString(pc.Environment[j].Name)
	})
	sort.SliceStable(pc.Secrets, func(i, j int) bool {
		return aws.ToString(pc.Secrets[i].Name) < aws.ToString(pc.Secrets[j].Name)
	})

	// fill default values to make diff stable
	if ex.Cpu == nil {
		ex.Cpu = aws.String("1024")
	}
	if ex.Memory == nil {
		ex.Memory = aws.String("2048")
	}
	if ex.HealthCheckPath == nil {
		ex.HealthCheckPath = aws.String("/")
	}
	if pc := ex.PrimaryContainer; pc != nil {
		if pc.ContainerPort == nil {
			pc.ContainerPort = aws.Int32(80)
		}
	}
}
