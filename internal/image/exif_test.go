package image

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestReadCaptureTime(t *testing.T) {
	for _, value := range []string{"2024:05:06 12:13:14", "0000:00:00 00:00:00"} {
		// A minimal TIFF with one ASCII DateTimeOriginal entry.
		data := make([]byte, 46)
		copy(data, []byte{'I', 'I', 42, 0, 8, 0, 0, 0})
		binary.LittleEndian.PutUint16(data[8:], 1)
		binary.LittleEndian.PutUint16(data[10:], 0x9003)
		binary.LittleEndian.PutUint16(data[12:], 2)
		binary.LittleEndian.PutUint32(data[14:], 20)
		binary.LittleEndian.PutUint32(data[18:], 26)
		copy(data[26:], value)
		want, _ := time.Parse("2006:01:02 15:04:05", value)
		if got := ReadCaptureTime(bytes.NewReader(data), "photo.tiff"); !got.Equal(want) {
			t.Fatalf("capture time = %v, want %v", got, want)
		}
	}
	if got := ReadCaptureTime(bytes.NewReader([]byte("invalid")), "photo.jpg"); !got.IsZero() {
		t.Fatal("invalid file has a capture date")
	}
}
