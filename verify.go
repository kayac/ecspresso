package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogsTypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso/registry"
	"github.com/pkg/errors"
)

type verifier struct {
	cwl            *cloudwatchlogs.Client
	elbv2          []*elbv2.Client // fallback to executionRole until v1.6
	ssm            *ssm.Client
	secretsmanager *secretsmanager.Client
	ecr            *ecr.Client
	opt            *VerifyOption
	isAssumed      bool
}

func newVerifier(execCfg, appCfg aws.Config, opt *VerifyOption) *verifier {
	return &verifier{
		cwl:            cloudwatchlogs.NewFromConfig(execCfg),
		elbv2:          []*elbv2.Client{elbv2.NewFromConfig(appCfg), elbv2.NewFromConfig(execCfg)},
		ssm:            ssm.NewFromConfig(execCfg),
		secretsmanager: secretsmanager.NewFromConfig(execCfg),
		ecr:            ecr.NewFromConfig(execCfg),
		opt:            opt, isAssumed: &execCfg != &appCfg,
	}
}

func (v *verifier) existsSecretValue(ctx context.Context, from string) error {
	if !aws.ToBool(v.opt.GetSecrets) {
		return verifySkipErr(fmt.Sprintf("get a secret value for %s", from))
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
		return errors.Wrapf(err, "failed to get secret value from %s secret id %s", from, secretArn)
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
	return errors.Wrapf(err, "failed to get ssm parameter %s", name)
}

func (d *App) newAssumedVerifier(cfg aws.Config, executionRole *string, opt *VerifyOption) (*verifier, error) {
	if executionRole == nil {
		return newVerifier(cfg, cfg, opt), nil
	}
	svc := sts.NewFromConfig(d.config.awsv2Config)
	out, err := svc.AssumeRole(context.TODO(), &sts.AssumeRoleInput{
		RoleArn:         executionRole,
		RoleSessionName: aws.String("ecspresso-verifier"),
	})
	if err != nil {
		fmt.Println(
			color.CyanString("INFO: failed to assume role to taskExecutionRole. Continue to verify with current session. %s", err.Error()),
		)
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

type verifySkipErr string

func (v verifySkipErr) Error() string {
	return string(v)
}

// Verify verifies service / task definitions related resources are valid.
func (d *App) Verify(opt VerifyOption) error {
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}
	d.verifier, err = d.newAssumedVerifier(d.config.awsv2Config, td.ExecutionRoleArn, &opt)
	if err != nil {
		return err
	}

	ctx, cancel := d.Start()
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
		if _, ok := err.(verifySkipErr); ok {
			print("--> %s [%s] %s", resourceType, color.CyanString("SKIP"), color.CyanString(err.Error()))
			return nil
		}
		print("--> %s [%s] %s", resourceType, color.RedString("NG"), color.RedString(err.Error()))
		return errors.Wrapf(err, "verify %s failed", resourceType)
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
		return errors.Wrapf(err, "failed to describe cluster %s", cluster)
	} else if len(out.Clusters) == 0 {
		return errors.Errorf("cluster %s is not found", cluster)
	}
	return nil
}

func (d *App) verifyServiceDefinition(ctx context.Context) error {
	if d.config.ServiceDefinitionPath == "" {
		return verifySkipErr("no ServiceDefinition")
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
			out, err := d.verifier.elbv2[0].DescribeTargetGroups(ctx, &elbv2.DescribeTargetGroupsInput{
				TargetGroupArns: []string{*lb.TargetGroupArn},
			})
			if err != nil && d.verifier.isAssumed {
				fmt.Fprintln(
					os.Stderr,
					color.YellowString(
						"WARNING: verifying the target group using the task execution role has been DEPRECATED and will be removed in the future. "+
							"Allow `elasticloadbalancing: DescribeTargetGroups` to the role that executes ecspresso."),
				)
				out, err = d.verifier.elbv2[1].DescribeTargetGroups(ctx, &elbv2.DescribeTargetGroupsInput{
					TargetGroupArns: []string{*lb.TargetGroupArn},
				})
			}
			if err != nil {
				return err
			} else if len(out.TargetGroups) == 0 {
				return errors.Errorf("target group %s is not found", *lb.TargetGroupArn)
			}
			tgPort := aws.ToInt32(out.TargetGroups[0].Port)
			cPort := aws.ToInt32(lb.ContainerPort)
			if int32(tgPort) != cPort {
				return errors.Errorf("target group's port %d and container's port %d mismatch", tgPort, cPort)
			}

			cname := aws.ToString(lb.ContainerName)
			var container *types.ContainerDefinition
			for _, c := range td.ContainerDefinitions {
				if aws.ToString(c.Name) == cname {
					container = &c
					break
				}
			}
			if container == nil {
				return errors.Errorf("container name %s is not defined in task definition", cname)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(sv.LoadBalancers) == 0 && sv.HealthCheckGracePeriodSeconds != nil {
		return errors.Errorf("service has no load balancers, but healthCheckGracePeriodSeconds is defined.")
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
			return d.verifyContainer(ctx, &c)
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
	d.DebugLog("VERIFY ECR Image")
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
	d.DebugLog(fmt.Sprintf("image=%s tag=%s", image, tag))

	repo := registry.New(image, user, password)
	ok, err := repo.HasImage(ctx, tag)
	if err != nil {
		return err
	}
	if !ok {
		return errors.Errorf("%s:%s is not found in Registry", image, tag)
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
			return verifySkipErr(err.Error())
		}
		return err
	}
	if ok {
		return nil
	}
	return errors.Errorf("%s:%s for arch=%s os=%s is not found in Registry", image, tag, arch, os)
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

func (d *App) verifyContainer(ctx context.Context, c *types.ContainerDefinition) error {
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
	return nil
}

func (d *App) verifyLogConfiguration(ctx context.Context, c *types.ContainerDefinition) error {
	options := c.LogConfiguration.Options
	d.Log(fmt.Sprintf("LogConfiguration[awslogs] options=%v", options))
	group, region, prefix := options["awslogs-group"], options["awslogs-region"], options["awslogs-stream-prefix"]
	if group == "" {
		return errors.New("awslogs-group is required")
	}
	if region == "" {
		return errors.New("awslogs-region is required")
	}

	if !aws.ToBool(d.verifier.opt.PutLogs) {
		return verifySkipErr(fmt.Sprintf("putting logs to %s", group))
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
		return errors.Wrapf(err, "failed to create log stream %s in %s", stream, group)
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
		return errors.Wrapf(err, "failed to put log events to %s stream %s", group, stream)
	}
	return nil
}

func parseRoleArn(arn string) (roleName string, err error) {
	if !strings.HasPrefix(arn, "arn:aws:iam::") {
		return "", errors.Errorf("not a valid role arn")
	}
	rn := strings.Split(arn, "/")
	if len(rn) < 2 {
		return "", errors.New("invalid role syntax")
	}
	if !strings.HasSuffix(rn[0], ":role") {
		return "", errors.Errorf("not a valid role arn")
	}
	return rn[len(rn)-1], nil
}

func (d *App) verifyRole(ctx context.Context, arn string) error {
	roleName, err := parseRoleArn(arn)
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
		return errors.Wrap(err, "failed to parse IAM policy document")
	}
	for _, st := range doc.Statement {
		if st.Principal.Service == "ecs-tasks.amazonaws.com" && st.Action == "sts:AssumeRole" {
			return nil
		}
	}
	return errors.Errorf("executionRole %s has not a valid policy document", roleName)
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
