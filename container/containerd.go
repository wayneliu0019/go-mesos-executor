package container

import (
	"context"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"go-mesos-executor/logger"
	"go.uber.org/zap"
	"net"
	"syscall"
)

type ContainerdContainerizer struct {
	Client *containerd.Client

	Image string
	Namespace string

}


func NewContainerdContainerizer(socket, image, namespace  string) (*ContainerdContainerizer, error) {
	client, err := containerd.New(socket)
	if err != nil {
		return nil, err
	}

	return &ContainerdContainerizer{Client: client, Image: image, Namespace: namespace}, nil
}


func (c *ContainerdContainerizer) ContainerCreate(info Info) (string, error){

	// create a new context with namespace
	ctx := namespaces.WithNamespace(context.Background(), c.Namespace)

	id:=info.Name

	var image containerd.Image
	var err error
	// pull the image
	if len(c.Image) > 0 {
		image, err = c.Client.Pull(ctx, c.Image, containerd.WithPullUnpack)
		if err != nil {
			logger.GetInstance().Error("pull images failed", zap.Error(err))
			return "", err
		}
	}


	//memorylimit := int64(info.MemoryLimit *1024 * 1024)
	//cpushare := info.CPUSharesLimit * 1024
	//resources := func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
	//	s.Linux.Resources.Memory = &specs.LinuxMemory{
	//		Limit: &memorylimit,
	//	}
	//	s.Linux.Resources.CPU = &specs.LinuxCPU{
	//		Shares: &cpushare,
	//	}
	//	return nil
	//}

	//container, err := c.Client.NewContainer(
	//	ctx,
	//	id,
	//	containerd.WithImage(image),
	//	containerd.WithNewSnapshot(id, image),
	//	containerd.WithNewSpec(oci.WithImageConfig(image), resources),
	//)

	containerOpts :=[]containerd.NewContainerOpts{}
	if image != nil {
		containerOpts= append(containerOpts, containerd.WithImage(image))
		containerOpts = append(containerOpts, containerd.WithNewSnapshot(id, image))
		containerOpts = append(containerOpts, containerd.WithNewSpec(oci.WithImageConfig(image)))
	}

	// create a container
	container, err := c.Client.NewContainer(
		ctx,
		id,
		containerOpts ...
		//containerd.WithImage(image),
		//containerd.WithNewSnapshot(id, image),
		//containerd.WithNewSpec(oci.WithImageConfig(image)),
	)

	if err != nil {
		logger.GetInstance().Error("create container failed ", zap.Error(err))
		return "", err
	}

	logger.GetInstance().Info("task created ", zap.String("ID", container.ID()))

	return container.ID(), nil
}

func (c *ContainerdContainerizer) ContainerRun(id string) error {

	// create a new context with namespace
	ctx := namespaces.WithNamespace(context.Background(), c.Namespace)

	container, err:= c.Client.LoadContainer(ctx, id)
	if err != nil {
		logger.GetInstance().Error("get container from id failed", zap.String("id", id), zap.Error(err))
		return err
	}

	// create a task from the container
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		logger.GetInstance().Error("create task failed ", zap.Error(err))
		return err
	}

	if err := task.Start(ctx); err != nil {
		logger.GetInstance().Error("start task failed ", zap.Error(err))
		return err
	}

	return nil
}

// ContainerWait waits for the given container to stop and returns its
// exit code. This function is blocking.
func (c *ContainerdContainerizer) ContainerWait(id string) (int, error) {

	// create a new context with namespace
	ctx := namespaces.WithNamespace(context.Background(), c.Namespace)

	container, err:= c.Client.LoadContainer(ctx, id)
	if err != nil {
		logger.GetInstance().Error("get container from id failed", zap.String("id", id), zap.Error(err))
		return -1, err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		logger.GetInstance().Error("get task from id failed", zap.String("id", id), zap.Error(err))
		return -1, err
	}

	exitStatusC, _ := task.Wait(ctx)
	status := <-exitStatusC
	code, _, err := status.Result()
	if err != nil {
		logger.GetInstance().Error("get task exit status error ", zap.Error(err))
		return -1, err
	}

	return int(code), nil
}

//stop the given container
func (c *ContainerdContainerizer) ContainerStop(id string) error {
	// create a new context with  namespace
	ctx := namespaces.WithNamespace(context.Background(), c.Namespace)

	container, err:= c.Client.LoadContainer(ctx, id)
	if err != nil {
		logger.GetInstance().Warn("get container from id failed", zap.String("id", id), zap.Error(err))
		return  nil
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		logger.GetInstance().Warn("get task from id failed", zap.String("id", id), zap.Error(err))
		return nil
	}

	logger.GetInstance().Info(fmt.Sprintf("task info is %v", task))
	taskstatus,_:=task.Status(ctx)
	if taskstatus.Status != containerd.Stopped{

		logger.GetInstance().Info(fmt.Sprintf("task %s status %v is not stopped, need to kill first", id, taskstatus.Status))

		exitStatusC, _ := task.Wait(ctx)

		// kill the task first
		if err := task.Kill(ctx, syscall.SIGKILL); err != nil {
			logger.GetInstance().Error("kill task by id failed", zap.String("id", id), zap.Error(err))
			return err
		}

		status := <-exitStatusC
		code, _, err := status.Result()
		if err != nil {
			return err
		}

		logger.GetInstance().Info("task killed with status", zap.String("id", id), zap.Int("status", int(code)))
	}
	//stopped task can be delete directly
	_, errt:=task.Delete(ctx)
	if errt != nil {
		logger.GetInstance().Error("task delete failed", zap.String("id", id), zap.Error(errt))
		return errt
	}

	logger.GetInstance().Info("task deleted ", zap.String("id", id))

	return nil
}

// ContainerRemove removes the given container
func (c *ContainerdContainerizer) ContainerRemove(id string) error {
	// create a new context with namespace
	ctx := namespaces.WithNamespace(context.Background(), c.Namespace)

	container, err:= c.Client.LoadContainer(ctx, id)
	if err != nil {
		logger.GetInstance().Warn("get container from id failed", zap.String("id", id), zap.Error(err))
		return  err
	}

	//delete container
	if err:= container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		logger.GetInstance().Error("delete container by id failed", zap.String("id", id), zap.Error(err))
		return err
	}

	logger.GetInstance().Info("container deleted ", zap.String("id", id))
	return nil
}

func (c *ContainerdContainerizer) ContainerGetPID(id string) (int, error) {
	return -1, nil
}

func (c *ContainerdContainerizer) ContainerExec(ctx context.Context, id string, cmd []string) (chan error)  {
	return nil
}

func (c *ContainerdContainerizer) ContainerGetIPsByInterface(id string, interfaceName string) ([]net.IP,  error){
	return nil, nil
}



