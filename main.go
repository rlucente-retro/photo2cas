package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/disintegration/imaging"
)

func main() {
	var (
		audioOut = flag.String("audio", "picture.au", "Output audio file name")
		casOut   = flag.String("cas", "picture.cas", "Output cassette binary file name")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <image_file>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	filename := flag.Arg(0)
	if filename == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(filename, *audioOut, *casOut); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(filename, audioOut, casOut string) error {
	img, err := loadImage(filename)
	if err != nil {
		return fmt.Errorf("loading image: %w", err)
	}

	// Resize image to fit within CoCo's PMODE 4 resolution while preserving aspect ratio
	shrunk := imaging.Fit(img, tgtWidth, tgtHeight, imaging.Lanczos)
	
	// Convert to grayscale
	gray := toGray(shrunk)
	
	// Apply Floyd-Steinberg dithering
	dithered := floydSteinbergDither(gray)
	
	// Pack into 1-bit-per-pixel data for CoCo
	data := packImage(dithered)

	fmt.Printf("Writing cassette files: %s, %s\n", audioOut, casOut)
	if err := writeImageToCassette(data, audioOut, casOut); err != nil {
		return fmt.Errorf("writing cassette files: %w", err)
	}

	return nil
}
