# Studio One Song Converter

A cross-platform GUI app to downgrade Studio One `.song` files between versions.

## Features
- Drag & drop or browse for `.song` files
- Auto-detects project version and shows only valid conversion options
- Applies all required patches (FormatVersion + Pro EQ UUID fix)
- Studio One-inspired dark theme

---

## Building

### Prerequisites
- [Go 1.21+](https://go.dev/dl/)
- A C compiler (required by Fyne for CGo):
  - **Linux:** `sudo apt install gcc libgl1-mesa-dev xorg-dev`
  - **Windows:** Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or [MSYS2](https://www.msys2.org/)
  - **macOS:** Install Xcode Command Line Tools: `xcode-select --install`
- [Docker](https://www.docker.com/) — only needed for cross-compilation

---

### Option A: Build for your current OS only (simplest)

```bash
cd songconverter
go mod tidy          # downloads dependencies
go build -o SongConverter .
```

On Windows this will produce `SongConverter.exe`.

---

### Option B: Build for all platforms at once (requires Docker)

```bash
chmod +x build_all.sh
./build_all.sh
```

Outputs will be placed in `fyne-cross/dist/`:
```
fyne-cross/dist/
  linux-amd64/SongConverter
  windows-amd64/SongConverter.exe
  darwin-amd64/SongConverter
  darwin-arm64/SongConverter        ← Apple Silicon
```

---

## Usage

1. Launch the app
2. Drag a `.song` file onto the drop zone, or click **Browse**
3. The app detects the project version and shows available conversions
4. Click a conversion button — the patched file is saved next to the original

| Detected Version   | Available Conversions              |
|--------------------|------------------------------------|
| Studio Pro v8 (9)  | → Studio One 7 (v8), → SO6 (v7)   |
| Studio One v7 (8)  | → Studio One 6 (v7)                |
