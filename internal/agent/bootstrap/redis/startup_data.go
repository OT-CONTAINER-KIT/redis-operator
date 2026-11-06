package bootstrap

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// localDatasetEmpty recognizes a fresh volume, a checksummed empty RDB, or
// a multipart AOF with an empty RDB base and empty/SELECT-only append segments. Other
// data, formats and read failures cannot authorize destructive synchronization.
func localDatasetEmpty(directory string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if entry.Name() == "lost+found" {
			continue
		}
		if entry.Name() == "appendonlydir" && entry.IsDir() {
			empty, err := emptyAOFDirectory(filepath.Join(directory, entry.Name()))
			if err != nil || !empty {
				return false, err
			}
			continue
		}
		if entry.Name() != "dump.rdb" || !entry.Type().IsRegular() {
			return false, nil
		}
		info, err := entry.Info()
		if err != nil {
			return false, err
		}
		if info.Size() > 1<<20 {
			return false, nil
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return false, err
		}
		empty, parseErr := checkedEmptyRDB(data)
		if parseErr != nil {
			return false, parseErr
		}
		if !empty {
			return false, nil
		}
	}
	return true, nil
}

func checkedEmptyRDB(data []byte) (bool, error) {
	if len(data) < 18 {
		return false, fmt.Errorf("truncated RDB")
	}
	// Redis CRC64-Jones, initial zero and no final inversion.
	table := crc64.MakeTable(0x95ac9329ac4bc9b5)
	var checksum uint64
	for _, b := range data[:len(data)-8] {
		checksum = table[byte(checksum)^b] ^ (checksum >> 8)
	}
	stored := binary.LittleEndian.Uint64(data[len(data)-8:])
	if stored == 0 || stored != checksum {
		return false, fmt.Errorf("RDB checksum cannot confirm an empty dataset")
	}
	return emptyRDB(bufio.NewReader(bytes.NewReader(data)))
}

// Only a known manifest, an empty checksummed RDB base and empty or SELECT-only
// append segments establish emptiness. We never replay arbitrary AOF commands.
func emptyAOFDirectory(directory string) (bool, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, err
	}
	var manifest []byte
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return false, nil
		}
		info, err := entry.Info()
		if err != nil {
			return false, err
		}
		if entry.Name() == "appendonly.aof.manifest" {
			if info.Size() > 1<<16 {
				return false, nil
			}
			manifest, err = os.ReadFile(filepath.Join(directory, entry.Name()))
			if err != nil {
				return false, err
			}
		}
	}
	if len(manifest) == 0 {
		return false, nil
	}
	known := map[string]bool{"appendonly.aof.manifest": true}
	for _, line := range strings.Split(string(manifest), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if (len(fields) != 6 && len(fields) != 8 && len(fields) != 10) || fields[0] != "file" || fields[2] != "seq" || fields[4] != "type" || filepath.Base(fields[1]) != fields[1] || known[fields[1]] {
			return false, nil
		}
		if seq, err := strconv.ParseUint(fields[3], 10, 64); err != nil || seq == 0 {
			return false, nil
		}
		offsets := map[string]bool{}
		for i := 6; i < len(fields); i += 2 {
			if fields[5] != "i" || (fields[i] != "startoffset" && fields[i] != "endoffset") || offsets[fields[i]] {
				return false, nil
			}
			value, err := strconv.ParseInt(fields[i+1], 10, 64)
			if err != nil || value < -1 {
				return false, nil
			}
			offsets[fields[i]] = true
		}
		name := fields[1]
		if !strings.HasPrefix(name, "appendonly.aof.") {
			return false, nil
		}
		known[name] = true
		path := filepath.Join(directory, name)
		info, err := os.Lstat(path)
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, nil
		}
		switch fields[5] {
		case "b":
			if !strings.HasSuffix(name, ".base.rdb") || info.Size() > 1<<20 {
				return false, nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return false, err
			}
			empty, err := checkedEmptyRDB(data)
			if err != nil || !empty {
				return false, err
			}
		case "i":
			if !strings.HasSuffix(name, ".incr.aof") || info.Size() > 1<<20 {
				return false, nil
			}
			empty, err := emptyAOFSegment(path)
			if err != nil || !empty {
				return false, err
			}
		default:
			return false, nil
		}
	}
	if len(known) <= 1 || len(known) != len(entries) {
		return false, nil
	}
	for _, entry := range entries {
		if !known[entry.Name()] {
			return false, nil
		}
	}
	return true, nil
}

func emptyAOFSegment(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return false, err
	}
	if len(data) > 1<<20 {
		return false, nil
	}
	reader := bufio.NewReader(bytes.NewReader(data))
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF && line == "" {
			return true, nil
		}
		if err != nil || line != "*2\r\n" {
			return false, nil
		}
		args := make([]string, 2)
		for i := range args {
			line, err := reader.ReadString('\n')
			if err != nil || !strings.HasPrefix(line, "$") {
				return false, nil
			}
			n, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(line, "$"), "\r\n"), 10, 8)
			if err != nil || n > 32 {
				return false, nil
			}
			value := make([]byte, int(n)+2)
			if _, err := io.ReadFull(reader, value); err != nil || string(value[n:]) != "\r\n" {
				return false, nil
			}
			args[i] = string(value[:n])
		}
		if !strings.EqualFold(args[0], "SELECT") {
			return false, nil
		}
		if _, err := strconv.ParseUint(args[1], 10, 32); err != nil {
			return false, nil
		}
	}
}

func rdbLength(r *bufio.Reader) (uint64, int, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	switch b >> 6 {
	case 0:
		return uint64(b & 63), -1, nil
	case 1:
		next, err := r.ReadByte()
		return uint64(b&63)<<8 | uint64(next), -1, err
	case 2:
		var data [8]byte
		switch b {
		case 0x80:
			_, err = io.ReadFull(r, data[:4])
			return uint64(binary.BigEndian.Uint32(data[:4])), -1, err
		case 0x81:
			_, err = io.ReadFull(r, data[:])
			return binary.BigEndian.Uint64(data[:]), -1, err
		default:
			return 0, 0, fmt.Errorf("unsupported RDB length")
		}
	default:
		return 0, int(b & 63), nil
	}
}

func skipRDBString(r *bufio.Reader) error {
	n, encoding, err := rdbLength(r)
	if err != nil {
		return err
	}
	if encoding >= 0 {
		switch encoding {
		case 0:
			n = 1
		case 1:
			n = 2
		case 2:
			n = 4
		case 3:
			var kind int
			n, kind, err = rdbLength(r)
			if err != nil || kind != -1 {
				return fmt.Errorf("invalid compressed RDB auxiliary value")
			}
			_, kind, err = rdbLength(r)
			if err != nil || kind != -1 {
				return fmt.Errorf("invalid compressed RDB auxiliary length")
			}
		default:
			return fmt.Errorf("unsupported RDB auxiliary encoding")
		}
	}
	if n > 1<<30 {
		return fmt.Errorf("oversized RDB auxiliary value")
	}
	_, err = io.CopyN(io.Discard, r, int64(n))
	return err
}

func emptyRDB(r *bufio.Reader) (bool, error) {
	var header [9]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return false, err
	}
	if string(header[:]) != "REDIS0012" {
		return false, fmt.Errorf("invalid RDB header")
	}
	for {
		op, err := r.ReadByte()
		if err != nil {
			return false, err
		}
		switch op {
		case 0xfa: // AUX, two encoded strings.
			if err := skipRDBString(r); err != nil {
				return false, err
			}
			if err := skipRDBString(r); err != nil {
				return false, err
			}
		case 0xfe: // SELECTDB.
			if _, kind, err := rdbLength(r); err != nil || kind != -1 {
				return false, fmt.Errorf("invalid RDB database selector")
			}
		case 0xfb: // RESIZEDB: any key or expiry proves non-empty.
			for i := 0; i < 2; i++ {
				n, kind, err := rdbLength(r)
				if err != nil || kind != -1 {
					return false, fmt.Errorf("invalid RDB database size")
				}
				if n > 0 {
					return false, nil
				}
			}
		case 0xff:
			var checksum [8]byte
			if _, err := io.ReadFull(r, checksum[:]); err != nil {
				return false, err
			}
			if _, err := r.ReadByte(); err != io.EOF {
				return false, fmt.Errorf("unexpected data after RDB end")
			}
			return true, nil
		default:
			return false, fmt.Errorf("RDB data or unsupported record requires manual recovery")
		}
	}
}
