package agent

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"remote-agent/biz"
	"remote-agent/utils"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func run_upgrade(task *biz.AgentNotify) {
	url := biz.Config.BaseUrl + "/api/agent/" + biz.Config.Name + "/" + task.Id
	url = strings.Replace(url, "http", "ws", 1)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}
	defer c.Close()

	wg := sync.WaitGroup{}
	ws := utils.WSConnToChannels(c, &wg)

	defer wg.Wait()
	defer close(ws.Write)

	// ---- setup process
	err = func() error {
		recv, ok := <-ws.Read
		if !ok || recv[0] != 0x00 || len(recv) != 1 {
			// bad request
			return fmt.Errorf("bad request")
		}

		exec_path, temp_path, err := make_executable_backup()
		if err != nil {
			return fmt.Errorf("failed to make executable backup: %w", err)
		}
		exec_stat, err := os.Stat(exec_path)
		if err != nil {
			return fmt.Errorf("failed to stat executable: %w", err)
		}

		ws.Write <- utils.PrependBytes([]byte{0x00}, []byte(exec_path))

		// ---- recv executable info
		recv, ok = <-ws.Read
		if !ok || recv[0] != 0x01 || len(recv) != 9 {
			// bad request
			return fmt.Errorf("bad request, cannot read info")
		}
		total_size := int64(binary.LittleEndian.Uint64(recv[1:9]))

		// ---- recv executable chunks
		if err := func() error {
			file, err := os.OpenFile(temp_path, os.O_RDWR|os.O_CREATE, exec_stat.Mode())
			if err != nil {
				return fmt.Errorf("failed to open temp file: %w", err)
			}
			defer file.Close()

			if err = file.Truncate(total_size); err != nil {
				return fmt.Errorf("failed to truncate temp file: %w", err)
			}

			// ---- recv executable chunk
			for size_received := int64(0); size_received < total_size; {
				recv, ok := <-ws.Read
				if !ok || recv[0] != 0x02 || len(recv) < 17 {
					return fmt.Errorf("bad request, cannot read chunk")
				}

				offset := int64(binary.LittleEndian.Uint64(recv[1:9]))
				if offset != size_received {
					return fmt.Errorf("bad request, offset mismatch")
				}

				data := recv[9:]
				if _, err := file.WriteAt(data, offset); err != nil {
					return fmt.Errorf("failed to write temp file: %w", err)
				}

				size_received += int64(len(data))
				ws.Write <- binary.LittleEndian.AppendUint64([]byte{0x00}, uint64(size_received))
			}

			// --- recv done
			return nil
		}(); err != nil {
			return err
		}

		// ---- rename executable
		temp_path2 := temp_path + ".del"
		if err = os.Rename(exec_path, temp_path2); err != nil {
			return fmt.Errorf("failed to rename temp file: %w", err)
		}
		if err = os.Rename(temp_path, exec_path); err != nil {
			os.Rename(temp_path2, exec_path) // rollback
			return fmt.Errorf("failed to rename executable: %w", err)
		}

		ws.Write <- []byte{0x01}

		// ---- now only exists "exec_path" and "temp_path2"
		// in next launch, agent shall delete it

		// start a new executable
		cmd := exec.Command(exec_path, os.Args[1:]...)
		cmd.Env = os.Environ()
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("failed to run executable: %w", err)
		}

		time.Sleep(time.Second * 2)
		if cmd.ProcessState.Exited() {
			// rollback
			os.Rename(exec_path, temp_path)
			os.Rename(temp_path2, exec_path)
			os.Rename(temp_path, temp_path2)
			return fmt.Errorf("executable exited with code %d", cmd.ProcessState.ExitCode())
		}

		// success. terminate self
		cancel_agent_task_stream()
		ws.Write <- []byte{0x02}

		return nil
	}()
	if err != nil {
		ws.Write <- utils.PrependBytes([]byte{0x99}, []byte(err.Error()))
	}
}

// renameExecutable renames the executable file to include a ".backup.<timestamp>" suffix
func make_executable_backup() (original, temp_path string, err error) {
	// Get the current executable path
	original, err = os.Executable()
	if err != nil {
		return
	}

	// Get the directory and base name of the executable
	execDir := filepath.Dir(original)
	execName := filepath.Base(original)

	// Generate the new name for the executable with current timestamp
	timestamp := time.Now().Format("20060102150405") // Format YYYYMMDDHHMMSS
	newName := fmt.Sprintf("%s.temp.%s", execName, timestamp)
	temp_path = filepath.Join(execDir, newName)

	// Rename the executable file
	err = os.Rename(original, temp_path)
	if err != nil {
		return
	}

	// permission good, swap back
	err = os.Rename(temp_path, original)
	return
}
