package main

import (
	"fmt"
	"strings"
	"time"

	tabwriter "github.com/NonerKao/color-aware-tabwriter"

	"github.com/fatih/color"
	"github.com/jroimartin/gocui"
)

var DefaultSummaryRefreshInterval = 100 * time.Millisecond

type AnalyzerUI struct {
	SummaryRefreshInterval time.Duration
	analyzer               *Analyzer
	gui                    *gocui.Gui
	summarySortBy          SortKey
	sortReverse            bool
	pauseUpdates           bool
}

func NewAnalyzerUI(analyzer *Analyzer) *AnalyzerUI {
	return &AnalyzerUI{
		SummaryRefreshInterval: DefaultSummaryRefreshInterval,
		analyzer:               analyzer,
		summarySortBy:          SortByLastSeen,
		sortReverse:            true,
	}
}

func (self *AnalyzerUI) Run() error {
	if g, err := gocui.NewGui(gocui.OutputNormal); err == nil {
		self.gui = g
	} else {
		return err
	}

	defer self.gui.Close()
	go self.startEventGroupPoller()

	self.gui.SetManagerFunc(self.layout)

	if err := self.setupKeybindings(); err != nil {
		return err
	}

	if err := self.gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}

	return nil
}

func (self *AnalyzerUI) setupKeybindings() error {
	if err := self.gui.SetKeybinding(``, gocui.KeyCtrlC, gocui.ModNone, self.quit); err != nil {
		return err
	}

	if err := self.gui.SetKeybinding(``, gocui.KeyCtrlL, gocui.ModNone, func(_ *gocui.Gui, _ *gocui.View) error {
		self.analyzer.ClearSummary()
		self.gui.Update(self.updateSummaryView)
		return nil
	}); err != nil {
		return err
	}

	if err := self.gui.SetKeybinding(``, 's', gocui.ModNone, func(_ *gocui.Gui, _ *gocui.View) error {
		switch self.summarySortBy {
		case SortByID:
			self.summarySortBy = SortByCount
		case SortByCount:
			self.summarySortBy = SortByLastSeen
		case SortByLastSeen:
			self.summarySortBy = SortByLength
		case SortByLength:
			self.summarySortBy = SortByID
		}

		self.gui.Update(self.updateSummaryView)
		return nil
	}); err != nil {
		return err
	}

	if err := self.gui.SetKeybinding(``, 'S', gocui.ModNone, func(_ *gocui.Gui, _ *gocui.View) error {
		self.sortReverse = (!self.sortReverse)
		self.gui.Update(self.updateSummaryView)
		return nil
	}); err != nil {
		return err
	}

	if err := self.gui.SetKeybinding(``, '+', gocui.ModNone, func(_ *gocui.Gui, _ *gocui.View) error {
		self.analyzer.FrameSummaryLimit += 1
		return nil
	}); err != nil {
		return err
	}

	if err := self.gui.SetKeybinding(``, '-', gocui.ModNone, func(_ *gocui.Gui, _ *gocui.View) error {
		if self.analyzer.FrameSummaryLimit > 1 {
			self.analyzer.FrameSummaryLimit -= 1
		}
		return nil
	}); err != nil {
		return err
	}

	if err := self.gui.SetKeybinding(``, 'p', gocui.ModNone, func(_ *gocui.Gui, _ *gocui.View) error {
		self.pauseUpdates = (!self.pauseUpdates)
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (self *AnalyzerUI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if _, err := g.SetView(`summary`, 0, 0, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
	}

	return nil
}

func (self *AnalyzerUI) quit(_ *gocui.Gui, _ *gocui.View) error {
	return gocui.ErrQuit
}

func (self *AnalyzerUI) summaryHeader() []string {
	return []string{
		self.h(`summary`, string(SortByID)),
		self.h(`summary`, string(SortByCount)),
		self.h(`summary`, string(SortByLastSeen)),
		self.h(`summary`, string(SortByLength)),
		`RAW`,
		`U8`,
		`U16LE`,
		`U16BE`,
		`U32LE`,
		`U32BE`,
		`S32`,
		`ASCII`,
	}
}

func (self *AnalyzerUI) h(view string, in string) string {
	sortcolor := color.New(color.Bold)
	sortSuffix := ` `

	if self.sortReverse {
		sortSuffix += "\u25bc"
	} else {
		sortSuffix += "\u25b2"
	}

	switch view {
	case `summary`:
		if in == string(self.summarySortBy) {
			return sortcolor.Sprintf("%v%v", strings.ToUpper(in), sortSuffix)
		}
	}

	return strings.ToUpper(in) + `  `
}

func (self *AnalyzerUI) startEventGroupPoller() {
	for {
		self.gui.Update(self.updateSummaryView)
		time.Sleep(self.SummaryRefreshInterval)
	}
}

func (self *AnalyzerUI) updateSummaryView(g *gocui.Gui) error {
	if self.pauseUpdates {
		return nil
	}

	nop := color.New(color.Reset)

	if v, err := g.View(`summary`); err == nil {
		v.Frame = true
		v.Title = `Message Summary`

		table := tabwriter.NewWriter(v, 5, 2, 1, ' ', tabwriter.Debug)
		// x, _ := v.Size()

		// print header
		fmt.Fprintf(table, strings.Join(self.summaryHeader(), "\t")+"\n")

		for _, frameSummary := range self.analyzer.GetFrameSummary(self.summarySortBy, self.sortReverse) {
			fmt.Fprintf(table, strings.Join([]string{
				nop.Sprintf("%04X", frameSummary.LatestFrame.ID),
				nop.Sprintf("%d", frameSummary.Count),
				nop.Sprintf("%v", time.Since(frameSummary.LastSeen).Round(time.Second)),
				nop.Sprintf("%d", frameSummary.LatestFrame.Length),
				PrettifyFrameData(frameSummary, true, DisplayRaw),
				PrettifyFrameData(frameSummary, true, DisplayU8),
				PrettifyFrameData(frameSummary, true, DisplayU16LE),
				PrettifyFrameData(frameSummary, true, DisplayU16BE),
				PrettifyFrameData(frameSummary, true, DisplayU32LE),
				PrettifyFrameData(frameSummary, true, DisplayU32BE),
				PrettifyFrameData(frameSummary, true, DisplayS32),
				PrettifyFrameData(frameSummary, true, DisplayASCII),
			}, "\t")+"\n")
		}

		v.Clear()
		return table.Flush()
	} else {
		return err
	}
}
