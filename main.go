package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ── Constants ─────────────────────────────────────────────────────────────────

const (
	oldUUID = "D9AE9ACD-69B4-4B43-B8D5-983E39C559A5"
	newUUID = "073C4094-E062-4FB5-8328-74608DD1A3A4"
)

// ── Studio One Theme ──────────────────────────────────────────────────────────

type soTheme struct{}

var _ fyne.Theme = (*soTheme)(nil)

func (t soTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x1C, G: 0x1C, B: 0x1C, A: 0xFF}
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x38, G: 0x38, B: 0x38, A: 0xFF}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xDF, G: 0xDF, B: 0xDF, A: 0xFF}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x55, G: 0x55, B: 0x55, A: 0xFF}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x3D, G: 0x8E, B: 0xBF, A: 0xFF}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x3D, G: 0x8E, B: 0xBF, A: 0x28}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 0x3D, G: 0x8E, B: 0xBF, A: 0x50}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x26, G: 0x26, B: 0x26, A: 0xFF}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x3A, G: 0x3A, B: 0x3A, A: 0xFF}
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (t soTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (t soTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (t soTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 8
	case theme.SizeNameInnerPadding:
		return 10
	}
	return theme.DefaultTheme().Size(n)
}

// ── Song Logic ────────────────────────────────────────────────────────────────

func readFormatVersion(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("not a valid .song file")
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != "metainfo.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		re := regexp.MustCompile(`<Attribute id="Document:FormatVersion" value="(\d+)"/>`)
		m := re.FindSubmatch(data)
		if m == nil {
			return "", fmt.Errorf("FormatVersion not found in metainfo.xml")
		}
		return string(m[1]), nil
	}
	return "", fmt.Errorf("metainfo.xml not found in archive")
}

func convertSong(inputPath, currentVersion, targetVersion, suffix string) (string, error) {
	r, err := zip.OpenReader(inputPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	dir := filepath.Dir(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ".song")
	outputPath := filepath.Join(dir, base+"_"+suffix+".song")

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}

		if f.Name == "metainfo.xml" {
			oldStr := fmt.Sprintf(`<Attribute id="Document:FormatVersion" value="%s"/>`, currentVersion)
			newStr := fmt.Sprintf(`<Attribute id="Document:FormatVersion" value="%s"/>`, targetVersion)
			data = bytes.ReplaceAll(data, []byte(oldStr), []byte(newStr))
		}
		if f.Name == "Devices/audiomixer.xml" {
			data = bytes.ReplaceAll(data, []byte(oldUUID), []byte(newUUID))
		}

		fw, err := w.Create(f.Name)
		if err != nil {
			return "", err
		}
		if _, err := fw.Write(data); err != nil {
			return "", err
		}
	}

	if err := w.Close(); err != nil {
		return "", err
	}
	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}
	return outputPath, nil
}

// ── Colors ────────────────────────────────────────────────────────────────────

var (
	colAccent  = color.NRGBA{R: 0x3D, G: 0x8E, B: 0xBF, A: 0xFF}
	colMuted   = color.NRGBA{R: 0x77, G: 0x77, B: 0x77, A: 0xFF}
	colSuccess = color.NRGBA{R: 0x5C, G: 0xB8, B: 0x5C, A: 0xFF}
	colError   = color.NRGBA{R: 0xD9, G: 0x53, B: 0x4F, A: 0xFF}
	colText    = color.NRGBA{R: 0xDF, G: 0xDF, B: 0xDF, A: 0xFF}
	colPanel   = color.NRGBA{R: 0x26, G: 0x26, B: 0x26, A: 0xFF}
)

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	a := app.NewWithID("com.songconverter.app")
	a.Settings().SetTheme(&soTheme{})

	w := a.NewWindow("Studio One Song Converter")
	w.Resize(fyne.NewSize(460, 460))
	w.SetFixedSize(true)

	// ── State ──
	type songEntry struct {
		path    string
		version string
	}
	var loadedFiles []songEntry

	// ── UI Elements ──

	// Drop zone — no border, just the dark background panel
	dropBg := canvas.NewRectangle(colPanel)
	dropBg.CornerRadius = 6

	dropIcon := widget.NewIcon(theme.UploadIcon())
	dropMainText := canvas.NewText("Drop .song file(s) here", colText)
	dropMainText.TextSize = 14
	dropMainText.Alignment = fyne.TextAlignCenter

	dropSubText := canvas.NewText("or use the Browse button below", colMuted)
	dropSubText.TextSize = 11
	dropSubText.Alignment = fyne.TextAlignCenter

	dropContent := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(dropIcon),
		container.NewCenter(dropMainText),
		container.NewCenter(dropSubText),
		layout.NewSpacer(),
	)
	dropZone := container.NewStack(dropBg, dropContent)
	dropZone.Resize(fyne.NewSize(420, 130))

	// Info panel
	fileNameLabel := canvas.NewText("", colMuted)
	fileNameLabel.TextSize = 11
	fileNameLabel.Alignment = fyne.TextAlignCenter

	versionLabel := canvas.NewText("", colAccent)
	versionLabel.TextSize = 17
	versionLabel.TextStyle = fyne.TextStyle{Bold: true}
	versionLabel.Alignment = fyne.TextAlignCenter

	infoBg := canvas.NewRectangle(colPanel)
	infoBg.CornerRadius = 6
	infoContent := container.NewVBox(
		container.NewCenter(versionLabel),
		container.NewCenter(fileNameLabel),
	)
	infoPanel := container.NewStack(infoBg, container.NewPadded(infoContent))
	infoPanel.Hide()

	// Note label
	noteLabel := canvas.NewText("Converted file will be placed in the same folder as the original. The original .song file will stay intact.", colMuted)
	noteLabel.TextSize = 10
	noteLabel.Alignment = fyne.TextAlignCenter

	// Convert buttons — both MediumImportance, no pre-selection
	btnSO7 := widget.NewButton("Convert to Studio One 7", nil)
	btnSO6 := widget.NewButton("Convert to Studio One 6", nil)
	btnSO7.Importance = widget.MediumImportance
	btnSO6.Importance = widget.MediumImportance
	btnSO7.Hide()
	btnSO6.Hide()

	// Result label
	resultLabel := canvas.NewText("", colSuccess)
	resultLabel.TextSize = 12
	resultLabel.Alignment = fyne.TextAlignCenter

	// Browse button
	browseBtn := widget.NewButton("  Browse for .song file...  ", nil)

	// ── Logic ──

	refreshUI := func() {
		btnSO7.Hide()
		btnSO6.Hide()
		noteLabel.Hide()
		resultLabel.Text = ""
		resultLabel.Refresh()

		if len(loadedFiles) == 0 {
			infoPanel.Hide()
			return
		}

		hasV9, hasV8 := false, false
		for _, e := range loadedFiles {
			if e.version == "9" {
				hasV9 = true
			} else if e.version == "8" {
				hasV8 = true
			}
		}

		single := len(loadedFiles) == 1
		thisThese := map[bool]string{true: "This is a", false: "These are"}[single]
		suffix := map[bool]string{true: "Project", false: "Projects"}[single]

		switch {
		case hasV9 && !hasV8:
			versionLabel.Text = thisThese + " Studio Pro v8 " + suffix
			btnSO7.Show()
			btnSO6.Show()
		case hasV8 && !hasV9:
			versionLabel.Text = thisThese + " Studio One v7 " + suffix
			btnSO6.Show()
		case hasV9 && hasV8:
			versionLabel.Text = "Mixed versions detected"
			btnSO7.Show()
			btnSO6.Show()
		}

		if !single {
			noteLabel.Text = "All files will be converted at once and placed in the same folder as the original. The original .song file will stay intact."
		} else {
			noteLabel.Text = "Converted file will be placed in the same folder as the original. The original .song file will stay intact."
		}

		if single {
			fileNameLabel.Text = filepath.Base(loadedFiles[0].path)
		} else {
			fileNameLabel.Text = fmt.Sprintf("%d files loaded", len(loadedFiles))
		}
		fileNameLabel.Refresh()
		versionLabel.Refresh()
		noteLabel.Refresh()
		infoPanel.Show()
		infoPanel.Refresh()
	}

	addFiles := func(paths []string) {
		added := 0
		skipped := 0
		for _, path := range paths {
			if !strings.HasSuffix(strings.ToLower(path), ".song") {
				skipped++
				continue
			}
			version, err := readFormatVersion(path)
			if err != nil {
				skipped++
				continue
			}
			// avoid duplicates
			dupe := false
			for _, e := range loadedFiles {
				if e.path == path {
					dupe = true
					break
				}
			}
			if !dupe {
				loadedFiles = append(loadedFiles, songEntry{path: path, version: version})
				added++
			}
		}
		if skipped > 0 && added == 0 {
			resultLabel.Text = fmt.Sprintf("✗  %d file(s) could not be loaded", skipped)
			resultLabel.Color = colError
			resultLabel.Refresh()
		}
		refreshUI()
	}

	doConvert := func(targetVersion, suffix string) {
		if len(loadedFiles) == 0 {
			return
		}
		converted, skipped := 0, 0
		var lastOut string
		for _, e := range loadedFiles {
			// In mixed mode, skip v8 files when converting to SO7
			if e.version == "8" && targetVersion == "8" {
				skipped++
				continue
			}
			out, err := convertSong(e.path, e.version, targetVersion, suffix)
			if err != nil {
				resultLabel.Text = "✗  Error converting " + filepath.Base(e.path) + ": " + err.Error()
				resultLabel.Color = colError
				resultLabel.Refresh()
				return
			}
			lastOut = out
			converted++
		}
		if converted == 1 && skipped == 0 {
			resultLabel.Text = "✓  Saved: " + filepath.Base(lastOut)
		} else if skipped > 0 {
			resultLabel.Text = fmt.Sprintf("✓  %d converted, %d skipped (already at target version)", converted, skipped)
		} else {
			resultLabel.Text = fmt.Sprintf("✓  %d file(s) converted", converted)
		}
		resultLabel.Color = colSuccess
		resultLabel.Refresh()
	}

	btnSO7.OnTapped = func() { doConvert("8", "SO7") }
	btnSO6.OnTapped = func() { doConvert("7", "SO6") }

	browseBtn.OnTapped = func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			addFiles([]string{reader.URI().Path()})
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".song"}))
		fd.Show()
	}

	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		paths := make([]string, 0, len(uris))
		for _, u := range uris {
			paths = append(paths, u.Path())
		}
		addFiles(paths)
	})

	// ── Layout ──
	buttonsRow := container.NewGridWithColumns(2, btnSO7, btnSO6)

	content := container.NewVBox(
		container.NewPadded(dropZone),
		container.NewCenter(browseBtn),
		widget.NewSeparator(),
		container.NewPadded(infoPanel),
		container.NewPadded(buttonsRow),
		container.NewCenter(noteLabel),
		container.NewCenter(resultLabel),
	)

	w.SetContent(container.NewPadded(content))
	w.CenterOnScreen()
	w.ShowAndRun()
}
