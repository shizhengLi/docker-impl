package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/urfave/cli/v2"
	"github.com/sirupsen/logrus"
	"docker-impl/pkg/container"
	"docker-impl/pkg/image"
	"docker-impl/pkg/store"
	"docker-impl/pkg/types"
)

type App struct {
	cliApp       *cli.App
	store        *store.Store
	imageMgr     *image.Manager
	containerMgr *container.Manager
}

func New() (*App, error) {
	store, err := store.NewStore("")
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %v", err)
	}

	imageMgr := image.NewManager(store)
	containerMgr := container.NewManager(store, imageMgr)

	app := &App{
		store:        store,
		imageMgr:     imageMgr,
		containerMgr: containerMgr,
	}

	app.cliApp = &cli.App{
		Name:    "mydocker",
		Usage:   "A simple Docker implementation",
		Version: "1.0.0",
		Commands: []*cli.Command{
			app.createImageCommands(),
			app.createContainerCommands(),
			app.createSystemCommands(),
		},
	}

	// Add cluster commands
	app.addClusterCommands()

	return app, nil
}

func (app *App) Run(args []string) error {
	return app.cliApp.Run(args)
}

func (app *App) createImageCommands() *cli.Command {
	return &cli.Command{
		Name:  "image",
		Usage: "Manage images",
		Subcommands: []*cli.Command{
			{
				Name:    "pull",
				Usage:   "Pull an image from a registry",
				Aliases: []string{"p"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "tag",
						Usage: "Image tag",
						Value: "latest",
					},
				},
				Action: app.pullImage,
			},
			{
				Name:    "list",
				Usage:   "List images",
				Aliases: []string{"ls"},
				Action:  app.listImages,
			},
			{
				Name:    "remove",
				Usage:   "Remove an image",
				Aliases: []string{"rm"},
				Action:  app.removeImage,
			},
			{
				Name:    "build",
				Usage:   "Build an image from a Dockerfile",
				Action:  app.buildImage,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "tag",
						Usage: "Name and optionally a tag in the 'name:tag' format",
					},
					&cli.StringFlag{
						Name:  "file",
						Usage: "Name of the Dockerfile",
						Value: "Dockerfile",
					},
				},
			},
		},
	}
}

func (app *App) createContainerCommands() *cli.Command {
	return &cli.Command{
		Name:  "container",
		Usage: "Manage containers",
		Subcommands: []*cli.Command{
			{
				Name:    "run",
				Usage:   "Run a command in a new container",
				Aliases: []string{"r"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "name",
						Usage: "Assign a name to the container",
					},
					&cli.StringFlag{
						Name:  "network",
						Usage: "Connect a container to a network",
						Value: "bridge",
					},
					&cli.BoolFlag{
						Name:  "interactive",
						Usage: "Keep STDIN open even if not attached",
						Aliases: []string{"i"},
					},
					&cli.BoolFlag{
						Name:  "tty",
						Usage: "Allocate a pseudo-TTY",
						Aliases: []string{"t"},
					},
					&cli.StringSliceFlag{
						Name:  "publish",
						Usage: "Publish a container's port(s) to the host",
						Aliases: []string{"p"},
					},
					&cli.StringSliceFlag{
						Name:  "volume",
						Usage: "Bind mount a volume",
						Aliases: []string{"v"},
					},
					&cli.BoolFlag{
						Name:  "detach",
						Usage: "Run container in background and print container ID",
						Aliases: []string{"d"},
					},
				},
				Action: app.runContainer,
			},
			{
				Name:    "list",
				Usage:   "List containers",
				Aliases: []string{"ls", "ps"},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "all",
						Usage: "Show all containers (default shows just running)",
						Aliases: []string{"a"},
					},
				},
				Action: app.listContainers,
			},
			{
				Name:    "start",
				Usage:   "Start one or more stopped containers",
				Action:  app.startContainer,
			},
			{
				Name:    "stop",
				Usage:   "Stop one or more running containers",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "time",
						Usage: "Seconds to wait for stop before killing it",
						Value: 10,
						Aliases: []string{"t"},
					},
				},
				Action: app.stopContainer,
			},
			{
				Name:    "remove",
				Usage:   "Remove one or more containers",
				Aliases: []string{"rm"},
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Force the removal of a running container",
						Aliases: []string{"f"},
					},
				},
				Action: app.removeContainer,
			},
			{
				Name:    "logs",
				Usage:   "Fetch the logs of a container",
				Action:  app.containerLogs,
			},
			{
				Name:    "inspect",
				Usage:   "Return low-level information on Docker objects",
				Action:  app.inspectContainer,
			},
		},
	}
}

func (app *App) createSystemCommands() *cli.Command {
	return &cli.Command{
		Name:  "system",
		Usage: "Manage mydocker system",
		Subcommands: []*cli.Command{
			{
				Name:    "info",
				Usage:   "Display system-wide information",
				Action:  app.systemInfo,
			},
			{
				Name:    "prune",
				Usage:   "Remove unused data",
				Action:  app.systemPrune,
			},
		},
	}
}

func (app *App) addClusterCommands() {
	// Add cluster commands dynamically
	clusterCmd := &cli.Command{
		Name:  "cluster",
		Usage: "Manage mydocker cluster",
		Subcommands: []*cli.Command{
			{
				Name:    "init",
				Usage:   "Initialize a new cluster",
				Action:  app.initCluster,
			},
			{
				Name:    "info",
				Usage:   "Show cluster information",
				Action:  app.clusterInfo,
			},
			{
				Name:    "status",
				Usage:   "Show cluster status",
				Action:  app.clusterStatus,
			},
		},
	}

	// Add node command group
	nodeCmd := &cli.Command{
		Name:  "node",
		Usage: "Manage cluster nodes",
		Subcommands: []*cli.Command{
			{
				Name:    "ls",
				Usage:   "List nodes in the cluster",
				Aliases: []string{"list"},
				Action:  app.listNodes,
			},
		},
	}

	// Add task command group
	taskCmd := &cli.Command{
		Name:  "task",
		Usage: "Manage cluster tasks",
		Subcommands: []*cli.Command{
			{
				Name:    "ls",
				Usage:   "List tasks",
				Aliases: []string{"list"},
				Action:  app.listTasks,
			},
		},
	}

	// Add commands to CLI app
	app.cliApp.Commands = append(app.cliApp.Commands, clusterCmd, nodeCmd, taskCmd)
}