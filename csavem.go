package main

import (
	"bufio"
	"io"
	"os"
)

var sineTable = []int{
	0x082, 0x092, 0x0AA, 0x0BA, 0x0CA, 0x0DA,
	0x0EA, 0x0F2, 0x0FA, 0x0FA, 0x0FA, 0x0F2,
	0x0EA, 0x0DA, 0x0CA, 0x0BA, 0x0AA, 0x092,
	0x07A, 0x06A, 0x052, 0x042, 0x032, 0x022,
	0x012, 0x00A, 0x002, 0x002, 0x002, 0x00A,
	0x012, 0x022, 0x032, 0x042, 0x052, 0x06A}

type CassetteWriter struct {
	audio    *bufio.Writer
	cassette *bufio.Writer
	da       byte
	clstsn   int
}

func NewCassetteWriter(audioW, cassetteW io.Writer) *CassetteWriter {
	return &CassetteWriter{
		audio:    bufio.NewWriter(audioW),
		cassette: bufio.NewWriter(cassetteW),
		da:       0x80,
	}
}

func (cw *CassetteWriter) Flush() error {
	if err := cw.audio.Flush(); err != nil {
		return err
	}
	return cw.cassette.Flush()
}

func (cw *CassetteWriter) updateDa(val int) {
	cw.da = byte(((val & 0x0FC) - 128) & 0x0FF)
}

func (cw *CassetteWriter) writeAuFileHeader() error {
	_, err := cw.audio.Write([]byte{
		0x2e, 0x73, 0x6e, 0x64, // .snd
		0x00, 0x00, 0x00, 0x18, // header size
		0xff, 0xff, 0xff, 0xff, // unknown length
		0x00, 0x00, 0x00, 0x02, // 8-bit linear PCM
		0x00, 0x0d, 0xa8, 0x18, // 895000 samples/sec (approx) - wait, 0x000DA818 is 895000? No, 0x00002B11 is 11025. 0x0000AC44 is 44100.
		// Let's check 0x000DA818: it's 895000. This is the CoCo's CPU clock speed.
		0x00, 0x00, 0x00, 0x01, // 1 channel
	})
	return err
}

func (cw *CassetteWriter) writeDaFor(cycles int) error {
	for i := 0; i < cycles; i++ {
		if err := cw.audio.WriteByte(cw.da); err != nil {
			return err
		}
	}
	return nil
}

func (cw *CassetteWriter) writeBit(bit bool) error {
	step := 1
	timingBump := 0

	if bit {
		step = 2
		timingBump = 1
	}

	for i := 0; i < len(sineTable)-1; i += step {
		if err := cw.writeDaFor(6 + 5 + 3 + 5 + timingBump); err != nil {
			return err
		}
		cw.updateDa(sineTable[i])
		if err := cw.writeDaFor(3); err != nil {
			return err
		}
	}

	if err := cw.writeDaFor(6 + 5 + 3 + timingBump); err != nil {
		return err
	}
	if err := cw.writeDaFor(4 + 2 + 3); err != nil {
		return err
	}
	cw.clstsn = sineTable[len(sineTable)-step]
	return nil
}

func (cw *CassetteWriter) writeByte(val byte) error {
	if err := cw.cassette.WriteByte(val); err != nil {
		return err
	}

	if err := cw.writeDaFor(6 + 2); err != nil {
		return err
	}

	var mask byte = 1
	for i := 0; i < 8; i++ {
		if err := cw.writeDaFor(4 + 5); err != nil {
			return err
		}
		cw.updateDa(cw.clstsn)
		if err := cw.writeDaFor(4 + 4 + 3); err != nil {
			return err
		}

		if err := cw.writeBit((val & mask) != 0); err != nil {
			return err
		}
		mask <<= 1
	}

	return cw.writeDaFor(8)
}

func (cw *CassetteWriter) writeBlock(blkType byte, blkData []byte) error {
	blkLen := byte(len(blkData))
	if err := cw.writeDaFor(3 + 4 + 4 + 4 + 3); err != nil {
		return err
	}

	cksum := blkLen + blkType
	if blkLen != 0 {
		if err := cw.writeDaFor(5 + int(blkLen)*(6+2+3)); err != nil {
			return err
		}
		for _, val := range blkData {
			cksum += val
		}
	}

	if err := cw.writeDaFor(4 + 4 + 5 + 7 + 2); err != nil {
		return err
	}

	if err := cw.writeByte(0x55); err != nil { // start of block
		return err
	}

	if err := cw.writeDaFor(2 + 7); err != nil {
		return err
	}
	if err := cw.writeByte(0x3C); err != nil { // sync char
		return err
	}

	if err := cw.writeDaFor(4 + 7); err != nil {
		return err
	}
	if err := cw.writeByte(blkType); err != nil {
		return err
	}

	if err := cw.writeDaFor(4 + 7); err != nil {
		return err
	}
	if err := cw.writeByte(blkLen); err != nil {
		return err
	}

	if err := cw.writeDaFor(2 + 3); err != nil {
		return err
	}
	for _, val := range blkData {
		if err := cw.writeDaFor(6 + 7); err != nil {
			return err
		}
		if err := cw.writeByte(val); err != nil {
			return err
		}
		if err := cw.writeDaFor(6 + 3); err != nil {
			return err
		}
	}

	if err := cw.writeDaFor(4 + 7); err != nil {
		return err
	}
	if err := cw.writeByte(cksum); err != nil {
		return err
	}

	if err := cw.writeDaFor(2); err != nil {
		return err
	}
	return cw.writeByte(0x55)
}

func (cw *CassetteWriter) writeLeader() error {
	if err := cw.writeDaFor(5 + 7 + 2); err != nil {
		return err
	}
	if err := cw.writeByte(0x55); err != nil {
		return err
	}

	for i := 0; i < 127; i++ {
		if err := cw.writeDaFor(5 + 3 + 7 + 2); err != nil {
			return err
		}
		if err := cw.writeByte(0x55); err != nil {
			return err
		}
	}

	return cw.writeDaFor(5 + 3 + 5)
}

func (cw *CassetteWriter) writeHalfSecondDelay() error {
	// 5 + 65536*(5+3) + 5 cycles
	return cw.writeDaFor(524298)
}

func (cw *CassetteWriter) writeDataBlocks(data []byte) error {
	if err := cw.writeDaFor(6); err != nil {
		return err
	}
	for i := 0; i < len(data); i += 255 {
		end := i + 255
		if end > len(data) {
			end = len(data)
		}
		blkData := data[i:end]

		if err := cw.writeDaFor(5 + 2 + 4 + 6 + 6 + 3 + 5 + 3); err != nil {
			return err
		}

		if len(blkData) < 255 {
			if err := cw.writeDaFor(2 + 4); err != nil {
				return err
			}
		}

		if err := cw.writeDaFor(8); err != nil {
			return err
		}
		if err := cw.writeBlock(0x01, blkData); err != nil {
			return err
		}
		if err := cw.writeDaFor(3); err != nil {
			return err
		}
	}

	return cw.writeDaFor(5 + 2 + 4 + 6 + 6 + 3 + 5 + 4)
}

func writeImageToCassette(data []byte, audioName, casName string) error {
	audioFile, err := os.Create(audioName)
	if err != nil {
		return err
	}
	defer audioFile.Close()

	casFile, err := os.Create(casName)
	if err != nil {
		return err
	}
	defer casFile.Close()

	cw := NewCassetteWriter(audioFile, casFile)

	if err := cw.writeAuFileHeader(); err != nil {
		return err
	}

	// turn the cassette motor on
	if err := cw.writeDaFor(5); err != nil {
		return err
	}
	if err := cw.writeHalfSecondDelay(); err != nil {
		return err
	}
	if err := cw.writeLeader(); err != nil {
		return err
	}

	// write the header block
	// Disk Extended Color Basic 1.1 values
	addrBasicInterpreter := []byte{0xad, 0xfb}
	addrGraphicsPage := []byte{0x0e, 0x00}

	headerBlk := make([]byte, 0, 15)
	headerBlk = append(headerBlk, "PICTURE "...)
	headerBlk = append(headerBlk, 0x02, 0x00, 0x00)
	headerBlk = append(headerBlk, addrBasicInterpreter...)
	headerBlk = append(headerBlk, addrGraphicsPage...)

	if err := cw.writeDaFor(7); err != nil {
		return err
	}
	if err := cw.writeBlock(0x00, headerBlk); err != nil {
		return err
	}

	// turn the cassette motor off
	if err := cw.writeDaFor(3 + 5 + 2 + 5); err != nil {
		return err
	}

	// turn the cassette motor on again
	if err := cw.writeDaFor(5); err != nil {
		return err
	}
	if err := cw.writeHalfSecondDelay(); err != nil {
		return err
	}
	if err := cw.writeLeader(); err != nil {
		return err
	}

	if err := cw.writeDataBlocks(data); err != nil {
		return err
	}

	if err := cw.writeDaFor(6 + 6 + 4); err != nil {
		return err
	}
	if err := cw.writeDaFor(7); err != nil {
		return err
	}

	// write eof block
	if err := cw.writeBlock(0xff, nil); err != nil {
		return err
	}

	// turn cassette motor off
	if err := cw.writeDaFor(3 + 5 + 2 + 5); err != nil {
		return err
	}

	return cw.Flush()
}
