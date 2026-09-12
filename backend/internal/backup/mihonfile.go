package backup

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/proto"

	"tsunagu/backend/internal/backup/mihonpb"
)

func WriteMihonFile(path string, b *mihonpb.Backup) error {
	data, err := proto.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshal backup: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	if _, err := gw.Write(data); err != nil {
		return err
	}
	return gw.Close()
}

func ReadMihonFile(path string) (*mihonpb.Backup, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data := raw
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer gr.Close()
		data, err = io.ReadAll(gr)
		if err != nil {
			return nil, fmt.Errorf("gzip read: %w", err)
		}
	}
	var b mihonpb.Backup
	if err := proto.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("decode backup: %w", err)
	}
	return &b, nil
}
