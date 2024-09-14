package agent_upgrade

import (
	"context"
	"encoding/binary"
	"fmt"
	"remote-agent/agent/agent_common"
	"remote-agent/biz"
	"remote-agent/utils"
	"sync"
)

func Run(task *biz.AgentNotify, cancel_agent_task_stream context.CancelFunc) {
	c, err := agent_common.MakeWsConn(task.Id)
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

		upgrade, err := StartUpgradeExecutable()
		if err != nil {
			return fmt.Errorf("failed to make executable backup: %w", err)
		}
		defer upgrade.Close()

		ws.Write <- utils.PrependBytes([]byte{0x00}, []byte(upgrade.ExecPath))

		// ---- recv executable info
		recv, ok = <-ws.Read
		if !ok || recv[0] != 0x01 || len(recv) != 9 {
			// bad request
			return fmt.Errorf("bad request, cannot read info")
		}
		total_size := int64(binary.LittleEndian.Uint64(recv[1:9]))
		if err = upgrade.Truncate(total_size); err != nil {
			return fmt.Errorf("failed to truncate temp file: %w. total_size: %d", err, total_size)
		}

		// ---- recv executable chunks
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
			if _, err := upgrade.Write(data); err != nil {
				return fmt.Errorf("failed to write temp file: %w", err)
			}

			size_received += int64(len(data))
			ws.Write <- binary.LittleEndian.AppendUint64([]byte{0x00}, uint64(size_received))
		}

		// --- recv done
		ws.Write <- []byte{0x01}

		// --- run new executable
		if err = upgrade.Close(); err != nil {
			return err
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
