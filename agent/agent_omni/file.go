package agent_omni

import (
	"encoding/binary"
	"os"
	"remote-agent/biz"
	"remote-agent/utils"
)

func (s *PtySession) SetupFileTransfer() {
	// upload file chunk
	s.Handlers[0x10] = func(recv []byte) {
		offset := int64(binary.LittleEndian.Uint64(recv[1:]))
		length := int64(binary.LittleEndian.Uint64(recv[9:]))
		data_since := int64(len(recv)) - length
		path := string(recv[17:data_since])
		data := recv[data_since:]
		if err := write_file_chunk(path, offset, data); err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}
		s.Write(utils.JoinBytes2(
			0x10,
			binary.LittleEndian.AppendUint64(nil, uint64(offset)),
			[]byte(path),
		))
	}

	// read file info
	s.Handlers[0x11] = func(recv []byte) {
		path := string(recv[1:])
		info, err := os.Stat(path)
		if err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}

		msg := biz.FileInfo{
			Path:  path,
			Size:  int64(info.Size()),
			Mode:  uint32(info.Mode()),
			Mtime: info.ModTime().Unix(),
		}
		msg_bytes, err := msg.MarshalMsg(nil)
		if err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}
		s.Write(utils.PrependBytes([]byte{0x11}, msg_bytes))
	}

	// read file chunk
	s.Handlers[0x12] = func(recv []byte) {
		offset := int64(binary.LittleEndian.Uint64(recv[1:]))
		length := int64(binary.LittleEndian.Uint64(recv[9:]))
		file_path := string(recv[17:])

		data, err := read_file_chunk(file_path, offset, length)
		if data == nil {
			s.WriteDebugMessage(err.Error())
			return
		}

		actual_length := int64(len(data))
		s.Write(utils.JoinBytes2(
			0x12,
			binary.LittleEndian.AppendUint64(nil, uint64(offset)),
			binary.LittleEndian.AppendUint64(nil, uint64(actual_length)),
			[]byte(file_path),
			data,
		))
	}
}

// create or update a file.
// if file exists, extend file size and overwrite data to the certain offset
// if data length is 0, it means truncate the file to length of offset
func write_file_chunk(path string, offset int64, data []byte) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if len(data) == 0 {
		// truncate file
		err = file.Truncate(offset)
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// extend file size if necessary
	file_size := stat.Size()
	min_required_size := offset + int64(len(data))
	if min_required_size > file_size {
		// file is not big enough
		if err := file.Truncate(min_required_size); err != nil {
			return err
		}
	}

	// write data
	if _, err := file.WriteAt(data, offset); err != nil {
		return err
	}

	return nil
}

// read a file chunk
func read_file_chunk(path string, offset int64, max_length int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := make([]byte, max_length)
	n, err := file.ReadAt(buf, offset)
	return buf[:n], err
}
