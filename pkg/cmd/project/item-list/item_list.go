package itemlist

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/tableprinter"
	"github.com/cli/cli/v2/pkg/cmd/project/shared/client"
	"github.com/cli/cli/v2/pkg/cmd/project/shared/format"
	"github.com/cli/cli/v2/pkg/cmd/project/shared/queries"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type listOpts struct {
	limit  int
	login  string
	number int32
	format string
}

type listConfig struct {
	io     *iostreams.IOStreams
	tp     *tableprinter.TablePrinter
	client *queries.Client
	opts   listOpts
}

func NewCmdList(f *cmdutil.Factory, runF func(config listConfig) error) *cobra.Command {
	opts := listOpts{}
	listCmd := &cobra.Command{
		Short: "List the items in a project",
		Use:   "item-list [<number>]",
		Example: heredoc.Doc(`
			# list the items in the current users's project "1"
			gh project item-list 1 --login "@me"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := client.New(f)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				num, err := strconv.ParseInt(args[0], 10, 32)
				if err != nil {
					return cmdutil.FlagErrorf("invalid number: %v", args[0])
				}
				opts.number = int32(num)
			}

			t := tableprinter.New(f.IOStreams)
			config := listConfig{
				io:     f.IOStreams,
				tp:     t,
				client: client,
				opts:   opts,
			}

			// allow testing of the command without actually running it
			if runF != nil {
				return runF(config)
			}
			return runList(config)
		},
	}

	listCmd.Flags().StringVar(&opts.login, "login", "", "Login of the owner. Use \"@me\" for the current user.")
	cmdutil.StringEnumFlag(listCmd, &opts.format, "format", "", "", []string{"json"}, "Output format")
	listCmd.Flags().IntVarP(&opts.limit, "limit", "L", queries.LimitDefault, "Maximum number of items to fetch")

	return listCmd
}

func runList(config listConfig) error {
	canPrompt := config.io.CanPrompt()
	owner, err := config.client.NewOwner(canPrompt, config.opts.login)
	if err != nil {
		return err
	}

	// no need to fetch the project if we already have the number
	if config.opts.number == 0 {
		project, err := config.client.NewProject(canPrompt, owner, config.opts.number, false)
		if err != nil {
			return err
		}
		config.opts.number = project.Number
	}

	project, err := config.client.ProjectItems(owner, config.opts.number, config.opts.limit)
	if err != nil {
		return err
	}

	if config.opts.format == "json" {
		return printJSON(config, project)
	}

	return printResults(config, project.Items.Nodes, owner.Login)
}

func printResults(config listConfig, items []queries.ProjectItem, login string) error {
	if len(items) == 0 {
		return cmdutil.NewNoResultsError(fmt.Sprintf("Project %d for login %s has no items", config.opts.number, login))
	}

	config.tp.HeaderRow("Type", "Title", "Number", "Repository", "ID")

	for _, i := range items {
		config.tp.AddField(i.Type())
		config.tp.AddField(i.Title())
		if i.Number() == 0 {
			config.tp.AddField("")
		} else {
			config.tp.AddField(strconv.Itoa(i.Number()))
		}
		config.tp.AddField(i.Repo())
		config.tp.AddField(i.ID())
		config.tp.EndRow()
	}

	return config.tp.Render()
}

func printJSON(config listConfig, project *queries.Project) error {
	b, err := format.JSONProjectDetailedItems(project)
	if err != nil {
		return err
	}
	_, err = config.io.Out.Write(b)
	return err
}
