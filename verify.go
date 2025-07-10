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
	"github.com/aws/aws-sdk-go-v2/service/vpclattice"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso/v2/registry"
	"github.com/samber/lo"
)

type verifier struct {
	cwl            *cloudwatchlogs.Client
	ssm            *ssm.Client
	secretsmanager *secretsmanager.Client
	ecr            map[string]*ecr.Client
	s3             *s3.Client
	opt            *VerifyOption
	isAssumed      bool
	execCfg        *aws.Config
}

func (v *verifier) IsAssumed() bool {
	return v.isAssumed
}

func newVerifier(execCfg, appCfg *aws.Config, opt *VerifyOption) *verifier {
	return &verifier{
		cwl:            cloudwatchlogs.NewFromConfig(*execCfg),
		ssm:            ssm.NewFromConfig(*execCfg),
		secretsmanager: secretsmanager.NewFromConfig(*execCfg),
		ecr: map[string]*ecr.Client{
			execCfg.Region: ecr.NewFromConfig(*execCfg),
		},
		s3:        s3.NewFromConfig(*appCfg),
		opt:       opt,
		isAssumed: execCfg != appCfg,
		execCfg:   execCfg,
	}
}

func (v *verifier) ecrClient(region string) *ecr.Client {
	if c, ok := v.ecr[region]; ok {
		return c
	}
	cfg := v.execCfg.Copy()
	cfg.Region = region
	client := ecr.NewFromConfig(cfg)
	v.ecr[region] = client
	return client
}

func (v *verifier) existsSecretValue(ctx context.Context, from string) error {
	if !v.opt.GetSecrets {
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
		res, err := v.secretsmanager.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
			SecretId: &secretArn,
		})
		if err != nil {
			return fmt.Errorf("failed to get secret value from %s secret id %s: %w", from, secretArn, err)
		}
		if len(part) < 8 {
			return nil
		}
		key := part[7]
		if key == "" {
			return nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(*res.SecretString), &m); err != nil {
			return fmt.Errorf("failed to parse secret string from %s secret id %s: %w", from, secretArn, err)
		}

		if _, ok := m[key]; !ok {
			return fmt.Errorf("failed to find key %s on secret json value from %s secret id %s", key, from, secretArn)
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
	out, err := v.ssm.GetParameters(ctx, &ssm.GetParametersInput{
		Names:          []string{name},
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("failed to get ssm parameters %s: %w", name, err)
	}
	if len(out.Parameters) == 0 || len(out.InvalidParameters) > 0 {
		return fmt.Errorf("ssm parameter %s is not found", name)
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
		d.LogInfo("executionRoleArn is not set. Continue to verify with current session.")
		return newVerifier(&cfg, &cfg, opt), nil
	}
	svc := sts.NewFromConfig(d.config.awsv2Config)
	out, err := svc.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         executionRole,
		RoleSessionName: aws.String("ecspresso-verifier"),
	})
	if err != nil {
		d.LogInfo("failed to assume role to taskExecutionRole. Continue to verify with current session. %s", err.Error())
		return newVerifier(&cfg, &cfg, opt), nil
	}
	d.LogInfo("success to assume role: %s", aws.ToString(executionRole))
	ec := aws.Config{}
	ec.Region = d.config.Region
	ec.Credentials = credentials.NewStaticCredentialsProvider(
		aws.ToString(out.Credentials.AccessKeyId),
		aws.ToString(out.Credentials.SecretAccessKey),
		aws.ToString(out.Credentials.SessionToken),
	)
	return newVerifier(&ec, &cfg, opt), nil
}

// VerifyOption represents options for Verify()
type VerifyOption struct {
	GetSecrets bool `help:"get secrets from ParameterStore or SecretsManager" default:"true" negatable:""`
	PutLogs    bool `help:"put logs to CloudWatchLogs" default:"true" negatable:""`
	Cache      bool `help:"use cache" default:"true" negatable:""`
}

type verifyResourceFunc func(context.Context) error

type verifyContextKeyType string

var verifyStateKey = verifyContextKeyType("verifyState")

// Verify verifies service / task definitions related resources are valid.
func (d *App) Verify(ctx context.Context, opt VerifyOption) error {
	vs := newVerifyState(opt.Cache)
	ctx = context.WithValue(ctx, verifyStateKey, vs)

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

	d.LogInfo("Starting verify")
	resources := []struct {
		name string
		fn   verifyResourceFunc
	}{
		{name: "TaskDefinition", fn: d.verifyTaskDefinition},
		{name: "ServiceDefinition", fn: d.verifyServiceDefinition},
		{name: "Cluster", fn: d.verifyCluster},
	}
	for _, r := range resources {
		if _, err := vs.VerifyResource(ctx, r.name, r.fn); err != nil {
			return err
		}
	}
	d.LogInfo("Verify OK!")
	return nil
}

type verifyState struct {
	cache verifyCache
	level int
}

func newVerifyState(cache bool) *verifyState {
	vs := &verifyState{}
	if cache {
		vs.cache = make(verifyCache, 100)
	} else {
		vs.cache = verifyCache(nil)
	}
	vs.level = 0
	return vs
}

type verifyCache map[string]error

func (vc verifyCache) Do(ctx context.Context, name string, fn verifyResourceFunc) (error, bool) {
	if vc == nil {
		// no cache
		return fn(ctx), false
	}
	if err, ok := vc[name]; ok {
		return err, true
	}
	err := fn(ctx)
	vc[name] = err
	return err, false
}

type verifyResult struct {
	Name        string `json:"verify"`
	Result      string `json:"result"`
	Error       string `json:"error,omitempty"`
	Cached      bool   `json:"cached,omitempty"`
	indentLevel int    `json:"-"`
}

func (r *verifyResult) JSON() string {
	b, _ := json.Marshal(r)
	return string(b)
}

const (
	verifyResultOK      = "OK"
	verifyResultNG      = "NG"
	verifyResultSkip    = "SKIP"
	verifyResultUnknown = "UNKNOWN"
)

func (r *verifyResult) String() string {
	buf := &strings.Builder{}
	indent := strings.Repeat("  ", r.indentLevel)
	buf.WriteString(indent)
	buf.WriteString(r.Name)
	buf.WriteString("\n")
	buf.WriteString(indent)
	buf.WriteString("--> [")
	switch r.Result {
	case verifyResultOK:
		buf.WriteString(color.GreenString(r.Result))
	case verifyResultNG:
		buf.WriteString(color.RedString(r.Result))
	case verifyResultSkip:
		buf.WriteString(color.CyanString(r.Result))
	default:
		buf.WriteString(color.RedString(verifyResultUnknown))
	}
	buf.WriteString("]")
	if r.Cached {
		buf.WriteString(color.CyanString("(cached)"))
	}
	if r.Error != "" {
		buf.WriteString(" ")
		buf.WriteString(r.Error)
	}
	buf.WriteString("\n")
	return buf.String()
}

func (vs *verifyState) VerifyResource(ctx context.Context, name string, verifyFunc func(context.Context) error) (*verifyResult, error) {
	vs.level++
	r := &verifyResult{
		Name:        name,
		indentLevel: vs.level,
	}
	defer func() {
		vs.level--
		WriteOutput(r)
	}()
	err, hit := vs.cache.Do(ctx, name, verifyFunc)
	r.Cached = hit
	if err != nil {
		if errors.As(err, &errSkipVerify) {
			r.Error = err.Error()
			r.Result = verifyResultSkip
			return r, nil
		}
		r.Error = err.Error()
		r.Result = verifyResultNG
		return r, fmt.Errorf("verify %s failed: %w", name, err)
	}
	r.Result = verifyResultOK
	return r, nil
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
	vs := ctx.Value(verifyStateKey).(*verifyState)
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
		_, err := vs.VerifyResource(ctx, name, func(context.Context) error {
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

	for i, vc := range sv.VolumeConfigurations {
		name := fmt.Sprintf("VolumeConfigurations[%d]", i)
		_, err := vs.VerifyResource(ctx, name, func(context.Context) error {
			if ebs := vc.ManagedEBSVolume; ebs != nil {
				if len(ebs.TagSpecifications) > 1 {
					d.LogWarn("%s has more than one tag specifications. Only the first tag specification is used.", name)
				}
				roleArn := aws.ToString(ebs.RoleArn)
				if _, err := vs.VerifyResource(ctx, fmt.Sprintf("RoleArn[%s]", roleArn), func(ctx context.Context) error {
					return d.verifyRole(ctx, roleArn, "ecs.amazonaws.com")
				}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// VPC Lattice
	for i, lc := range sv.VpcLatticeConfigurations {
		name := fmt.Sprintf("VpcLatticeConfiguration[%d]", i)
		_, err := vs.VerifyResource(ctx, name, func(context.Context) error {
			roleArn := aws.ToString(lc.RoleArn)
			if _, err := vs.VerifyResource(ctx, fmt.Sprintf("RoleArn[%s]", roleArn), func(ctx context.Context) error {
				return d.verifyRole(ctx, roleArn, "ecs.amazonaws.com")
			}); err != nil {
				return err
			}

			tgArn := aws.ToString(lc.TargetGroupArn)
			if _, err := vs.VerifyResource(ctx, fmt.Sprintf("TargetGroup[%s]", tgArn), func(ctx context.Context) error {
				_, err := d.lattice.GetTargetGroup(ctx, &vpclattice.GetTargetGroupInput{
					TargetGroupIdentifier: lc.TargetGroupArn,
				})
				return err
			}); err != nil {
				return err
			}

			portName := aws.ToString(lc.PortName)
			if _, err := vs.VerifyResource(ctx, fmt.Sprintf("PortName[%s]", portName), func(ctx context.Context) error {
				if portName == "" {
					return fmt.Errorf("portName is required for vpcLatticeConfiguration")
				}
				var portMappings []types.PortMapping
				for _, cd := range td.ContainerDefinitions {
					portMappings = append(portMappings, cd.PortMappings...)
				}
				if _, found := lo.Find(portMappings, func(pm types.PortMapping) bool {
					return portName == aws.ToString(pm.Name)
				}); !found {
					return fmt.Errorf("portName %s is not found in any containerDefinitions", portName)
				}
				return nil
			}); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *App) verifyTaskDefinition(ctx context.Context) error {
	vs := ctx.Value(verifyStateKey).(*verifyState)
	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}

	if execRole := td.ExecutionRoleArn; execRole != nil {
		name := fmt.Sprintf("ExecutionRole[%s]", *execRole)
		_, err := vs.VerifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyRole(ctx, *execRole, "ecs-tasks.amazonaws.com")
		})
		if err != nil {
			return err
		}
	}
	if taskRole := td.TaskRoleArn; taskRole != nil {
		name := fmt.Sprintf("TaskRole[%s]", *taskRole)
		_, err := vs.VerifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyRole(ctx, *taskRole, "ecs-tasks.amazonaws.com")
		})
		if err != nil {
			return err
		}
	}

	for _, c := range td.ContainerDefinitions {
		name := fmt.Sprintf("ContainerDefinition[%s]", aws.ToString(c.Name))
		_, err := vs.VerifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyContainer(ctx, &c, td)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	// e.g. {aws_account_id}.dkr.ecr.{region}.amazonaws.com/amazonlinux:latest
	ecrImageURLRegex = regexp.MustCompile(`^([0-9]+)\.dkr\.ecr\.([0-9a-zA-Z-]+)\.amazonaws\.com(?:\.cn)?\/.*`)
)

func (d *App) verifyECRImage(ctx context.Context, image, region string) error {
	out, err := d.verifier.ecrClient(region).GetAuthorizationToken(
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
	d.LogDebug("image=%s tag=%s", image, tag)

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
	if m := ecrImageURLRegex.FindStringSubmatch(image); len(m) == 3 {
		d.LogDebug("VERIFY ECR Image %s in region %s", image, m[2])
		return d.verifyECRImage(ctx, image, m[2]) // m[1] is aws account id, m[2] is region
	}
	d.LogDebug("VERIFY Registry Image %s", image)
	return d.verifyRegistryImage(ctx, image, "", "")
}

func (d *App) verifyContainer(ctx context.Context, c *types.ContainerDefinition, td *TaskDefinitionInput) error {
	vs := ctx.Value(verifyStateKey).(*verifyState)
	image := aws.ToString(c.Image)
	name := fmt.Sprintf("Image[%s]", image)
	_, err := vs.VerifyResource(ctx, name, func(ctx context.Context) error {
		return d.verifyImage(ctx, image)
	})
	if err != nil {
		return err
	}
	for i, secret := range c.Secrets {
		name := aws.ToString(secret.Name)
		if name == "" {
			return fmt.Errorf("secrets[%d] name is missing", i)
		}
		valueFrom := aws.ToString(secret.ValueFrom)
		if valueFrom == "" {
			return fmt.Errorf("secrets[%d] %s valueFrom is missing", i, name)
		}
		if _, err := vs.VerifyResource(ctx, fmt.Sprintf("Secret %s[%s]", name, valueFrom), func(ctx context.Context) error {
			return d.verifier.existsSecretValue(ctx, *secret.ValueFrom)
		}); err != nil {
			return err
		}
	}
	if c.LogConfiguration != nil && c.LogConfiguration.LogDriver == types.LogDriverAwslogs {
		name := fmt.Sprintf("LogConfiguration[%s]", map2str(c.LogConfiguration.Options))
		_, err := vs.VerifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyLogConfiguration(ctx, c)
		})
		if err != nil {
			return err
		}
	}
	for _, envFile := range c.EnvironmentFiles {
		name := fmt.Sprintf("EnvironmentFile[%s %s]", envFile.Type, aws.ToString(envFile.Value))
		if _, err := vs.VerifyResource(ctx, name, func(ctx context.Context) error {
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
	d.LogDebug("LogConfiguration[awslogs] options=%v", options)
	group, region, prefix := options["awslogs-group"], options["awslogs-region"], options["awslogs-stream-prefix"]
	if group == "" {
		return errors.New("awslogs-group is required")
	}
	if region == "" {
		return errors.New("awslogs-region is required")
	}

	if !d.verifier.opt.PutLogs {
		return ErrSkipVerify(fmt.Sprintf("putting logs to %s", group))
	}

	if options["awslogs-create-group"] == "true" {
		if _, err := d.verifier.cwl.CreateLogGroup(ctx, &cloudwatchlogs.CreateLogGroupInput{
			LogGroupName: &group,
		}); err != nil {
			var ex *cloudwatchlogsTypes.ResourceAlreadyExistsException
			if errors.As(err, &ex) {
				// ignore error if log group already exists
				d.LogDebug("log group %s already exists, ignored", group)
			} else if d.verifier.IsAssumed() {
				return fmt.Errorf("failed to create log group %s: %w", group, err)
			} else {
				d.LogWarn("failed to create log group %s: %s", group, err)
			}
		} else {
			d.LogInfo("created log group %s", group)
		}
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

func (d *App) verifyRole(ctx context.Context, roleArn, principalService string) error {
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
		if st.Principal.Service == principalService && st.Action == "sts:AssumeRole" {
			return nil
		}
	}
	return fmt.Errorf("role %s has not a valid policy document", roleName)
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
