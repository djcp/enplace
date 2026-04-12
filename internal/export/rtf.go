package export

import (
	"fmt"
	"strings"

	"github.com/djcp/enplace/internal/models"
	"github.com/djcp/enplace/internal/scaling"
)

// ToRTF renders a recipe as an RTF 1.x document (cp1252 encoding).
func ToRTF(r *models.Recipe, opts Options) string {
	ren := &rtfRenderer{}
	b, _ := RenderRecipe(r, opts, ren)
	return string(b)
}

type rtfRenderer struct {
	sb strings.Builder
}

func (r *rtfRenderer) Title(name string) {
	// \ansicpg1252 declares the default code page explicitly so RTF readers
	// use cp1252 rather than the system default when interpreting \'XX escapes.
	r.sb.WriteString("{\\rtf1\\ansi\\ansicpg1252\\deff0\n")
	r.sb.WriteString("{\\fonttbl{\\f0\\fswiss Helvetica;}}\n")
	// cf1=terracotta  cf2=sage green  cf3=warm gray  cf4=50% gray (attribution)
	r.sb.WriteString("{\\colortbl;\\red201\\green100\\blue66;\\red124\\green158\\blue110;\\red142\\green129\\blue120;\\red128\\green128\\blue128;}\n")
	r.sb.WriteString("\\f0\\fs22\n")
	r.sb.WriteString(fmt.Sprintf("{\\fs36\\b\\cf1 %s\\cf0\\b0\\par}\n", rtfEnc(name)))
	r.sb.WriteString("\\par\n")
}

func (r *rtfRenderer) Meta(timingSummary string, _, _ *int, servings *int, servingUnits string) {
	var parts []string
	if timingSummary != "" {
		parts = append(parts, timingSummary)
	}
	if servings != nil && *servings > 0 {
		units := servingUnits
		if units == "" {
			units = "servings"
		}
		parts = append(parts, formatServings(*servings, units))
	}
	if len(parts) > 0 {
		r.sb.WriteString(fmt.Sprintf("{\\fs20\\cf3 %s\\cf0\\par}\n", rtfEnc(strings.Join(parts, "  \u00b7  "))))
	}
}

func (r *rtfRenderer) Hydration(pct float64, totalGrams int, starterAssumed bool) {
	line := fmt.Sprintf("Hydration: %.1f%%  \u00b7  %dg total", pct, totalGrams)
	if starterAssumed {
		line += "  (100% hydration starter assumed)"
	}
	r.sb.WriteString(fmt.Sprintf("{\\fs20\\b\\cf1 %s\\cf0\\b0\\par}\n", rtfEnc(line)))
}

func (r *rtfRenderer) Description(text string) {
	r.sb.WriteString(fmt.Sprintf("{\\fs22\\i %s\\i0\\par}\n", rtfEnc(text)))
	r.sb.WriteString("\\par\n")
}

func (r *rtfRenderer) TagLine(ctxLabel, joined string) {
	r.sb.WriteString(fmt.Sprintf("{\\fs18\\cf3 %s: %s\\cf0\\par}\n", rtfEnc(ctxLabel), rtfEnc(joined)))
}

func (r *rtfRenderer) IngredientsHeader() {
	r.sb.WriteString("\\par\n")
	r.sb.WriteString("{\\fs26\\b\\cf2 Ingredients\\cf0\\b0\\par}\n")
	r.sb.WriteString("\\par\n")
}

func (r *rtfRenderer) IngredientSection(section string) {
	r.sb.WriteString(fmt.Sprintf("{\\fs22\\b %s\\b0\\par}\n", rtfEnc(section)))
}

func (r *rtfRenderer) Ingredient(display string) {
	r.sb.WriteString(fmt.Sprintf("{\\fs22 - %s\\par}\n", rtfEnc(display)))
}

func (r *rtfRenderer) DirectionsHeader() {
	r.sb.WriteString("\\par\n")
	r.sb.WriteString("{\\fs26\\b\\cf2 Directions\\cf0\\b0\\par}\n")
	r.sb.WriteString("\\par\n")
}

func (r *rtfRenderer) Directions(text string) {
	r.sb.WriteString(fmt.Sprintf("{\\fs22 %s\\par}\n", rtfEnc(text)))
	r.sb.WriteString("\\par\n")
}

func (r *rtfRenderer) SourceURL(url string) {
	r.sb.WriteString(fmt.Sprintf("{\\fs18\\cf3 Source: %s\\cf0\\par}\n", rtfEnc(url)))
}

func (r *rtfRenderer) BreadMetricsTable(perIngredient []scaling.IngredientBakerPct, hydrationPct float64, starterAssumed bool) {
	if len(perIngredient) == 0 {
		return
	}
	r.sb.WriteString("\\par\n")
	r.sb.WriteString("{\\fs26\\b\\cf1 Baker\\u8217?s Percentages\\cf0\\b0\\par}\n")
	r.sb.WriteString("\\par\n")
	// \tqr\tx2000 right-aligns the weight at 2000 twips; \tqr\tx3000 right-aligns % at 3000.
	for _, ing := range perIngredient {
		name := ing.Name
		if ing.Type == "starter" {
			name += "*"
		}
		grams := int(ing.WeightGrams + 0.5)
		line := fmt.Sprintf("%s\t%dg\t%.1f%%", name, grams, ing.Percentage)
		r.sb.WriteString(fmt.Sprintf(
			"{\\pard\\tqr\\tx2000\\tqr\\tx3000\\fs20 %s\\par}\n",
			rtfEnc(line),
		))
	}
	totalG := 0
	for _, ing := range perIngredient {
		totalG += int(ing.WeightGrams + 0.5)
	}
	r.sb.WriteString("\\par\n")
	hydLine := fmt.Sprintf("Hydration: %.1f%%  \u00b7  %dg total", hydrationPct, totalG)
	r.sb.WriteString(fmt.Sprintf("{\\fs20\\b\\cf1 %s\\cf0\\b0\\par}\n", rtfEnc(hydLine)))
	if starterAssumed {
		r.sb.WriteString("{\\fs18\\cf3 * 100% hydration starter assumed\\cf0\\par}\n")
	}
}

func (r *rtfRenderer) Footer(credits, versionStr string) {
	// \tqr\tx9360 places a right-aligned tab stop at 9360 twips (6.5", the text
	// width of a Letter page with standard 1-inch margins). \tab jumps to it so
	// the version lands flush-right while credits sit flush-left.
	if credits != "" {
		r.sb.WriteString(fmt.Sprintf(
			"{\\pard\\tqr\\tx9360\\fs16\\cf3 %s\\cf4\\tab %s\\cf0\\par}\n",
			rtfEnc(credits),
			rtfEnc(versionStr),
		))
	} else {
		r.sb.WriteString(fmt.Sprintf("{\\pard\\qr\\fs16\\cf4 %s\\cf0\\par}\n",
			rtfEnc(versionStr)))
	}
	r.sb.WriteString("}\n")
}

func (r *rtfRenderer) Result() ([]byte, error) {
	return []byte(r.sb.String()), nil
}

// cp1252Special maps Unicode code points in the U+0080–U+009F range that
// cp1252 assigns to printable characters (unlike Latin-1, which uses them for
// control codes) to their cp1252 byte values.
var cp1252Special = map[rune]byte{
	0x20AC: 0x80, // €
	0x201A: 0x82, // ‚
	0x0192: 0x83, // ƒ
	0x201E: 0x84, // „
	0x2026: 0x85, // …
	0x2020: 0x86, // †
	0x2021: 0x87, // ‡
	0x02C6: 0x88, // ˆ
	0x2030: 0x89, // ‰
	0x0160: 0x8A, // Š
	0x2039: 0x8B, // ‹
	0x0152: 0x8C, // Œ
	0x017D: 0x8E, // Ž
	0x2018: 0x91, // '
	0x2019: 0x92, // '
	0x201C: 0x93, // "
	0x201D: 0x94, // "
	0x2022: 0x95, // •
	0x2013: 0x96, // –
	0x2014: 0x97, // —
	0x02DC: 0x98, // ˜
	0x2122: 0x99, // ™
	0x0161: 0x9A, // š
	0x203A: 0x9B, // ›
	0x0153: 0x9C, // œ
	0x017E: 0x9E, // ž
	0x0178: 0x9F, // Ÿ
}

// rtfEnc encodes a UTF-8 string for safe embedding in an RTF 1.x document
// declared with \ansi\ansicpg1252. It handles characters as follows:
//
//   - RTF metacharacters (\, {, }): escaped as \\, \{, \}
//   - Newlines (\n): converted to \par (paragraph break)
//   - \r: dropped (line endings are handled by \n)
//   - Other control characters (< 0x20): dropped
//   - ASCII printable (0x20–0x7E): passed through unchanged
//   - Latin-1 supplement (U+00A0–U+00FF): \'XX, same byte value as cp1252
//   - cp1252 special range (e.g. •, –, °, curly quotes): \'XX via cp1252Special
//   - Everything else: \uN? (signed 16-bit decimal Unicode escape, fallback '?')
func rtfEnc(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r == '\\':
			sb.WriteString(`\\`)
		case r == '{':
			sb.WriteString(`\{`)
		case r == '}':
			sb.WriteString(`\}`)
		case r == '\n':
			sb.WriteString(`\par` + "\n")
		case r == '\r':
			// dropped; \n handles line endings
		case r < 0x20:
			// drop other control characters
		case r < 0x80:
			sb.WriteRune(r)
		case r >= 0xA0 && r <= 0xFF:
			// Latin-1 supplement: code point equals cp1252 byte value
			fmt.Fprintf(&sb, "\\'%02x", byte(r))
		default:
			if b, ok := cp1252Special[r]; ok {
				fmt.Fprintf(&sb, "\\'%02x", b)
			} else {
				// RTF Unicode escape: signed 16-bit decimal with '?' fallback
				n := int32(r)
				if n > 32767 {
					n -= 65536
				}
				fmt.Fprintf(&sb, `\u%d?`, n)
			}
		}
	}
	return sb.String()
}
