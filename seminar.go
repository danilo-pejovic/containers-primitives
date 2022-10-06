package main
import (
 "fmt"
 "os"
 "os/exec"
 "path/filepath"
 "syscall"
 "time"
 "net"
 "strconv"
 "io/ioutil"
)


func main() {
switch os.Args[1] {
	case "run":
		run()
	case "child":
		child()
	default:
		panic("help")
	}
}

func pivotRoot(newroot string) error {
	putold := filepath.Join(newroot, "/putold")

	// https://unix.stackexchange.com/a/513417 
	// due to requirment that new root must not be on same filesystem as current root
	// to do this we have to do a slight workaround - which to be completely honest I dont understand the reasoning behind
	// but we are allowed to bind-mount new root to itself and then pivot the root to that directory
	must(syscall.Mount(newroot, newroot, "", syscall.MS_BIND|syscall.MS_REC, ""))

	// this is where we will put old root filesystem - has to be underneath new root
	must(os.MkdirAll(putold, 0700))

	
	must(syscall.PivotRoot(newroot, putold))

	// ensure current working directory is set to new root
	must(os.Chdir("/"))

	// umount putold, granted now path is relative to our new root folder

	must(syscall.Unmount("/putold", syscall.MNT_DETACH))

	// remove putold
	must(os.RemoveAll("/putold"))

	return nil
}


// found at https://github.com/teddyking/ns-process/blob/master/net.go
func waitForNetwork() error {
	maxWait := time.Second * 3
	checkInterval := time.Second
	timeStarted := time.Now()

	for {
		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}

		// pretty basic check ...
		// > 1 as a lo device will already exist
		if len(interfaces) > 1 {
			return nil
		}

		if time.Since(timeStarted) > maxWait {
			return fmt.Errorf("Timeout after %s waiting for network", maxWait)
		}

		time.Sleep(checkInterval)
	}
}


// We call run function before child function because we need to create namespaces before we pupulate them

func run() {
    // telling the programm to call itself but now in child case, we dont yet run the command - we only do that with cmd. lines
	 
    cmd := exec.Command("/proc/self/exe",append([]string{"child"}, os.Args[2:]...)...)
    	
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUSER,
		Unshareflags: syscall.CLONE_NEWNS,
		// mapping user and groups to be root insite the container
	UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID: os.Getuid(),
				Size: 1,
					},
					},
	GidMappings: []syscall.SysProcIDMap{
											{	
						ContainerID: 0,
								HostID: os.Getgid(),
								Size: 1,
								},
							},
					}
             
		must(cmd.Start())

		pid := fmt.Sprintf("%d", cmd.Process.Pid)


	netsetgoCmd := exec.Command("/usr/local/bin/netsetgo", "-pid", pid)
		must(netsetgoCmd.Run())
		must(cmd.Wait())
}
	
	
	
func child () {
        // using cgroup created in previous example to limit memory usage of our process
		parent_cgroup := "/sys/fs/cgroup/test"
	    leaf := filepath.Join(parent_cgroup, "child_test")
		// Limiting cgroup to 10 processes
	    must(ioutil.WriteFile(filepath.Join(leaf, "pids.max"), []byte("10"), 0700))
		// Limiting this cgroup to 10 Mb of memory
		must(ioutil.WriteFile(filepath.Join(leaf, "memory.max"), []byte("10M"), 0700))
		// adding this process to cgroup
	    must(ioutil.WriteFile(filepath.Join(leaf, "cgroup.procs"),[]byte(strconv.Itoa(os.Getpid())), 0700))
		// Execute all arguments after the first one
        
		cmd := exec.Command(os.Args[2], os.Args[3:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		// Mount our directory to place where we stored out alpine linux file system
		
		must(syscall.Mount(
		"proc",
		"/root/alpine/rootfs/proc",
		"proc",
		0,
		"",
	))
		

		// pivoting root to our new root directory
		must(pivotRoot("/root/alpine/rootfs"))
		
        

		python1 := exec.Command("apk add python3")
             python1.Run()

		// Using our newly created UTS namespace to set a new hostname 
		must(syscall.Sethostname([]byte("DaniloAlpineContainer")))
		
		must(waitForNetwork())
		must(cmd.Run())
}



func must(err error) {
 if err != nil {
 fmt.Printf("Error - %s\n", err)
 os.Exit(1)
 }
}
