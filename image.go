package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
)

const (
	tgtWidth     = 256
	tgtHeight    = 192
	widthInBytes = tgtWidth / 8
)

// ditherPixel returns the closest color (black or white) for a given gray value.
func ditherPixel(g color.Gray) color.Gray {
	if g.Y < 128 {
		return color.Gray{Y: 0} // Black
	}
	return color.Gray{Y: 255} // White
}

// floydSteinbergDither applies Floyd-Steinberg dithering to a grayscale image.
func floydSteinbergDither(gray *image.Gray) *image.Gray {
	bounds := gray.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	// Create a copy to avoid modifying the original
	dithered := image.NewGray(bounds)
	copy(dithered.Pix, gray.Pix)

	// Use a slice of int16 to store errors for better precision during distribution
	// and to avoid repeated uint8 <-> int16 conversions.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			oldPixel := dithered.GrayAt(x, y)
			newPixel := ditherPixel(oldPixel)
			dithered.SetGray(x, y, newPixel)

			err := int16(oldPixel.Y) - int16(newPixel.Y)

			// Distribute error to neighbors
			distributeError(dithered, x+1, y, (err*7)/16)
			distributeError(dithered, x-1, y+1, (err*3)/16)
			distributeError(dithered, x, y+1, (err*5)/16)
			distributeError(dithered, x+1, y+1, (err*1)/16)
		}
	}
	return dithered
}

func distributeError(img *image.Gray, x, y int, err int16) {
	if !image.Pt(x, y).In(img.Bounds()) {
		return
	}
	val := int16(img.GrayAt(x, y).Y) + err
	if val < 0 {
		val = 0
	} else if val > 255 {
		val = 255
	}
	img.SetGray(x, y, color.Gray{Y: uint8(val)})
}

// loadImage reads and decodes an image from a file path.
// It includes a workaround for Go's strict PNG decoder, which can fail on 
// non-essential metadata chunks (like iCCP) with invalid checksums.
func loadImage(filepath string) (image.Image, error) {
	infile, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer infile.Close()

	// Use a peek-like approach to check if it's a PNG
	header := make([]byte, 8)
	if n, _ := infile.Read(header); n == 8 && isPNG(header) {
		infile.Seek(0, io.SeekStart)
		return decodeSafePNG(infile)
	}

	// Fallback to standard decoding for other formats
	infile.Seek(0, io.SeekStart)
	img, _, err := image.Decode(infile)
	return img, err
}

func isPNG(header []byte) bool {
	return bytes.Equal(header, []byte("\x89PNG\r\n\x1a\n"))
}

// decodeSafePNG filters out problematic chunks (like iCCP) that might have 
// invalid checksums, which Go's standard decoder is strict about.
func decodeSafePNG(r io.Reader) (image.Image, error) {
	var filtered bytes.Buffer
	header := make([]byte, 8)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	filtered.Write(header)

	for {
		var length uint32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		chunkType := make([]byte, 4)
		if _, err := io.ReadFull(r, chunkType); err != nil {
			return nil, err
		}

		// Skip chunks that are known to often have CRC issues or are unnecessary
		if string(chunkType) == "iCCP" {
			if _, err := io.CopyN(io.Discard, r, int64(length)+4); err != nil {
				return nil, err
			}
			continue
		}

		// Write length and type
		binary.Write(&filtered, binary.BigEndian, length)
		filtered.Write(chunkType)

		// Copy data and CRC
		if _, err := io.CopyN(&filtered, r, int64(length)+4); err != nil {
			return nil, err
		}

		if string(chunkType) == "IEND" {
			break
		}
	}

	return png.Decode(&filtered)
}

// packImage converts a 1-bit-per-pixel image into a byte array suitable for the CoCo PMODE 4.
// It centers the input image within the target 256x192 resolution.
func packImage(img *image.Gray) []byte {
	data := make([]byte, widthInBytes*tgtHeight)
	
	imgBounds := img.Bounds()
	imgWidth := imgBounds.Dx()
	imgHeight := imgBounds.Dy()

	// Calculate centering offsets
	offsetX := (tgtWidth - imgWidth) / 2
	offsetY := (tgtHeight - imgHeight) / 2

	for y := 0; y < imgHeight; y++ {
		targetY := y + offsetY
		if targetY < 0 || targetY >= tgtHeight {
			continue
		}

		for x := 0; x < imgWidth; x++ {
			targetX := x + offsetX
			if targetX < 0 || targetX >= tgtWidth {
				continue
			}

			// CoCo PMODE 4: 0 is black, 1 is white.
			// image.Gray: 0 is black, 255 is white.
			if img.GrayAt(x, y).Y > 127 {
				byteIdx := targetY*widthInBytes + (targetX / 8)
				bitIdx := 7 - (targetX % 8) // CoCo uses MSB for leftmost pixel
				data[byteIdx] |= (1 << bitIdx)
			}
		}
	}

	return data
}

// toGray converts any image to a grayscale image.
func toGray(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return gray
}
