package main

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"math"
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

//go:embed icon.png
var iconBytes []byte

// ── UUIDs ─────────────────────────────────────────────────────────────────────

const (
	proEQ81Class = "D9AE9ACD-69B4-4B43-B8D5-983E39C559A5"
	proEQCID     = "073C4094-E062-4FB5-8328-74608DD1A3A4"
	comp81Class  = "36F3F4D1-CBB4-4BF7-A7E3-EBCCED53718B"
	compCID      = "54F19B72-352C-4AA5-A2AF-67F86F30D6BE"
)

// ── Song classification ───────────────────────────────────────────────────────

type songEntry struct {
	path  string
	class string // "81", "80", "7"
}

func classifySong(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("not a valid .song file")
	}
	defer r.Close()

	var formatVersion string
	var hasComp81 bool

	for _, f := range r.File {
		switch f.Name {
		case "metainfo.xml":
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			re := regexp.MustCompile(`<Attribute id="Document:FormatVersion" value="(\d+)"/>`)
			if m := re.FindSubmatch(data); m != nil {
				formatVersion = string(m[1])
			}
		case "Devices/audiomixer.xml":
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			data, _ := io.ReadAll(rc)
			rc.Close()
			if bytes.Contains(data, []byte(comp81Class)) {
				hasComp81 = true
			}
		}
	}

	switch formatVersion {
	case "9":
		if hasComp81 {
			return "81", nil
		}
		return "80", nil
	case "8":
		return "7", nil
	default:
		return "", fmt.Errorf("unrecognised FormatVersion %q", formatVersion)
	}
}

// ── Compressor JSON → XML (verified against working v7 references) ──────────

func convertCompressorJSON(data map[string]interface{}) []byte {
	p, _ := data["parameters"].(map[string]interface{})
	sc, _ := p["studiocomp"].(map[string]interface{})

	get := func(k string, def float64) float64 {
		if v, ok := sc[k]; ok {
			if f, ok := v.(float64); ok {
				return f
			}
		}
		return def
	}

	ratio := get("ratio", 2.0)
	xmlRatio := 0.0
	if ratio != 0 {
		xmlRatio = 1.0 - (1.0 / ratio)
	}
	xmlAttack := get("attack", 15.0) / 1000.0
	xmlRelease := get("release", 120.0) / 1000.0
	ingainDB := get("ingain", 0.0)
	xmlIngain := math.Pow(10.0, ingainDB/20.0)

	threshold := get("threshold", -10.0)
	knee := get("knee", 6.0)
	gain := get("gain", 0.0)
	autospeed := get("autospeed", 0.0)
	auto := int(get("autogain", 0.0))
	adaptive := int(get("dualband", 0.0))
	linked := int(get("linked", 1.0))
	lookahead := int(get("lookahead", 1.0))
	resetmin := int(get("resetmin", 0.0))
	mix := get("mix", 1.0)
	isc := int(get("internalsidechain", 0.0))
	scl := int(get("sidechainlisten", 0.0))
	scfl := get("sidechainfreqlow", 20.0)
	scfh := get("sidechainfreqhigh", 16000.0)
	swap := int(get("swapfreqs", 0.0))

	return []byte(fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
			"<AudioEffectPreset cid=\"{%s}\" version=\"1\" algorithmVersion=\"1\">\n"+
			"\t<Attributes x:id=\"ParameterData\" linked=\"%d\" lookAhead=\"%d\" resetmin=\"%d\" mix=\"%v\" ratio=\"%v\" threshold=\"%v\"\n"+
			"\t            knee=\"%v\" auto=\"%d\" gain=\"%v\" ingain=\"%v\" autospeed=\"%v\" attack=\"%v\"\n"+
			"\t            release=\"%v\" adaptive=\"%d\" internalsidechain=\"%d\" sidechainlisten=\"%d\"\n"+
			"\t            sidechainfreqlow=\"%v\" sidechainfreqhigh=\"%v\" swapfreqs=\"%d\"/>\n"+
			"</AudioEffectPreset>\n",
		compCID, linked, lookahead, resetmin, mix, xmlRatio, threshold,
		knee, auto, gain, xmlIngain, autospeed, xmlAttack,
		xmlRelease, adaptive, isc, scl, scfl, scfh, swap))
}

// ── Pro EQ JSON → XML (verified against working v7 references) ──────────────

func convertProEQJSON(data map[string]interface{}) []byte {
	flat := map[string]float64{}
	if params, ok := data["parameters"].(map[string]interface{}); ok {
		for _, section := range params {
			if sec, ok := section.(map[string]interface{}); ok {
				for k, v := range sec {
					if f, ok := v.(float64); ok {
						flat[k] = f
					}
				}
			}
		}
	}
	get := func(k string, def float64) float64 {
		if v, ok := flat[k]; ok {
			return v
		}
		return def
	}

	legacyAuto := 0.0
	if comp, ok := data["component"].(map[string]interface{}); ok {
		if agc, ok := comp["autogaincomponent"].(map[string]interface{}); ok {
			if v, ok := agc["legacyAutoGain"].(float64); ok {
				legacyAuto = v
			}
		}
	}

	gainDB := get("gain", 0.0)
	xmlGain := math.Pow(10.0, gainDB/20.0)
	autogain2 := get("autogain", 1.0)

	fields := []string{
		"linearphasesoft", "linearphasefreq", "linearphaseactive",
		"lcfreq", "lcslope", "lcactive",
		"lfgain", "lffreq", "lfq", "lftype", "lfactive", "lfdynamic", "lfdynthreshold", "lfdynrange", "lfsolo",
		"lmfgain", "lmffreq", "lmfq", "lmfactive", "lmfdynamic", "lmfdynthreshold", "lmfdynrange", "lmfsolo",
		"mfgain", "mffreq", "mfq", "mfactive", "mfdynamic", "mfdynthreshold", "mfdynrange", "mfsolo",
		"hmfgain", "hmffreq", "hmfq", "hmfactive", "hmfdynamic", "hmfdynthreshold", "hmfdynrange", "hmfsolo",
		"hfgain", "hffreq", "hfq", "hftype", "hfactive", "hfdynamic", "hfdynthreshold", "hfdynrange", "hfsolo",
		"hcfreq", "hcslope", "hcactive",
		"highqual", "showfft", "analyzerRangeMin", "analyzerRangeMax", "viewmode",
		"showControls", "showDynamics", "displayRange",
	}

	fmtNum := func(v float64) string {
		if v == math.Trunc(v) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	}

	var attrs strings.Builder
	for _, k := range fields {
		attrs.WriteString(fmt.Sprintf(`%s="%s" `, k, fmtNum(get(k, 0.0))))
	}

	return []byte(fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
			"<AudioEffectPreset cid=\"{%s}\" version=\"1\" algorithmVersion=\"1\">\n"+
			"\t<Attributes x:id=\"ParameterData\" %sgain=\"%s\" autogain2=\"%s\" autogain=\"%s\"/>\n"+
			"</AudioEffectPreset>\n",
		proEQCID, attrs.String(), fmtNum(xmlGain), fmtNum(autogain2), fmtNum(legacyAuto)))
}

// ── Main conversion: fix Pro EQ (always, for FormatVersion 9) + Compressor (8.1 only) ──

func fixProEQAndCompressor(files map[string][]byte, fixCompressor bool) (renamed int, warnings []string) {
	cidRe := regexp.MustCompile(`cid="\{([0-9A-Fa-f-]{36})\}"`)
	renameMap := map[string]string{} // old path -> new path

	for name, content := range files {
		if !strings.HasSuffix(name, ".dsppreset") {
			continue
		}
		trimmed := bytes.TrimLeft(content, "\xef\xbb\xbf \t\r\n")

		if bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<AudioEffectPreset")) {
			m := cidRe.FindSubmatch(trimmed)
			if m == nil {
				continue
			}
			cid := strings.ToUpper(string(m[1]))
			isEQ := cid == proEQCID
			isComp := cid == compCID
			if isEQ || (isComp && fixCompressor) {
				newName := strings.TrimSuffix(name, ".dsppreset") + ".fxpreset"
				renameMap[name] = newName
			}
		} else if bytes.HasPrefix(trimmed, []byte("{")) {
			var data map[string]interface{}
			if err := json.Unmarshal(trimmed, &data); err != nil {
				warnings = append(warnings, name+" (JSON parse error)")
				continue
			}
			classname, _ := data["classname"].(string)
			switch {
			case classname == "Compressor" && fixCompressor:
				newName := strings.TrimSuffix(name, ".dsppreset") + ".fxpreset"
				files[name] = convertCompressorJSON(data)
				renameMap[name] = newName
			case classname == "Pro EQ":
				newName := strings.TrimSuffix(name, ".dsppreset") + ".fxpreset"
				files[name] = convertProEQJSON(data)
				renameMap[name] = newName
			default:
				// All other classnames (Fat Channel, Ampire, De-Esser, etc.) are
				// natively JSON .dsppreset in v7 too — no action, no warning needed.
			}
		}
	}

	// Apply renames to the files map
	for oldName, newName := range renameMap {
		files[newName] = files[oldName]
		delete(files, oldName)
	}
	renamed = len(renameMap)

	// Patch audiomixer.xml
	const mixerPath = "Devices/audiomixer.xml"
	if mixerData, ok := files[mixerPath]; ok {
		text := string(mixerData)

		text = strings.ReplaceAll(text, proEQ81Class, proEQCID)
		if fixCompressor {
			text = strings.ReplaceAll(text, comp81Class, compCID)
		}

		knownCIDs := []string{proEQCID}
		if fixCompressor {
			knownCIDs = append(knownCIDs, compCID)
		}
		presetPattern := regexp.MustCompile(
			`(?s)(<Attributes x:id="ghostData" presetType=")dsppreset(">\s*` +
				`<Attributes x:id="classInfo" classID="\{(?:` + strings.Join(knownCIDs, "|") + `)\}")`)
		text = presetPattern.ReplaceAllString(text, "${1}fxpreset${2}")

		if fixCompressor {
			subPattern := regexp.MustCompile(
				`(?s)(classID="\{` + compCID + `\}" name="Compressor".*?subCategory=")\(Native\)/Mixing(")`)
			text = subPattern.ReplaceAllString(text, "${1}(Native)/Dynamics${2}")
		}

		for oldName, newName := range renameMap {
			oldRel := strings.TrimPrefix(oldName, "")
			newRel := strings.TrimPrefix(newName, "")
			text = strings.ReplaceAll(text, oldRel, newRel)
		}

		files[mixerPath] = []byte(text)
	}

	return renamed, warnings
}

func convertSong(entry songEntry, suffix string) (string, error) {
	r, err := zip.OpenReader(entry.path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	dir := filepath.Dir(entry.path)
	base := strings.TrimSuffix(filepath.Base(entry.path), ".song")
	outputPath := filepath.Join(dir, base+"_"+suffix+".song")

	var targetFV string
	switch suffix {
	case "SO6":
		targetFV = "7"
	case "SO7":
		targetFV = "8"
	case "80":
		targetFV = "9"
	}

	// ── Pass 1: read every file into memory, preserving order ──
	order := make([]string, 0, len(r.File))
	files := make(map[string][]byte, len(r.File))
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
		order = append(order, f.Name)
		files[f.Name] = data
	}

	// ── Pass 2: patch metainfo.xml ──
	if data, ok := files["metainfo.xml"]; ok {
		re := regexp.MustCompile(`<Attribute id="Document:FormatVersion" value="(\d+)"/>`)
		if m := re.FindSubmatch(data); m != nil && string(m[1]) != targetFV {
			old := fmt.Sprintf(`<Attribute id="Document:FormatVersion" value="%s"/>`, string(m[1]))
			neu := fmt.Sprintf(`<Attribute id="Document:FormatVersion" value="%s"/>`, targetFV)
			files["metainfo.xml"] = bytes.ReplaceAll(data, []byte(old), []byte(neu))
		}
	}

	// ── Pass 3: Pro EQ (always, for any FormatVersion 9 source) + Compressor (8.1 source only) ──
	if entry.class == "81" || entry.class == "80" {
		fixCompressor := entry.class == "81"
		renamedCount, _ := fixProEQAndCompressor(files, fixCompressor)
		// Update the order list for any renamed files
		if renamedCount > 0 {
			newOrder := make([]string, 0, len(order))
			for _, name := range order {
				if _, stillExists := files[name]; stillExists {
					newOrder = append(newOrder, name)
				} else {
					// find its replacement (same base name with .fxpreset)
					candidate := strings.TrimSuffix(name, ".dsppreset") + ".fxpreset"
					if _, ok := files[candidate]; ok {
						newOrder = append(newOrder, candidate)
					}
				}
			}
			order = newOrder
		}
	}

	// ── Write output zip in original order ──
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range order {
		fw, err := w.Create(name)
		if err != nil {
			return "", err
		}
		if _, err := fw.Write(files[name]); err != nil {
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
	a.SetIcon(fyne.NewStaticResource("icon.png", iconBytes))

	w := a.NewWindow("Studio One Song Converter")
	w.Resize(fyne.NewSize(420, 520))

	var loadedFiles []songEntry

	// ── Drop zone ──
	dropBg := canvas.NewRectangle(colPanel)
	dropBg.CornerRadius = 6
	dropMainText := canvas.NewText("Drop .song file(s) here", colText)
	dropMainText.TextSize = 14
	dropMainText.Alignment = fyne.TextAlignCenter
	dropSubText := canvas.NewText("or use the Browse button below", colMuted)
	dropSubText.TextSize = 11
	dropSubText.Alignment = fyne.TextAlignCenter
	dropZone := container.NewStack(dropBg, container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(widget.NewIcon(theme.UploadIcon())),
		container.NewCenter(dropMainText),
		container.NewCenter(dropSubText),
		layout.NewSpacer(),
	))
	dropZone.Resize(fyne.NewSize(380, 130))

	// ── Info panel ──
	versionLabel := canvas.NewText("", colAccent)
	versionLabel.TextSize = 17
	versionLabel.TextStyle = fyne.TextStyle{Bold: true}
	versionLabel.Alignment = fyne.TextAlignCenter
	fileNameLabel := canvas.NewText("", colMuted)
	fileNameLabel.TextSize = 11
	fileNameLabel.Alignment = fyne.TextAlignCenter
	infoBg := canvas.NewRectangle(colPanel)
	infoBg.CornerRadius = 6
	infoPanel := container.NewStack(infoBg, container.NewPadded(container.NewVBox(
		container.NewCenter(versionLabel),
		container.NewCenter(fileNameLabel),
	)))
	infoPanel.Hide()

	// ── Buttons ──
	btnSO80 := widget.NewButton("Convert to Studio Pro 8.0", nil)
	btnSO7 := widget.NewButton("Convert to Studio One 7", nil)
	btnSO6 := widget.NewButton("Convert to Studio One 6", nil)
	btnSO80.Importance = widget.MediumImportance
	btnSO7.Importance = widget.MediumImportance
	btnSO6.Importance = widget.MediumImportance
	btnSO80.Hide()
	btnSO7.Hide()
	btnSO6.Hide()

	noteLabel := canvas.NewText("", colMuted)
	noteLabel.TextSize = 10
	noteLabel.Alignment = fyne.TextAlignCenter

	resultLabel := canvas.NewText("", colSuccess)
	resultLabel.TextSize = 12
	resultLabel.Alignment = fyne.TextAlignCenter

	browseBtn := widget.NewButton("  Browse for .song file...  ", nil)

	// ── Refresh UI ──
	refreshUI := func() {
		btnSO80.Hide()
		btnSO7.Hide()
		btnSO6.Hide()
		resultLabel.Text = ""
		resultLabel.Refresh()

		if len(loadedFiles) == 0 {
			infoPanel.Hide()
			noteLabel.Text = ""
			noteLabel.Refresh()
			return
		}

		has81, has80, hasV7 := false, false, false
		for _, e := range loadedFiles {
			switch e.class {
			case "81":
				has81 = true
			case "80":
				has80 = true
			case "7":
				hasV7 = true
			}
		}

		single := len(loadedFiles) == 1
		mixed := (has81 && has80) || (has81 && hasV7) || (has80 && hasV7)

		switch {
		case mixed:
			versionLabel.Text = "Mixed versions detected"
			btnSO80.Show()
			btnSO7.Show()
			btnSO6.Show()
		case has81:
			if single {
				versionLabel.Text = "Studio Pro v8.1 Project"
			} else {
				versionLabel.Text = "Studio Pro v8.1 Projects"
			}
			btnSO80.Show()
			btnSO7.Show()
			btnSO6.Show()
		case has80:
			if single {
				versionLabel.Text = "Studio Pro v8.0 Project"
			} else {
				versionLabel.Text = "Studio Pro v8.0 Projects"
			}
			btnSO7.Show()
			btnSO6.Show()
		case hasV7:
			if single {
				versionLabel.Text = "Studio One v7 Project"
			} else {
				versionLabel.Text = "Studio One v7 Projects"
			}
			btnSO6.Show()
		}

		if single {
			fileNameLabel.Text = filepath.Base(loadedFiles[0].path)
			noteLabel.Text = "Converted file will be placed in the same folder as the original. The original .song file will stay intact."
		} else {
			fileNameLabel.Text = fmt.Sprintf("%d files loaded", len(loadedFiles))
			noteLabel.Text = "All files will be converted at once and placed in the same folder as the original. The original .song file will stay intact."
		}

		fileNameLabel.Refresh()
		versionLabel.Refresh()
		noteLabel.Refresh()
		infoPanel.Show()
		infoPanel.Refresh()
	}

	// ── Add files ──
	addFiles := func(paths []string) {
		skipped := 0
		for _, path := range paths {
			if !strings.HasSuffix(strings.ToLower(path), ".song") {
				skipped++
				continue
			}
			class, err := classifySong(path)
			if err != nil || class == "" {
				skipped++
				continue
			}
			dupe := false
			for _, e := range loadedFiles {
				if e.path == path {
					dupe = true
					break
				}
			}
			if !dupe {
				loadedFiles = append(loadedFiles, songEntry{path: path, class: class})
			}
		}
		if skipped > 0 && len(loadedFiles) == 0 {
			resultLabel.Text = fmt.Sprintf("✗  %d file(s) could not be loaded", skipped)
			resultLabel.Color = colError
			resultLabel.Refresh()
		}
		refreshUI()
	}

	// ── Convert ──
	doConvert := func(suffix string) {
		if len(loadedFiles) == 0 {
			return
		}
		converted, skipped := 0, 0
		var lastOut string

		for _, e := range loadedFiles {
			// Skip files already at or below the target
			if suffix == "80" && (e.class == "80" || e.class == "7") {
				skipped++
				continue
			}
			if suffix == "SO7" && e.class == "7" {
				skipped++
				continue
			}
			out, err := convertSong(e, suffix)
			if err != nil {
				resultLabel.Text = "✗  " + filepath.Base(e.path) + ": " + err.Error()
				resultLabel.Color = colError
				resultLabel.Refresh()
				return
			}
			lastOut = out
			converted++
		}

		if converted == 0 {
			resultLabel.Text = "✗  No files needed conversion."
			resultLabel.Color = colError
		} else if converted == 1 && skipped == 0 {
			resultLabel.Text = "✓  Saved: " + filepath.Base(lastOut)
			resultLabel.Color = colSuccess
		} else if skipped > 0 {
			resultLabel.Text = fmt.Sprintf("✓  %d converted, %d skipped (already at target version)", converted, skipped)
			resultLabel.Color = colSuccess
		} else {
			resultLabel.Text = fmt.Sprintf("✓  %d file(s) converted", converted)
			resultLabel.Color = colSuccess
		}
		resultLabel.Refresh()
	}

	btnSO80.OnTapped = func() { doConvert("80") }
	btnSO7.OnTapped = func() { doConvert("SO7") }
	btnSO6.OnTapped = func() { doConvert("SO6") }

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
	buttonsRow := container.NewVBox(btnSO80, btnSO7, btnSO6)
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
