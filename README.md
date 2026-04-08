# photo2cas

`photo2cas` is a Go-based utility that converts modern images (JPEG, PNG, etc.) into a format compatible with the **TRS-80 Color Computer (CoCo)**. It processes images using dithering and outputs both a `.cas` (cassette binary) file and a `.au` (audio) file that can be "played" into a physical or emulated CoCo to display the image.

## Features

- **Floyd-Steinberg Dithering**: Converts colorful or grayscale images into a high-contrast 1-bit-per-pixel (Black & White) format.
- **PMODE 4 Compatibility**: Specifically targets the CoCo's 256x192 high-resolution graphics mode.
- **Cassette ROM Emulation**: Implements the exact logic from the Color BASIC ROM (`CSAVE/CLOAD`) for writing tape blocks, including:
  - 128-byte 0x55 leader.
  - Header block with filename and load/exec addresses.
  - Data blocks (255 bytes max).
  - Checksum calculation.
  - EOF block.
- **Audio Generation**: Produces an 8-bit linear PCM `.au` file with timing synced to the CoCo's 0.895 MHz CPU clock.

## How it Works

1. **Preprocessing**: The input image is resized to fit a 256x192 canvas while maintaining its aspect ratio.
2. **Dithering**: The image is converted to grayscale and then dithered to 1-bit depth.
3. **Packing**: Pixels are packed 8-to-a-byte, with the Most Significant Bit (MSB) representing the leftmost pixel, matching the CoCo's PMODE 4 memory layout.
4. **Encoding**: The packed data is wrapped in standard Color BASIC cassette blocks.
5. **Signal Synthesis**: The bits are converted into sine-wave cycles at 1200 baud (1) and 2400 baud (0) following the original ROM's assembly logic.

## Prerequisites

- [Go](https://go.dev/doc/install) (1.18 or higher recommended)
- A PNG image file

## Installation

1. Clone or download this repository.
2. Build the executable:
   ```bash
   go build -o photo2cas
   ```

## Usage

Run the application by providing the path to an image:

```bash
./photo2cas path/to/your/image.png
```

### Options

You can specify custom output filenames using flags:

```bash
./photo2cas -audio my_image.au -cas my_image.cas path/to/image.png
```

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-audio` | `picture.au` | The output audio file for playback |
| `-cas` | `picture.cas` | The output cassette binary file |

## Loading on a TRS-80 Color Computer

### On Real Hardware
1. Connect your computer's audio output to the CoCo's cassette "Data In" port.
2. On the CoCo, enter the following BASIC program:
   ```basic
   10 PMODE 4,1:SCREEN 1,0:PCLS
   20 CLOADM:EXEC
   30 GOTO 30
   ```
3. Type `RUN` on the CoCo.
4. Play the generated `.au` file from your computer.

### On an Emulator (e.g., XRoar)
1. Load the generated `.cas` file as a cassette tape.
2. Enter the BASIC program provided above and `RUN`.

## Technical References
The cassette encoding logic is based on the [Color BASIC Unravelled](https://colorcomputerarchive.com/repo/Documents/Books/Unravelled%20Series/color-basic-unravelled.pdf) series, specifically the `CSAVE` and `CLOAD` ROM routines.
