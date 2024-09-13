package agent_upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type ExecutableUpgrade struct {
	exec_path, temp_path string
	file                 *os.File
}

func (upgrade *ExecutableUpgrade) Truncate(size int64) error {
	return upgrade.file.Truncate(size)
}

func (upgrade *ExecutableUpgrade) Write(data []byte) (n int, err error) {
	return upgrade.file.Write(data)
}

// close file and try pulling up the executable
// do nothing if already closed.
// otherwise, if fails to start the new executable, it will revert
func (upgrade *ExecutableUpgrade) Close() (err error) {
	if upgrade.file == nil {
		return fmt.Errorf("file already closed")
	}

	upgrade.file.Close()
	upgrade.file = nil

	defer func() {
		if err != nil {
			os.Remove(upgrade.exec_path)
			os.Rename(upgrade.temp_path, upgrade.exec_path)
		}
	}()

	// start the new executable

	cmd := exec.Command(upgrade.exec_path, os.Args[1:]...)
	cmd.Env = os.Environ()
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to run executable: %w", err)
	}

	// wait and check if it crashed
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		err = fmt.Errorf("executable exited with code %d", cmd.ProcessState.ExitCode())
		return err
	case <-time.After(time.Second * 2):
		// after 2 seconds, process is still alive, good
	}

	// very success
	os.Remove(upgrade.temp_path)
	return nil
}

// try to generate a "temp_path", rename current executable "path" to a "temp_path"
// and create a empty file to "path"
func makeExecutableUpgrade() (upgrade *ExecutableUpgrade, err error) {
	// Get the current executable exec_path
	var exec_path, temp_path string

	exec_path, err = os.Executable()
	if err != nil {
		return
	}

	// Get the directory and base name of the executable
	execDir := filepath.Dir(exec_path)
	execName := filepath.Base(exec_path)

	// Generate the new name for the executable with current timestamp
	timestamp := time.Now().Format("20060102150405") // Format YYYYMMDDHHMMSS
	newName := fmt.Sprintf("%s.temp.%s", execName, timestamp)
	temp_path = filepath.Join(execDir, newName)

	// Rename the executable file
	err = os.Rename(exec_path, temp_path)
	if err != nil {
		return
	}

	// seems good.
	defer func() {
		// in case things goes wrong.... swap back
		if err != nil {
			os.Remove(exec_path)
			os.Rename(temp_path, exec_path)
		}
	}()

	// create a placeholder of the new executable
	var exec_stat os.FileInfo
	exec_stat, err = os.Stat(temp_path)
	if err != nil {
		return
	}

	var file *os.File
	file, err = os.OpenFile(exec_path, os.O_CREATE|os.O_WRONLY, exec_stat.Mode())
	if err != nil {
		return
	}

	upgrade = &ExecutableUpgrade{
		exec_path: exec_path,
		temp_path: temp_path,
		file:      file,
	}

	return
}
