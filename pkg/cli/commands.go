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

	return app, nil
}

func (a *App) Run(args []string) error {
	return a.cliApp.Run(args)
}

func (a *App) createImageCommands() *cli.Command {
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
				Action: a.pullImage,
			},
			{
				Name:    "list",
				Usage:   "List images",
				Aliases: []string{"ls"},
				Action:  a.listImages,
			},
			{
				Name:    "remove",
				Usage:   "Remove an image",
				Aliases: []string{"rm"},
				Action:  a.removeImage,
			},
			{
				Name:    "build",
				Usage:   "Build an image from a Dockerfile",
				Action:  a.buildImage,
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

func (a *App) createContainerCommands() *cli.Command {
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
				Action: a.runContainer,
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
				Action: a.listContainers,
			},
			{
				Name:    "start",
				Usage:   "Start one or more stopped containers",
				Action:  a.startContainer,
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
				Action: a.stopContainer,
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
				Action: a.removeContainer,
			},
			{
				Name:    "logs",
				Usage:   "Fetch the logs of a container",
				Action:  a.containerLogs,
			},
			{
				Name:    "inspect",
				Usage:   "Return low-level information on Docker objects",
				Action:  a.inspectContainer,
			},
		},
	}
}

func (a *App) createSystemCommands() *cli.Command {
	return &cli.Command{
		Name:  "system",
		Usage: "Manage mydocker system",
		Subcommands: []*cli.Command{
			{
				Name:    "info",
				Usage:   "Display system-wide information",
				Action:  a.systemInfo,
			},
			{
				Name:    "prune",
				Usage:   "Remove unused data",
				Action:  a.systemPrune,
			},
		},
	}
}

func (a *App) pullImage(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify an image name")
	}

	imageName := c.Args().First()
	tag := c.String("tag")

	logrus.Infof("Pulling image %s:%s", imageName, tag)
	image, err := a.imageMgr.PullImage(imageName, tag)
	if err != nil {
		return fmt.Errorf("failed to pull image: %v", err)
	}

	fmt.Printf("Successfully pulled image %s:%s\n", imageName, tag)
	fmt.Printf("Image ID: %s\n", image.ID)
	return nil
}

func (a *App) listImages(c *cli.Context) error {
	images, err := a.imageMgr.ListImages()
	if err != nil {
		return fmt.Errorf("failed to list images: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "REPOSITORY\tTAG\tIMAGE ID\tCREATED\tSIZE")

	for _, img := range images {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			img.Name,
			img.Tag,
			img.ID[:12],
			img.CreatedAt.Format("2006-01-02 15:04:05"),
			img.Size)
	}

	w.Flush()
	return nil
}

func (a *App) removeImage(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify an image ID or name")
	}

	imageID := c.Args().First()

	if err := a.imageMgr.RemoveImage(imageID); err != nil {
		return fmt.Errorf("failed to remove image: %v", err)
	}

	fmt.Printf("Successfully removed image %s\n", imageID)
	return nil
}

func (a *App) buildImage(c *cli.Context) error {
	contextDir := "."
	if c.Args().Len() > 0 {
		contextDir = c.Args().First()
	}

	options := types.ImageBuildOptions{
		ContextDir: contextDir,
		Dockerfile: c.String("file"),
	}

	if tag := c.String("tag"); tag != "" {
		options.Tags = []string{tag}
	}

	logrus.Infof("Building image in context %s", contextDir)
	image, err := a.imageMgr.BuildImage(options)
	if err != nil {
		return fmt.Errorf("failed to build image: %v", err)
	}

	fmt.Printf("Successfully built image %s\n", image.ID)
	return nil
}

func (a *App) runContainer(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify an image name")
	}

	imageName := c.Args().First()
	cmd := c.Args().Tail()

	config := types.ContainerConfig{
		Image:        imageName,
		Env:          []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		Cmd:          cmd,
		WorkingDir:   "/",
		AttachStdin:  c.Bool("interactive"),
		AttachStdout: true,
		AttachStderr: true,
		Tty:          c.Bool("tty"),
		OpenStdin:    c.Bool("interactive"),
	}

	hostConfig := types.HostConfig{
		NetworkMode: c.String("network"),
	}

	options := types.ContainerCreateOptions{
		Name:       c.String("name"),
		Config:     config,
		HostConfig: hostConfig,
	}

	logrus.Infof("Creating container with image %s", imageName)
	container, err := a.containerMgr.CreateContainer(options)
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	if err := a.containerMgr.StartContainer(container.ID); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}

	fmt.Printf("Container started successfully: %s\n", container.ID[:12])
	return nil
}

func (a *App) listContainers(c *cli.Context) error {
	options := types.ContainerListOptions{
		All: c.Bool("all"),
	}

	containers, err := a.containerMgr.ListContainers(options)
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CONTAINER ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS\tNAMES")

	for _, ctr := range containers {
		cmdStr := ""
		if len(ctr.Config.Cmd) > 0 {
			cmdStr = ctr.Config.Cmd[0]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			ctr.ID[:12],
			ctr.Image,
			cmdStr,
			ctr.CreatedAt.Format("2006-01-02 15:04:05"),
			ctr.Status,
			"",
			ctr.Name)
	}

	w.Flush()
	return nil
}

func (a *App) startContainer(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a container ID")
	}

	containerID := c.Args().First()

	if err := a.containerMgr.StartContainer(containerID); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}

	fmt.Printf("Container %s started successfully\n", containerID)
	return nil
}

func (a *App) stopContainer(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a container ID")
	}

	containerID := c.Args().First()
	timeout := c.Int("time")

	if err := a.containerMgr.StopContainer(containerID, timeout); err != nil {
		return fmt.Errorf("failed to stop container: %v", err)
	}

	fmt.Printf("Container %s stopped successfully\n", containerID)
	return nil
}

func (a *App) removeContainer(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a container ID")
	}

	containerID := c.Args().First()
	options := types.ContainerRemoveOptions{
		Force: c.Bool("force"),
	}

	if err := a.containerMgr.RemoveContainer(containerID, options); err != nil {
		return fmt.Errorf("failed to remove container: %v", err)
	}

	fmt.Printf("Container %s removed successfully\n", containerID)
	return nil
}

func (a *App) containerLogs(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a container ID")
	}

	containerID := c.Args().First()

	logs, err := a.containerMgr.GetContainerLogs(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %v", err)
	}

	fmt.Print(logs)
	return nil
}

func (a *App) inspectContainer(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a container ID")
	}

	containerID := c.Args().First()

	container, err := a.containerMgr.GetContainer(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container: %v", err)
	}

	data, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal container data: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func (a *App) systemInfo(c *cli.Context) error {
	info := map[string]interface{}{
		"version":      "1.0.0",
		"data_dir":     a.store.GetDataDir(),
		"storage_driver": "overlay2",
		"kernel_version": "linux",
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal system info: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func (a *App) systemPrune(c *cli.Context) error {
	fmt.Println("System prune not implemented yet")
	return nil
}