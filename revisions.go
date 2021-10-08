package ecspresso

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
)

type RevisionsOption struct {
	Revision *int64
	Output   *string
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
	b, err := MarshalJSON(revs)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
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

func (d *App) Revesions(opt RevisionsOption) error {
	ctx, cancel := d.Start()
	defer cancel()

	inUse, err := d.inUseRevisions(ctx)
	if err != nil {
		return err
	}

	td, err := d.LoadTaskDefinition(d.config.TaskDefinitionPath)
	if err != nil {
		return errors.Wrap(err, "failed to load task definition")
	}

	if r := aws.Int64Value(opt.Revision); r > 0 {
		name := fmt.Sprintf("%s:%d", aws.StringValue(td.Family), r)
		res, err := d.ecs.DescribeTaskDefinitionWithContext(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &name,
			Include:        []*string{aws.String("TAGS")},
		})
		if err != nil {
			return errors.Wrap(err, "failed to describe task definition")
		}
		b, err := MarshalJSON(res.TaskDefinition)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(b)
		return err
	}

	revs := revisions{}
	var nextToken *string
	for {
		res, err := d.ecs.ListTaskDefinitionsWithContext(ctx, &ecs.ListTaskDefinitionsInput{
			FamilyPrefix: td.Family,
			NextToken:    nextToken,
		})
		if err != nil {
			return errors.Wrap(err, "failed to list task definitions")
		}
		for _, a := range res.TaskDefinitionArns {
			name, err := taskDefinitionToName(*a)
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
	switch aws.StringValue(opt.Output) {
	case "json":
		revs.OutputJSON(os.Stdout)
	case "table":
		revs.OutputTable(os.Stdout)
	case "tsv":
		revs.OutputTSV(os.Stdout)
	}
	return nil
}
