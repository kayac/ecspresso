package ecspresso

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/samber/lo"
)

func arnToName(s string) string {
	ns := strings.Split(s, "/")
	return ns[len(ns)-1]
}

func isLongArnFormat(a string) (bool, error) {
	an, err := arn.Parse(a)
	if err != nil {
		return false, err
	}
	rs := strings.Split(an.Resource, "/")
	switch rs[0] {
	case "container-instance", "service", "task":
		return len(rs) >= 3, nil
	default:
		return false, nil
	}
}

func (d *App) readDefinitionFile(path string) ([]byte, error) {
	switch filepath.Ext(path) {
	case jsonnetExt:
		jsonStr, err := d.loader.VM.EvaluateFile(path)
		if err != nil {
			return nil, err
		}
		return d.loader.ReadWithEnvBytes([]byte(jsonStr))
	}
	return d.loader.ReadWithEnv(path)
}

func parseTags(s string) ([]types.Tag, error) {
	tags := make([]types.Tag, 0)
	if s == "" {
		return tags, nil
	}

	tagsStr := strings.Split(s, ",")
	for _, tag := range tagsStr {
		if tag == "" {
			continue
		}
		pair := strings.SplitN(tag, "=", 2)
		if len(pair) != 2 {
			return tags, fmt.Errorf("invalid tag format. Key=Value is required: %s", tag)
		}
		if len(pair[0]) == 0 {
			return tags, fmt.Errorf("tag Key is required")
		}
		tags = append(tags, types.Tag{
			Key:   aws.String(pair[0]),
			Value: aws.String(pair[1]),
		})
	}
	return tags, nil
}

func map2str(m map[string]string) string {
	var p []string
	keys := lo.Keys(m)
	sort.Strings(keys)
	for _, k := range keys {
		p = append(p, fmt.Sprintf("%s=%s", k, m[k]))
	}
	return strings.Join(p, ",")
}

func CompareTags(oldTags, newTags []types.Tag) (added, updated, deleted []types.Tag) {
	oldTagMap := make(map[string]string)
	newTagMap := make(map[string]string)

	for _, t := range oldTags {
		oldTagMap[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	for _, t := range newTags {
		newTagMap[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}

	for k, v := range oldTagMap {
		if newVal, ok := newTagMap[k]; ok {
			if v != newVal {
				updated = append(updated, types.Tag{Key: aws.String(k), Value: aws.String(newVal)})
			}
			delete(newTagMap, k)
		} else {
			deleted = append(deleted, types.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
	}

	for k, v := range newTagMap {
		added = append(added, types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}

	return
}

func serviceVolumeConfigurationsToTask(vcs []types.ServiceVolumeConfiguration, deleteOnTermination *bool) []types.TaskVolumeConfiguration {
	var tvc []types.TaskVolumeConfiguration
	for _, vc := range vcs {
		v := vc.ManagedEBSVolume
		if v == nil {
			continue
		}
		tagSpecs := lo.Filter(v.TagSpecifications, func(t types.EBSTagSpecification, _ int) bool {
			// PropagateTagsService is not supported in RunTask
			return t.PropagateTags != types.PropagateTagsService
		})
		tvc = append(tvc, types.TaskVolumeConfiguration{
			Name: vc.Name,
			ManagedEBSVolume: &types.TaskManagedEBSVolumeConfiguration{
				RoleArn:           v.RoleArn,
				Encrypted:         v.Encrypted,
				FilesystemType:    v.FilesystemType,
				Iops:              v.Iops,
				KmsKeyId:          v.KmsKeyId,
				SizeInGiB:         v.SizeInGiB,
				SnapshotId:        v.SnapshotId,
				TagSpecifications: tagSpecs,
				Throughput:        v.Throughput,
				VolumeType:        v.VolumeType,
				TerminationPolicy: &types.TaskManagedEBSVolumeTerminationPolicy{
					DeleteOnTermination: deleteOnTermination,
				},
			},
		})
	}
	return tvc
}
