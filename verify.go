package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/fatih/color"
	"github.com/kayac/ecspresso/dockerhub"
	"github.com/pkg/errors"
)

// VerifyOption represents options for Verify()
type VerifyOption struct {
}

type verifyResourceFunc func(context.Context) error

type verifySkipErr string

func (v verifySkipErr) Error() string {
	return string(v)
}

// Verify verifies service / task definitions related resources are valid.
func (d *App) Verify(opt VerifyOption) error {
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
	hasError := 0
	for _, r := range resources {
		if err := d.verifyResource(ctx, r.name, r.fn); err != nil {
			hasError++
			d.Log(err.Error())
		}
	}
	if hasError > 0 {
		return errors.Errorf("%d errors found", hasError)
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
	out, err := d.ecs.DescribeClustersWithContext(ctx, &ecs.DescribeClustersInput{
		Clusters: aws.StringSlice([]string{cluster}),
	})
	if err != nil {
		return err
	} else if len(out.Clusters) == 0 {
		return errors.Errorf("cluster %s is not found", cluster)
	} else {
		d.DebugLog(out.Clusters[0].GoString())
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

	// LB
	for i, lb := range sv.LoadBalancers {
		name := fmt.Sprintf("LoadBalancer[%d]", i)
		err := d.verifyResource(ctx, name, func(context.Context) error {
			out, err := d.elbv2.DescribeTargetGroupsWithContext(ctx, &elbv2.DescribeTargetGroupsInput{
				TargetGroupArns: []*string{lb.TargetGroupArn},
			})
			if err != nil {
				return err
			} else if len(out.TargetGroups) == 0 {
				return errors.Errorf("target group %s is not found", *lb.TargetGroupArn)
			}
			d.DebugLog(out.GoString())
			tgPort := aws.Int64Value(out.TargetGroups[0].Port)
			cPort := aws.Int64Value(lb.ContainerPort)
			if tgPort != cPort {
				return errors.Errorf("target group's port %d and container's port %d mismatch", tgPort, cPort)
			}

			cname := aws.StringValue(lb.ContainerName)
			var container *ecs.ContainerDefinition
			for _, c := range td.ContainerDefinitions {
				if aws.StringValue(c.Name) == cname {
					container = c
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
		name := fmt.Sprintf("ContainerDefinition[%s]", aws.StringValue(c.Name))
		err := d.verifyResource(ctx, name, func(ctx context.Context) error {
			return d.verifyContainer(ctx, c)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var (
	ecrImageURLRegex    = regexp.MustCompile(`dkr\.ecr\..+.amazonaws\.com/.*:.*`)
	dockerHubImageRegex = regexp.MustCompile(`([a-zA-Z0-9_-]+/)?[a-zA-Z0-9_-]+:.*`)
)

func (d *App) verifyECRImage(ctx context.Context, image string) error {
	rr := strings.Split(strings.SplitN(image, "/", 2)[1], ":")
	repo, tag := rr[0], rr[1]
	var nextToken *string
	for {
		out, err := d.ecr.ListImagesWithContext(ctx, &ecr.ListImagesInput{
			RepositoryName: aws.String(repo),
			Filter:         &ecr.ListImagesFilter{TagStatus: aws.String("TAGGED")},
			NextToken:      nextToken,
		})
		if err != nil {
			return err
		}
		nextToken = out.NextToken
		for _, img := range out.ImageIds {
			d.DebugLog(*img.ImageTag)
			if aws.StringValue(img.ImageTag) == tag {
				return nil
			}
		}
		if nextToken == nil {
			break
		}
	}
	return errors.Errorf("%s:%s not found", repo, tag)
}

func (d *App) verifyDockerHubImage(ctx context.Context, image string) error {
	rr := strings.Split(image, ":")
	repoName, tag := rr[0], rr[1]
	if !strings.Contains(repoName, "/") {
		repoName = "library/" + repoName
	}
	d.DebugLog(fmt.Sprintf("dockerhub repo=%s tag=%s", repoName, tag))

	repo, err := dockerhub.New(repoName)
	if err != nil {
		return err
	}
	ok, err := repo.HasImage(tag)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return errors.Errorf("%s:%s is not found in DockerHub", repoName, tag)
}

func (d *App) verifyImage(ctx context.Context, image string) error {
	if image == "" {
		return errors.New("image is not defined")
	}
	if ecrImageURLRegex.MatchString(image) {
		return d.verifyECRImage(ctx, image)
	} else if dockerHubImageRegex.MatchString(image) {
		return d.verifyDockerHubImage(ctx, image)
	}
	return verifySkipErr("not supported URL (patches are welcome!)")
}

func (d *App) verifyContainer(ctx context.Context, c *ecs.ContainerDefinition) error {
	image := aws.StringValue(c.Image)
	name := fmt.Sprintf("Image[%s]", image)
	err := d.verifyResource(ctx, name, func(ctx context.Context) error {
		return d.verifyImage(ctx, image)
	})
	if err != nil {
		return err
	}
	for _, secret := range c.Secrets {
		name := fmt.Sprintf("Secret[%s]", *secret.Name)
		err := d.verifyResource(ctx, name, func(ctx context.Context) error {
			_, err := d.ssm.GetParameterWithContext(ctx, &ssm.GetParameterInput{
				Name:           secret.ValueFrom,
				WithDecryption: aws.Bool(true),
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if c.LogConfiguration != nil && aws.StringValue(c.LogConfiguration.LogDriver) == "awslogs" {
		err := d.verifyResource(ctx, "LogConfiguration[awslogs]", func(ctx context.Context) error {
			return d.verifyLogConfiguration(ctx, c)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *App) verifyLogConfiguration(ctx context.Context, c *ecs.ContainerDefinition) error {
	options := c.LogConfiguration.Options
	group, region, prefix := options["awslogs-group"], options["awslogs-region"], options["awslogs-stream-prefix"]
	if group == nil {
		return errors.New("awslogs-group is required")
	}
	if region == nil {
		return errors.New("awslogs-region is required")
	}
	var stream string
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	if prefix != nil {
		stream = fmt.Sprintf("%s/%s/%s-%s", *prefix, *c.Name, "ecspresso-verify", suffix)
	} else {
		stream = fmt.Sprintf("%s/%s-%s", *c.Name, "ecspresso-verify", suffix)
	}

	if _, err := d.cwl.CreateLogStreamWithContext(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  group,
		LogStreamName: aws.String(stream),
	}); err != nil {
		return err
	}
	if _, err := d.cwl.PutLogEventsWithContext(ctx, &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  group,
		LogStreamName: aws.String(stream),
		LogEvents: []*cloudwatchlogs.InputLogEvent{
			{
				Message:   aws.String("This is a verify message by ecspresso"),
				Timestamp: aws.Int64(time.Now().Unix() * 1000),
			},
		},
	}); err != nil {
		return err
	}
	return nil
}

func (d *App) verifyRole(ctx context.Context, name string) error {
	rn := strings.Split(name, "/")
	if len(rn) < 2 {
		return errors.New("invalid role syntax")
	}
	roleName := rn[1]
	out, err := d.iam.GetRoleWithContext(ctx, &iam.GetRoleInput{
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
