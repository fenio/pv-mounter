package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// checkNFSClient verifies that the NFS mount client is available.
func checkNFSClient() {
	switch runtime.GOOS {
	case "darwin":
		// macOS ships with mount -t nfs built in
		_, err := exec.LookPath("mount")
		if err != nil {
			fmt.Println("mount is not available in your environment.")
			fmt.Println("For macOS, mount should be available by default.")
			os.Exit(1)
		}
	default:
		_, err := exec.LookPath("mount.nfs4")
		if err != nil {
			fmt.Println("mount.nfs4 is not available in your environment.")
			fmt.Println("For Linux, please install nfs-common: sudo apt-get install nfs-common")
			os.Exit(1)
		}
	}
}
