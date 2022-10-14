package ecspresso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogsTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso/registry"
)

type verifier struct {
	cwl            *cloudwatchlogs.Client
	ssm            *ssm.Client
	secretsmanager *secretsmanager.Client
	ecr            *ecr.Client
	s3             *s3.Client
	opt            *VerifyOption
	isAssumed      bool
}

func newVerifier(execCfg, appCfg aws.Config, opt *VerifyOption) *verifier {
	return &verifier{
		cwl:            cloudwatchlogs.NewFromConfig(execCfg),
		ssm:            ssm.NewFromConfig(execCfg),
		secretsmanager: secretsmanager.NewFromConfig(execCfg),
		ecr:            ecr.NewFromConfig(execCfg),
		s3:             s3.NewFromConfig(appCfg),
		opt:            opt,
		isAssumed:      &execCfg != &appCfg,
	}
}

func (v *verifier) existsSecretValue(ctx context.Context, from string) error {
	if !aws.ToBool(v.opt.GetSecrets) {
		return ErrSkipVerify(fmt.Sprintf("get a secret value for %s", from))
	}

	// secrets manager
	if strings.HasPrefix(from, "arn:aws:secretsmanager:") {
		// https://docs.aws.amazon.com/ja_jp/AmazonECS/latest/developerguide/specifying-sensitive-data-secrets.html
		// Truncate additional params in secretsmanager Arn.
		part := strings.Split(from, ":")
		if len(part) < 7 {
			return errors.New("invalid arn format")
		}
		secretArn := strings.Join(part[0:7], ":")
		_, err := v.secretsmanager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: &secretArn,
		})
		if err != nil {
			return fmt.Errorf("failed to get secret value from %s secret id %s: %w", from, secretArn, err)
		}
		return nil
	}

	// ssm
	var name string
	if strings.HasPrefix(from, "arn:aws:ssm:") {
		ns := strings.Split(from, ":")
		name = strings.TrimPrefix(ns[len(ns)-1], "parameter")
	} else {
		name = from
	}
	_, err := v.ssm.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("failed to get ssm parameter %s: %w", name, err)
	}
	return nil
}

func (v *verifier) existsEnvironmentFile(ctx context.Context, envFile types.EnvironmentFile) error {
	if envFile.Type != types.EnvironmentFileTypeS3 {
		return ErrSkipVerify("unsupported environment file type: " + string(envFile.Type))
	}
	s3arn := aws.ToString(envFile.Value)
	a, err := arn.Parse(s3arn)
	if err != nil {
		return fmt.Errorf("failed to parse s3 arn %s: %w", s3arn, err)
	}
	if a.Service != "s3" {
		return fmt.Errorf("invalid s3 arn %s", s3arn)
	}
	rs := strings.SplitN(a.Resource, "/", 2)
	if len(rs) != 2 {
		return fmt.Errorf("invalid s3 arn %s", s3arn)
	}
	bucket, key := rs[0], rs[1]
	if _, err = v.s3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}); err != nil {
		return fmt.Errorf("failed to head s3 object %s: %w", s3arn, err)
	}
	return nil
}

func (d *App) newAssumedVerifier(ctx context.Context, cfg aws.Config, executionRole *string, opt *VerifyOption) (*verifier, error) {
	if executionRole == nil {
		return newVerifier(cfg, cfg, opt), nil
	}
	svc := sts.NewFromConfig(d.config.awsv2Config)
	out, err := svc.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         executionRole,
		RoleSessionName: aws.String("ecspresso-verifier"),
	})
	if err != nil {
		d.Log("[INFO] failed to assume role to taskExecutionRole. Continue to verify with current session. %s", err.Error())
		return newVerifier(cfg, cfg, opt), nil
	}
	ec := aws.Config{}
	ec.Region = d.config.Region
	ec.Credentials = credentials.NewStaticCredentialsProvider(
		aws.ToString(out.Credentials.AccessKeyId),
		aws.ToString(out.Credentials.SecretAccessKey),
		aws.ToString(out.Credentials.SessionToken),
	)
	return newVerifier(ec, cfg, opt), nil
}

// VerifyOption represents options for Verify()
type VerifyOption struct {
	GetSecrets *bool
	PutLogs    *bool
}

type verifyResourceFunc func(context.Context) error

// Verify verifies service / task definitions related resources are valid.
func (d *App) Verify(ctx context.Context, opt VerifyOption) error {
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	d.verifier, err = d.newAssumedVerifier(ctx, d.config.awsv2Config, td.ExecutionRoleArn, &opt)
	if err != nil {
		return err
	}

	ctx, cancel := d.Start(ctx)
	defer cancel()

	d.Log("Starting verify")
	resources := []struct {
		name string
		fn   verifyResourceFunc
	}{
		{name: "TaskDefinition", fn: d.verifyTaskDefinition},
		{name: "ServiceDefinition", fn: d.verifyServiceDefinition},
		{name: "Cluster", fn: d.verifyCluster},
	}
	for _, r := range resources {
		if err := d.verifyResource(ctx, r.name, r.fn); err != nil {
			return err
		}
	}
	d.Log("Verify OK!")
	return nil
}

var verifyResourceNestLevel = 0

func (d *App) verifyResource(ctx context.Context, resourceType string, verifyFunc func(context.Context) error) error {
	verifyResourceNestLevel++
	defer func() { verifyResourceNestLevel-- }()
	indent := strings.Repeat("  ", verifyResourceNestLevel)
	print := func(f string, args ...interface{}) {
		fmt.Printf(indent+f+"\n", args...)
	}
	print("%s", resourceType)
	err := verifyFunc(ctx)
	if err != nil {
		if errors.As(err, &errSkipVerify) {
			print("--> %s [%s] %s", resourceType, color.CyanString("SKIP"), color.CyanString(err.Error()))
			return nil
		}
		print("--> %s [%s] %s", resourceType, color.RedString("NG"), color.RedString(err.Error()))
		return fmt.Errorf("verify %s failed: %w", resourceType, err)
	}
	print("--> [%s]", color.GreenString("OK"))
	return nil
}

func (d *App) verifyCluster(ctx context.Context) error {
	cluster := d.config.Cluster
	out, err := d.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: []string{cluster},
	})
	if err != nil {
		return fmt.Errorf("failed to describe cluster %s: %w", cluster, err)
	} else if len(out.Clusters) == 0 {
		return ErrNotFound(fmt.Sprintf("cluster %s is not found", cluster))
	}
	return nil
}

func (d *App) verifyServiceDefinition(ctx context.Context) error {
	if d.config.ServiceDefinitionPath == "" {
		return ErrSkipVerify("no ServiceDefinition")
	}
	sv, err := d.LoadServiceDefinition(d.config.ServiceDefinitionPath)
	if err != nil {
		return err
	}
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}

	// networkMode
	if td.NetworkMode == types.NetworkModeAwsvpc {
		if sv.NetworkConfiguration == nil || sv.NetworkConfiguration.AwsvpcConfiguration == nil {
			return errors.New(
				`networkConfiguration.awsvpcConfiguration required for the taskDefinition networkMode=awsvpc`,
			)
		}
	}

	// LB
	for i, lb := range sv.LoadBalancers {
		name := fmt.Sprintf("LoadBalancer[%d]", i)
		err := d.verifyResource(ctx, name, func(context.Context) error {
			out, err := d.elbv2.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
				TargetGroupArns: []string{*lb.TargetGroupArn},
			})
			if err != nil {
				return err
			} else if len(out.TargetGroups) == 0 {
				return ErrNotFound(fmt.Sprintf("target group %s is not found: %s", *lb.TargetGroupArn, err))
			}

			cname := aws.ToString(lb.ContainerName)
			cport := aws.ToInt32(lb.ContainerPort)
			var container *types.ContainerDefinition
		CONTAINER_DEF:
			for _, c := range td.ContainerDefinitions {
				if aws.ToString(c.Name) != cname {
					continue
				}
				for _, pm := range c.PortMappings {
					if aws.ToInt32(pm.ContainerPort) == cport {
						container = &c
						break CONTAINER_DEF
					}
				}
			}
			if container == nil {
				return fmt.Errorf("container name %s and port %d is not defined in task definition", cname, cport)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(sv.LoadBalancers) == 0 && sv.HealthCheckGracePeriodSeconds != nil {
		return fmt.Errorf("service has no load balancers, but healthCheckGracePeriodSeconds is defined.")
	}

	return nil
}

func (d *App) verifyTaskDefinition(ctx context.Context) error {
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}

	if execRole := td.ExecutionRoleArn; execRole != nil {
		name := fmt.Sprintf("ExecutionRole[%s]", *execRole)
		err := d.verifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyRole(ctx, *execRole)
		})
		if err != nil {
			return err
		}
	}
	if taskRole := td.TaskRoleArn; taskRole != nil {
		name := fmt.Sprintf("TaskRole[%s]", *taskRole)
		err := d.verifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyRole(ctx, *taskRole)
		})
		if err != nil {
			return err
		}
	}

	for _, c := range td.ContainerDefinitions {
		name := fmt.Sprintf("ContainerDefinition[%s]", aws.ToString(c.Name))
		err := d.verifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyContainer(ctx, &c, td)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	ecrImageURLRegex = regexp.MustCompile(`dkr\.ecr\..+.amazonaws\.com/.*`)
)

func (d *App) verifyECRImage(ctx context.Context, image string) error {
	d.Log("[DEBUG] VERIFY ECR Image")
	out, err := d.verifier.ecr.GetAuthorizationToken(
		ctx,
		&ecr.GetAuthorizationTokenInput{},
	)
	if err != nil {
		return err
	}
	token := out.AuthorizationData[0].AuthorizationToken
	return d.verifyRegistryImage(ctx, image, "AWS", aws.ToString(token))
}

func (d *App) verifyRegistryImage(ctx context.Context, image, user, password string) error {
	rr := strings.SplitN(image, ":", 2)
	image = rr[0]
	var tag string
	if len(rr) == 1 {
		tag = "latest"
	} else {
		tag = rr[1]
	}
	d.Log("[DEBUG] image=%s tag=%s", image, tag)

	repo := registry.New(image, user, password)
	ok, err := repo.HasImage(ctx, tag)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s:%s is not found in Registry", image, tag)
	}

	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	// when requiredCompatibilities contain only fargate, regard as fargate task definition
	isFargateTask := len(td.RequiresCompatibilities) == 1 && td.RequiresCompatibilities[0] == types.CompatibilityFargate
	isFargateService, err := d.isFargateService()
	if err != nil {
		return err
	}
	arch, os := NormalizePlatform(td.RuntimePlatform, isFargateTask || isFargateService)
	if arch == "" && os == "" {
		return nil
	}
	ok, err = repo.HasPlatformImage(ctx, tag, arch, os)
	if err != nil {
		if errors.Is(err, registry.ErrDeprecatedManifest) || errors.Is(err, registry.ErrPullRateLimitExceeded) {
			return ErrSkipVerify(err.Error())
		}
		return err
	}
	if ok {
		return nil
	}
	return fmt.Errorf("%s:%s for arch=%s os=%s is not found in Registry", image, tag, arch, os)
}

func (d *App) isFargateService() (bool, error) {
	p := d.config.ServiceDefinitionPath
	if p == "" {
		return false, nil
	}
	sv, err := d.LoadServiceDefinition(p)
	if err != nil {
		return false, err
	}
	if sv.PlatformVersion != nil && *sv.PlatformVersion != "" {
		return true, nil
	}
	if sv.LaunchType == types.LaunchTypeFargate {
		return true, nil
	}
	for _, s := range sv.CapacityProviderStrategy {
		name := *s.CapacityProvider
		if name == "FARGATE_SPOT" || name == "FARGATE" {
			return true, nil
		}
	}
	return false, nil
}

func NormalizePlatform(p *types.RuntimePlatform, isFargate bool) (arch, os string) {
	// if it is able to determine a fargate resource, set fargate default platform.
	// otherwise, default arch/os are empty as platform is not determined without RuntimePlatform.
	if isFargate {
		arch = "amd64"
		os = "linux"
	}
	if p == nil {
		return
	}

	// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RuntimePlatform.html
	if p.CpuArchitecture == types.CPUArchitectureArm64 {
		arch = "arm64"
	} else {
		arch = "amd64"
	}
	if p.OperatingSystemFamily == "" || p.OperatingSystemFamily == types.OSFamilyLinux {
		os = "linux"
	} else {
		os = "windows"
	}
	return
}

func (d *App) verifyImage(ctx context.Context, image string) error {
	if image == "" {
		return errors.New("image is not defined")
	}
	if ecrImageURLRegex.MatchString(image) {
		return d.verifyECRImage(ctx, image)
	}
	return d.verifyRegistryImage(ctx, image, "", "")
}

func (d *App) verifyContainer(ctx context.Context, c *types.ContainerDefinition, td *ecs.RegisterTaskDefinitionInput) error {
	image := aws.ToString(c.Image)
	name := fmt.Sprintf("Image[%s]", image)
	err := d.verifyResource(ctx, name, func(ctx context.Context) error {
		return d.verifyImage(ctx, image)
	})
	if err != nil {
		return err
	}
	for _, secret := range c.Secrets {
		name := fmt.Sprintf("Secret %s[%s]", *secret.Name, *secret.ValueFrom)
		err := d.verifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifier.existsSecretValue(ctx, *secret.ValueFrom)
		})
		if err != nil {
			return err
		}
	}
	if c.LogConfiguration != nil && c.LogConfiguration.LogDriver == types.LogDriverAwslogs {
		err := d.verifyResource(ctx, "LogConfiguration[awslogs]", func(ctx context.Context) error {
			return d.verifyLogConfiguration(ctx, c)
		})
		if err != nil {
			return err
		}
	}
	for _, envFile := range c.EnvironmentFiles {
		name := fmt.Sprintf("EnvironmentFile[%s %s]", envFile.Type, aws.ToString(envFile.Value))
		if err := d.verifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifier.existsEnvironmentFile(ctx, envFile)
		}); err != nil {
			return err
		}
	}

	if td.NetworkMode == types.NetworkModeAwsvpc {
		for _, pm := range c.PortMappings {
			if pm.HostPort != nil && aws.ToInt32(pm.ContainerPort) != aws.ToInt32(pm.HostPort) {
				return fmt.Errorf("hostPort must be same as containerPort for awsvpc networkMode")
			}
		}
	}

	return nil
}

func (d *App) verifyLogConfiguration(ctx context.Context, c *types.ContainerDefinition) error {
	options := c.LogConfiguration.Options
	d.Log("LogConfiguration[awslogs] options=%v", options)
	group, region, prefix := options["awslogs-group"], options["awslogs-region"], options["awslogs-stream-prefix"]
	if group == "" {
		return errors.New("awslogs-group is required")
	}
	if region == "" {
		return errors.New("awslogs-region is required")
	}

	if !aws.ToBool(d.verifier.opt.PutLogs) {
		return ErrSkipVerify(fmt.Sprintf("putting logs to %s", group))
	}

	var stream string
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	if prefix != "" {
		stream = fmt.Sprintf("%s/%s/%s-%s", prefix, *c.Name, "ecspresso-verify", suffix)
	} else {
		stream = fmt.Sprintf("%s/%s-%s", *c.Name, "ecspresso-verify", suffix)
	}

	if _, err := d.verifier.cwl.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  &group,
		LogStreamName: aws.String(stream),
	}); err != nil {
		return fmt.Errorf("failed to create log stream %s in %s: %w", stream, group, err)
	}
	if _, err := d.verifier.cwl.PutLogEvents(ctx, &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &group,
		LogStreamName: aws.String(stream),
		LogEvents: []cloudwatchlogsTypes.InputLogEvent{
			{
				Message:   aws.String("This is a verify message by ecspresso"),
				Timestamp: aws.Int64(time.Now().Unix() * 1000),
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to put log events to %s stream %s: %w", group, stream, err)
	}
	return nil
}

func extractRoleName(roleArn string) (string, error) {
	a, err := arn.Parse(roleArn)
	if err != nil {
		return "", fmt.Errorf("failed to parse role arn:%s %w", roleArn, err)
	}
	if a.Service != "iam" {
		return "", fmt.Errorf("not a valid role arn")
	}
	if strings.HasPrefix(a.Resource, "role/") {
		rs := strings.Split(a.Resource, "/")
		return rs[len(rs)-1], nil
	} else {
		return "", fmt.Errorf("not a valid role arn")
	}
}

func (d *App) verifyRole(ctx context.Context, roleArn string) error {
	roleName, err := extractRoleName(roleArn)
	if err != nil {
		return err
	}
	out, err := d.iam.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		return err
	}
	doc, err := parseIAMPolicyDocument(*out.Role.AssumeRolePolicyDocument)
	if err != nil {
		return fmt.Errorf("failed to parse IAM policy document: %w", err)
	}
	for _, st := range doc.Statement {
		if st.Principal.Service == "ecs-tasks.amazonaws.com" && st.Action == "sts:AssumeRole" {
			return nil
		}
	}
	return fmt.Errorf("executionRole %s has not a valid policy document: %w", roleName, err)
}

type iamPolicyDocument struct {
	Version   string `json:"Version"`
	Statement []struct {
		Effect    string `json:"Effect"`
		Principal struct {
			Service string `json:"Service"`
		} `json:"Principal"`
		Action string `json:"Action"`
	} `json:"Statement"`
}

func parseIAMPolicyDocument(s string) (*iamPolicyDocument, error) {
	var doc iamPolicyDocument
	s, err := url.QueryUnescape(s)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
