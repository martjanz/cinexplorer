// Package platform opens files and folders with the OS default applications.
package platform

import "os/exec"

// start launches cmd without blocking and reaps it in the background.
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
