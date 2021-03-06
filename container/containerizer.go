package container

import (
	"context"
	"github.com/mesos/mesos-go/api/v1/lib"
	"net"
)

// Containerizer represents a containerizing technology such as docker
type Containerizer interface {
	ContainerCreate(info Info) (id string, err error)                                     // Creates the container with the given info and returns its ID
	ContainerGetPID(id string) (pid int, err error)                                       // Returns the main PID (1) of the given container
	ContainerRemove(id string) error                                                      // Removes the given container
	ContainerRun(id string) error                                                         // Starts the given container
	ContainerStop(id string) error                                                        // Stops the given container
	ContainerWait(id string) (code int, err error)                                        // Wait for the given container to stop, returning its exit code
	ContainerExec(ctx context.Context, id string, cmd []string) (result chan error)       // Executes the given command with the given context in the given container and returns result in a chan (asynchronous)
	ContainerGetIPsByInterface(id string, interfaceName string) (ips []net.IP, err error) // Returns the given container IP corresponding to a host interface
}

// Info represents container information such as image name, CPU/memory limits...
type Info struct {
	CPUSharesLimit uint64
	MemoryLimit    uint64
	Name          string
	TaskInfo       mesos.TaskInfo
}
