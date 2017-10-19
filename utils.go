package canfriend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

type DataDisplayType int

const (
	DisplayRaw DataDisplayType = iota
	DisplayU8
	DisplayU16LE
	DisplayU16BE
	DisplayU32LE
	DisplayU32BE
	DisplayS32
	DisplayASCII
)

func PrettifyFrameData(summary *FrameSummary, highlightDiff bool, displayType DataDisplayType) string {
	var outputs []string

	changed := color.New(color.FgRed, color.Bold)

	switch displayType {
	case DisplayRaw, DisplayU8:
		for i, b := range summary.LatestFrame.Data {
			fmtstr := "%02X"

			if displayType == DisplayU8 {
				fmtstr = "%03d"
			}

			segment := fmt.Sprintf(fmtstr, b)

			if highlightDiff {
				if summary.PreviousFrame != nil {
					if len(summary.LatestFrame.Data) >= len(summary.PreviousFrame.Data) {
						if summary.PreviousFrame.Data[i] != b {
							segment = changed.Sprintf(fmtstr, b)
						}
					}
				}
			}

			outputs = append(outputs, segment)
		}

	case DisplayU16LE, DisplayU16BE, DisplayU32LE, DisplayU32BE, DisplayS32:
		var byteOrder binary.ByteOrder

		// defaults
		byteOrder = binary.LittleEndian
		bsz := 2
		fmtstr := "% 6d"

		switch displayType {
		case DisplayU16BE, DisplayU32BE:
			byteOrder = binary.BigEndian
		}

		switch displayType {
		case DisplayU32LE, DisplayU32BE, DisplayS32:
			bsz = 4
			fmtstr = "% 11d"
		}

		for i := 0; i < len(summary.LatestFrame.Data); i += bsz {
			if (i + bsz) <= len(summary.LatestFrame.Data) {
				data := []byte(summary.LatestFrame.Data[i:(i + bsz)])

				var value interface{}

				switch bsz {
				case 2:
					value = byteOrder.Uint16(data)
				case 4:
					if displayType == DisplayS32 {
						if v, err := binary.ReadVarint(bytes.NewReader(data)); err == nil {
							value = v
						} else {
							outputs = append(outputs, strings.Repeat(`?`, 10))
						}
					} else {
						value = byteOrder.Uint32(data)
					}
				}

				if value != nil {
					outputs = append(outputs, fmt.Sprintf(fmtstr, value))
				}
			}
		}

	case DisplayASCII:
		ascii := ``

		for _, b := range summary.LatestFrame.Data {
			// this range covers "printable" characters
			switch {
			case b > 32 && b < 127:
				ascii += string(rune(b))
			default:
				ascii += `.`
			}
		}

		outputs = append(outputs, ascii)

	default:
		outputs = []string{`-`}
	}

	return strings.Join(outputs, ` `)
}
