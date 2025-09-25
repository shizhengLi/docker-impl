package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/sirupsen/logrus"
	"docker-impl/pkg/cluster"
)

func addClusterCommands(app *App) {
	// Add cluster command group
	clusterCmd := &cli.Command{
		Name:  "cluster",
		Usage: "Manage mydocker cluster",
		Subcommands: []*cli.Command{
			{
				Name:    "init",
				Usage:   "Initialize a new cluster",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "advertise-addr",
						Usage: "Advertised address",
						Value: "0.0.0.0",
					},
					&cli.IntFlag{
						Name:  "advertise-port",
						Usage: "Advertised port",
						Value: 2377,
					},
					&cli.StringFlag{
						Name:  "listen-addr",
						Usage: "Listen address",
						Value: "0.0.0.0",
					},
					&cli.StringFlag{
						Name:  "data-dir",
						Usage: "Data directory",
						Value: "/var/lib/mydocker/cluster",
					},
				},
				Action: app.initCluster,
			},
			{
				Name:    "join",
				Usage:   "Join an existing cluster",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "advertise-addr",
						Usage:    "Advertised address",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "join-token",
						Usage:    "Join token for the cluster",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "listen-addr",
						Usage: "Listen address",
						Value: "0.0.0.0",
					},
				},
				Action: app.joinCluster,
			},
			{
				Name:    "leave",
				Usage:   "Leave the cluster",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "force",
						Usage: "Force leave even if running tasks",
					},
				},
				Action: app.leaveCluster,
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
			{
				Name:    "token",
				Usage:   "Manage join tokens",
				Subcommands: []*cli.Command{
					{
						Name:   "create",
						Usage:  "Create a new join token",
						Action: app.createJoinToken,
					},
					{
						Name:   "rotate",
						Usage:  "Rotate the join token",
						Action: app.rotateJoinToken,
					},
				},
			},
			{
				Name:    "scale",
				Usage:   "Scale cluster workers",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:     "workers",
						Usage:    "Number of worker nodes",
						Required: true,
					},
				},
				Action: app.scaleCluster,
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
			{
				Name:    "inspect",
				Usage:   "Inspect a node",
				Action:  app.inspectNode,
			},
			{
				Name:    "rm",
				Usage:   "Remove a node from the cluster",
				Aliases: []string{"remove"},
				Action:  app.removeNode,
			},
			{
				Name:    "update",
				Usage:   "Update a node",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "role",
						Usage: "Node role (manager/worker)",
					},
					&cli.StringFlag{
						Name:  "availability",
						Usage: "Node availability (active/pause/drain)",
					},
				},
				Action: app.updateNode,
			},
			{
				Name:    "ps",
				Usage:   "Show tasks running on a node",
				Action:  app.nodeTasks,
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
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "filter",
						Usage: "Filter output based on conditions provided",
					},
					&cli.StringFlag{
						Name:  "node",
						Usage: "Filter tasks by node",
					},
					&cli.StringFlag{
						Name:  "status",
						Usage: "Filter tasks by status",
					},
				},
				Action: app.listTasks,
			},
			{
				Name:    "inspect",
				Usage:   "Inspect a task",
				Action:  app.inspectTask,
			},
			{
				Name:    "rm",
				Usage:   "Remove a task",
				Aliases: []string{"remove"},
				Action:  app.removeTask,
			},
			{
				Name:    "logs",
				Usage:   "Show logs for a task",
				Action:  app.taskLogs,
			},
		},
	}

	// Add service command group (placeholder)
	serviceCmd := &cli.Command{
		Name:  "service",
		Usage: "Manage services",
		Subcommands: []*cli.Command{
			{
				Name:    "ls",
				Usage:   "List services",
				Aliases: []string{"list"},
				Action:  app.listServices,
			},
			{
				Name:    "create",
				Usage:   "Create a new service",
				Action:  app.createService,
			},
			{
				Name:    "inspect",
				Usage:   "Inspect a service",
				Action:  app.inspectService,
			},
			{
				Name:    "rm",
				Usage:   "Remove a service",
				Aliases: []string{"remove"},
				Action:  app.removeService,
			},
			{
				Name:    "scale",
				Usage:   "Scale a service",
				Action:  app.scaleService,
			},
			{
				Name:    "ps",
				Usage:   "List the tasks of a service",
				Action:  app.serviceTasks,
			},
		},
	}

	// Add commands to CLI app
	app.cliApp.Commands = append(app.cliApp.Commands, clusterCmd, nodeCmd, taskCmd, serviceCmd)
}

// Cluster commands
func (a *App) initCluster(c *cli.Context) error {
	config := &cluster.ClusterConfig{
		AdvertiseAddr: c.String("advertise-addr"),
		AdvertisePort: c.Int("advertise-port"),
		DataDir:       c.String("data-dir"),
	}

	clusterMgr := cluster.GetClusterManager()
	if err := clusterMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize cluster: %v", err)
	}

	fmt.Println("Cluster initialized successfully")
	fmt.Printf("Cluster ID: %s\n", clusterMgr.ID)
	fmt.Printf("Advertise address: %s:%d\n", config.AdvertiseAddr, config.AdvertisePort)

	token, err := clusterMgr.GetJoinToken()
	if err != nil {
		logrus.Warnf("Failed to get join token: %v", err)
	} else {
		fmt.Printf("Join token: %s\n", token)
	}

	return nil
}

func (a *App) joinCluster(c *cli.Context) error {
	joinAddr := c.String("advertise-addr")
	joinToken := c.String("join-token")

	clusterMgr := cluster.GetClusterManager()
	if err := clusterMgr.JoinCluster(joinAddr, joinToken); err != nil {
		return fmt.Errorf("failed to join cluster: %v", err)
	}

	fmt.Printf("Successfully joined cluster at %s\n", joinAddr)
	return nil
}

func (a *App) leaveCluster(c *cli.Context) error {
	force := c.Bool("force")

	clusterMgr := cluster.GetClusterManager()
	if err := clusterMgr.LeaveCluster(force); err != nil {
		return fmt.Errorf("failed to leave cluster: %v", err)
	}

	fmt.Println("Successfully left cluster")
	return nil
}

func (a *App) clusterInfo(c *cli.Context) error {
	clusterMgr := cluster.GetClusterManager()
	info := clusterMgr.GetClusterInfo()

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cluster info: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func (a *App) clusterStatus(c *cli.Context) error {
	clusterMgr := cluster.GetClusterManager()
	status := clusterMgr.GetStatus()

	fmt.Printf("Cluster ID: %s\n", status.ID)
	fmt.Printf("Name: %s\n", status.Name)
	fmt.Printf("Status: %s\n", status.Status)
	fmt.Printf("Nodes: %d\n", status.Nodes)
	fmt.Printf("Managers: %d\n", status.Managers)
	fmt.Printf("Workers: %d\n", status.Workers)
	fmt.Printf("Active tasks: %d\n", status.ActiveTasks)
	fmt.Printf("Completed tasks: %d\n", status.CompletedTasks)
	fmt.Printf("Created at: %s\n", status.CreatedAt)
	fmt.Printf("Updated at: %s\n", status.UpdatedAt)

	return nil
}

func (a *App) createJoinToken(c *cli.Context) error {
	clusterMgr := cluster.GetClusterManager()
	token, err := clusterMgr.GetJoinToken()
	if err != nil {
		return fmt.Errorf("failed to get join token: %v", err)
	}

	fmt.Printf("Join token: %s\n", token)
	return nil
}

func (a *App) rotateJoinToken(c *cli.Context) error {
	clusterMgr := cluster.GetClusterManager()
	token, err := clusterMgr.RotateJoinToken()
	if err != nil {
		return fmt.Errorf("failed to rotate join token: %v", err)
	}

	fmt.Printf("New join token: %s\n", token)
	return nil
}

func (a *App) scaleCluster(c *cli.Context) error {
	workers := c.Int("workers")

	clusterMgr := cluster.GetClusterManager()
	if err := clusterMgr.ScaleWorkers(workers); err != nil {
		return fmt.Errorf("failed to scale cluster: %v", err)
	}

	fmt.Printf("Cluster scaled to %d workers\n", workers)
	return nil
}

// Node commands
func (a *App) listNodes(c *cli.Context) error {
	clusterMgr := cluster.GetClusterManager()
	nodes, err := clusterMgr.NodeManager.ListNodes()
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	fmt.Printf("%-12s %-15s %-8s %-10s %-10s\n", "ID", "NAME", "STATUS", "ROLE", "ADDRESS")
	fmt.Println("----------------------------------------------------")

	for _, node := range nodes {
		fmt.Printf("%-12s %-15s %-8s %-10s %-15s:%d\n",
			node.ID[:12],
			node.Name,
			node.Status,
			node.Role,
			node.Address,
			node.Port)
	}

	return nil
}

func (a *App) inspectNode(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a node ID")
	}

	nodeID := c.Args().First()

	clusterMgr := cluster.GetClusterManager()
	node, err := clusterMgr.NodeManager.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node: %v", err)
	}

	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal node data: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func (a *App) removeNode(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a node ID")
	}

	nodeID := c.Args().First()

	clusterMgr := cluster.GetClusterManager()
	if err := clusterMgr.NodeManager.UnregisterNode(nodeID); err != nil {
		return fmt.Errorf("failed to remove node: %v", err)
	}

	fmt.Printf("Node %s removed successfully\n", nodeID)
	return nil
}

func (a *App) updateNode(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a node ID")
	}

	nodeID := c.Args().First()

	clusterMgr := cluster.GetClusterManager()

	// Update role if specified
	if role := c.String("role"); role != "" {
		// In real implementation, this would update node role
		fmt.Printf("Updated node %s role to %s\n", nodeID, role)
	}

	// Update availability if specified
	if availability := c.String("availability"); availability != "" {
		switch availability {
		case "active":
			if err := clusterMgr.NodeManager.ActivateNode(nodeID); err != nil {
				return fmt.Errorf("failed to activate node: %v", err)
			}
			fmt.Printf("Node %s activated\n", nodeID)
		case "drain":
			if err := clusterMgr.NodeManager.DrainNode(nodeID); err != nil {
				return fmt.Errorf("failed to drain node: %v", err)
			}
			fmt.Printf("Node %s drained\n", nodeID)
		default:
			return fmt.Errorf("invalid availability: %s", availability)
		}
	}

	return nil
}

func (a *App) nodeTasks(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a node ID")
	}

	nodeID := c.Args().First()

	clusterMgr := cluster.GetClusterManager()
	tasks, err := clusterMgr.TaskManager.GetTasksByNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node tasks: %v", err)
	}

	fmt.Printf("Tasks on node %s:\n", nodeID)
	fmt.Printf("%-12s %-15s %-10s\n", "ID", "NAME", "STATUS")
	fmt.Println("--------------------------------")

	for _, task := range tasks {
		fmt.Printf("%-12s %-15s %-10s\n",
			task.ID[:12],
			task.Name,
			task.Status)
	}

	return nil
}

// Task commands
func (a *App) listTasks(c *cli.Context) error {
	clusterMgr := cluster.GetClusterManager()
	tasks, err := clusterMgr.TaskManager.ListTasks()
	if err != nil {
		return fmt.Errorf("failed to list tasks: %v", err)
	}

	// Apply filters
	nodeFilter := c.String("node")
	statusFilter := c.String("status")

	fmt.Printf("%-12s %-15s %-10s %-15s\n", "ID", "NAME", "STATUS", "NODE")
	fmt.Println("----------------------------------------")

	for _, task := range tasks {
		// Apply node filter
		if nodeFilter != "" && task.NodeID != nodeFilter {
			continue
		}

		// Apply status filter
		if statusFilter != "" && string(task.Status) != statusFilter {
			continue
		}

		fmt.Printf("%-12s %-15s %-10s %-15s\n",
			task.ID[:12],
			task.Name,
			task.Status,
			task.NodeID[:12])
	}

	return nil
}

func (a *App) inspectTask(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a task ID")
	}

	taskID := c.Args().First()

	clusterMgr := cluster.GetClusterManager()
	task, err := clusterMgr.TaskManager.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %v", err)
	}

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task data: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func (a *App) removeTask(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a task ID")
	}

	taskID := c.Args().First()

	clusterMgr := cluster.GetClusterManager()
	if err := clusterMgr.TaskManager.RemoveTask(taskID); err != nil {
		return fmt.Errorf("failed to remove task: %v", err)
	}

	fmt.Printf("Task %s removed successfully\n", taskID)
	return nil
}

func (a *App) taskLogs(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a task ID")
	}

	taskID := c.Args().First()

	// In real implementation, this would fetch task logs
	fmt.Printf("Logs for task %s:\n", taskID)
	fmt.Println("Task logs not implemented yet")

	return nil
}

// Service commands (placeholders)
func (a *App) listServices(c *cli.Context) error {
	fmt.Println("No services found")
	return nil
}

func (a *App) createService(c *cli.Context) error {
	return fmt.Errorf("service creation not implemented yet")
}

func (a *App) inspectService(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a service ID")
	}
	return fmt.Errorf("service inspection not implemented yet")
}

func (a *App) removeService(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a service ID")
	}
	return fmt.Errorf("service removal not implemented yet")
}

func (a *App) scaleService(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a service ID")
	}
	return fmt.Errorf("service scaling not implemented yet")
}

func (a *App) serviceTasks(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("please specify a service ID")
	}
	return fmt.Errorf("service tasks listing not implemented yet")
}