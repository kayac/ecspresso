package ecspresso

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/olekukonko/tablewriter"
)

type RevisionsOption struct {
	Revision    *int64  `help:"revision number to output" default:"0"`
	Output      *string `help:"output format (json, table, tsv)" default:"table" enum:"json,table,tsv"`
	DumpRunning *bool   `help:"dump the task definition which is running" default:"false"`
}

type revision struct {
	Name  string `json:"name"`
	InUse string `json:"in_use"`
}

func (rev revision) Cols() []string {
	return []string{rev.Name, rev.InUse}
}

type revisions []revision

func (revs revisions) OutputJSON(w io.Writer) error {
	for _, r := range revs {
		b, err := MarshalJSONForAPI(r)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		if err != nil {
			return err
		}
	}
	return nil
}

func (revs revisions) Header() []string {
	return []string{"Name", "In Use"}
}

func (revs revisions) OutputTSV(w io.Writer) error {
	for _, rev := range revs {
		_, err := fmt.Fprintln(w, strings.Join([]string{rev.Name, rev.InUse}, "\t"))
		if err != nil {
			return err
		}
	}
	return nil
}

func (revs revisions) OutputTable(w io.Writer) error {
	t := tablewriter.NewWriter(w)
	t.SetHeader(revs.Header())
	t.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	for _, rev := range revs {
		t.Append(rev.Cols())
	}
	t.Render()
	return nil
}

func (d *App) Revesions(ctx context.Context, opt RevisionsOption) error {
	ctx, cancel := d.Start(ctx)
	defer cancel()

	inUse, err := d.inUseRevisions(ctx)
	if err != nil {
		return err
	}
	if *opt.DumpRunning {
		for taskDef, status := range inUse {
			if status == "RUNNING task" {
				res, err := d.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
					TaskDefinition: &taskDef,
					Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
				})
				if err != nil {
					return fmt.Errorf("failed to describe task definition %s: %w", taskDef, err)
				}
				b, err := MarshalJSONForAPI(res.TaskDefinition)
				if err != nil {
					return err
				}
				_, err = os.Stdout.Write(b)
				if err != nil {
					return err
				}
				return nil
			}
		}
		return errors.New("there is no task definition which is marked as RUNNING")
	}

	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return err
	}

	if r := aws.ToInt64(opt.Revision); r > 0 {
		name := fmt.Sprintf("%s:%d", aws.ToString(td.Family), r)
		res, err := d.ecs.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &name,
			Include:        []types.TaskDefinitionField{types.TaskDefinitionFieldTags},
		})
		if err != nil {
			return fmt.Errorf("failed to describe task definition %s: %w", name, err)
		}
		b, err := MarshalJSONForAPI(res.TaskDefinition)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(b)
		return err
	}

	revs := revisions{}
	var nextToken *string
	for {
		res, err := d.ecs.ListTaskDefinitions(ctx, &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: td.Family,
			NextToken:    nextToken,
		})
		if err != nil {
			return fmt.Errorf("failed to list task definitions family %s: %w", aws.ToString(td.Family), err)
		}
		for _, a := range res.TaskDefinitionArns {
			name, err := taskDefinitionToName(a)
			if err != nil {
				continue
			}
			revs = append(revs, revision{
				Name:  name,
				InUse: inUse[name],
			})
		}
		if nextToken = res.NextToken; nextToken == nil {
			break
		}
	}
	switch aws.ToString(opt.Output) {
	case "json":
		revs.OutputJSON(os.Stdout)
	case "table":
		revs.OutputTable(os.Stdout)
	case "tsv":
		revs.OutputTSV(os.Stdout)
	}
	return nil
}
