package pak

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	SignatureMulti  uint32 = 0x3d458cde
	SignatureSingle uint32 = 0x883df70a
	Signature2      uint32 = 0x93847584
	Signature3      uint32 = 0xabfdefbd
	ChunkSize              = 1024
)

type FileDescriptor struct {
	Next             uint32
	Offset           uint32
	CompressedSize   uint32
	UncompressedSize uint32
	Name             string
}

// DecompressFile decompresses a PAK file (single or multi) to the output directory.
func DecompressFile(pakPath, outDir string) error {
	f, err := os.Open(pakPath)
	if err != nil {
		return fmt.Errorf("open pak: %w", err)
	}
	defer f.Close()

	var sig uint32
	if err := binary.Read(f, binary.LittleEndian, &sig); err != nil {
		return fmt.Errorf("read signature: %w", err)
	}

	switch sig {
	case SignatureMulti:
		return decompressMulti(f, outDir)
	case SignatureSingle:
		baseName := filepath.Base(pakPath)
		outName := strings.TrimPrefix(baseName, "pak_")
		return decompressSingle(f, filepath.Join(outDir, outName))
	default:
		return fmt.Errorf("unrecognized PAK signature: 0x%08x", sig)
	}
}

// DecompressSingleFromMemory decompresses a single-file PAK from a byte slice.
// Useful for reading pak__version._ style files.
func DecompressSingleFromMemory(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)

	var sig uint32
	if err := binary.Read(r, binary.LittleEndian, &sig); err != nil {
		return nil, fmt.Errorf("read signature: %w", err)
	}
	if sig != SignatureSingle {
		return nil, fmt.Errorf("expected single-file signature 0x%08x, got 0x%08x", SignatureSingle, sig)
	}

	var sig2, sig3, uncompSize uint32
	if err := binary.Read(r, binary.LittleEndian, &sig2); err != nil {
		return nil, err
	}
	if sig2 != Signature2 {
		return nil, fmt.Errorf("bad signature2: 0x%08x", sig2)
	}
	if err := binary.Read(r, binary.LittleEndian, &sig3); err != nil {
		return nil, err
	}
	if sig3 != Signature3 {
		return nil, fmt.Errorf("bad signature3: 0x%08x", sig3)
	}
	if err := binary.Read(r, binary.LittleEndian, &uncompSize); err != nil {
		return nil, err
	}

	fr := flate.NewReader(r)
	defer fr.Close()

	buf := make([]byte, uncompSize)
	_, err := io.ReadFull(fr, buf)
	if err != nil {
		return nil, fmt.Errorf("inflate: %w", err)
	}
	return buf, nil
}

func decompressSingle(r io.Reader, outPath string) error {
	var sig2, sig3, uncompSize uint32
	if err := binary.Read(r, binary.LittleEndian, &sig2); err != nil {
		return err
	}
	if sig2 != Signature2 {
		return fmt.Errorf("bad signature2: 0x%08x", sig2)
	}
	if err := binary.Read(r, binary.LittleEndian, &sig3); err != nil {
		return err
	}
	if sig3 != Signature3 {
		return fmt.Errorf("bad signature3: 0x%08x", sig3)
	}
	if err := binary.Read(r, binary.LittleEndian, &uncompSize); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	fr := flate.NewReader(r)
	defer fr.Close()

	written, err := io.Copy(out, fr)
	if err != nil {
		return fmt.Errorf("inflate: %w", err)
	}

	if uint32(written) != uncompSize {
		return fmt.Errorf("size mismatch: expected %d, got %d", uncompSize, written)
	}
	return nil
}

func readDescriptor(r io.ReadSeeker) (FileDescriptor, error) {
	var d FileDescriptor
	var header [20]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return d, err
	}

	d.Next = binary.LittleEndian.Uint32(header[0:4])
	d.Offset = binary.LittleEndian.Uint32(header[4:8])
	d.CompressedSize = binary.LittleEndian.Uint32(header[8:12])
	d.UncompressedSize = binary.LittleEndian.Uint32(header[12:16])
	nameLen := binary.LittleEndian.Uint32(header[16:20])

	nameBuf := make([]byte, nameLen)
	if _, err := io.ReadFull(r, nameBuf); err != nil {
		return d, err
	}
	d.Name = strings.TrimRight(string(nameBuf), "\x00")
	return d, nil
}

func decompressMulti(r io.ReadSeeker, outDir string) error {
	var fileCount uint32
	if err := binary.Read(r, binary.LittleEndian, &fileCount); err != nil {
		return err
	}

	descriptors := make([]FileDescriptor, fileCount)
	for i := uint32(0); i < fileCount; i++ {
		desc, err := readDescriptor(r)
		if err != nil {
			return fmt.Errorf("read descriptor %d: %w", i, err)
		}
		descriptors[i] = desc
	}

	for _, desc := range descriptors {
		outName := strings.ReplaceAll(desc.Name, "\\", string(os.PathSeparator))
		outPath := filepath.Join(outDir, outName)

		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}

		if _, err := r.Seek(int64(desc.Offset), io.SeekStart); err != nil {
			return err
		}

		limited := io.LimitReader(r, int64(desc.CompressedSize))
		fr := flate.NewReader(limited)

		out, err := os.Create(outPath)
		if err != nil {
			fr.Close()
			return err
		}

		written, err := io.Copy(out, fr)
		fr.Close()
		out.Close()
		if err != nil {
			return fmt.Errorf("inflate %s: %w", desc.Name, err)
		}
		if uint32(written) != desc.UncompressedSize {
			return fmt.Errorf("%s: size mismatch: expected %d, got %d", desc.Name, desc.UncompressedSize, written)
		}
	}
	return nil
}

// CompressFile compresses a single file into PAK single-file format.
func CompressFile(inputPath, outputDir string) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}
	uncompSize := uint32(info.Size())

	outPath := filepath.Join(outputDir, "pak_"+filepath.Base(inputPath))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	binary.Write(out, binary.LittleEndian, SignatureSingle)
	binary.Write(out, binary.LittleEndian, Signature2)
	binary.Write(out, binary.LittleEndian, Signature3)
	binary.Write(out, binary.LittleEndian, uncompSize)

	fw, err := flate.NewWriter(out, flate.DefaultCompression)
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, in); err != nil {
		fw.Close()
		return err
	}
	return fw.Close()
}

// CompressDir compresses a directory into PAK multi-file format.
func CompressDir(inputDir, outputDir string) error {
	var files []string
	var names []string

	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(inputDir, path)
		if err != nil {
			return err
		}
		files = append(files, path)
		names = append(names, strings.ReplaceAll(rel, "/", "\\"))
		return nil
	})
	if err != nil {
		return err
	}

	outPath := filepath.Join(outputDir, filepath.Base(inputDir)+".pak")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Calculate header size
	headerSize := 8 // signature + count
	for _, name := range names {
		headerSize += 20 + len(name) + 1
	}

	// Write placeholder header
	out.Write(make([]byte, headerSize))

	// Compress each file and track offsets
	descriptors := make([]FileDescriptor, len(files))
	dataOffset := headerSize

	for i, filePath := range files {
		in, err := os.Open(filePath)
		if err != nil {
			return err
		}
		info, _ := in.Stat()

		var compressed bytes.Buffer
		fw, err := flate.NewWriter(&compressed, flate.DefaultCompression)
		if err != nil {
			in.Close()
			return err
		}
		io.Copy(fw, in)
		fw.Close()
		in.Close()

		compData := compressed.Bytes()
		out.Seek(int64(dataOffset), io.SeekStart)
		out.Write(compData)

		descriptors[i] = FileDescriptor{
			Next:             uint32(20 + len(names[i]) + 1),
			Offset:           uint32(dataOffset),
			CompressedSize:   uint32(len(compData)),
			UncompressedSize: uint32(info.Size()),
			Name:             names[i],
		}
		dataOffset += len(compData)
	}

	// Write real header
	out.Seek(0, io.SeekStart)
	binary.Write(out, binary.LittleEndian, SignatureMulti)
	binary.Write(out, binary.LittleEndian, uint32(len(files)))

	for _, desc := range descriptors {
		binary.Write(out, binary.LittleEndian, desc.Next)
		binary.Write(out, binary.LittleEndian, desc.Offset)
		binary.Write(out, binary.LittleEndian, desc.CompressedSize)
		binary.Write(out, binary.LittleEndian, desc.UncompressedSize)
		nameBytes := append([]byte(desc.Name), 0)
		binary.Write(out, binary.LittleEndian, uint32(len(nameBytes)))
		out.Write(nameBytes)
	}

	return nil
}
