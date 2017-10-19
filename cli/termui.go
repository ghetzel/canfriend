package main

import (
	"fmt"
	"strings"
	"time"

	tabwriter "github.com/NonerKao/color-aware-tabwriter"

	"github.com/fatih/color"
	"github.com/ghetzel/canfriend"
	"github.com/jroimartin/gocui"
)

var DefaultSummaryRefreshInterval = 100 * time.Millisecond

type AnalyzerUI struct {
	SummaryRefreshInterval time.Duration
	analyzer               *canfriend.Analyzer
	gui                    *gocui.Gui
	summarySortBy          canfriend.SortKey
	sortReverse            bool
	pauseUpdates           bool
}

func NewAnalyzerUI(analyzer *canfriend.Analyzer) *AnalyzerUI {
	return &AnalyzerUI{
		SummaryRefreshInterval: DefaultSummaryRefreshInterval,
		analyzer:               analyzer,
		summarySortBy:          canfriend.SortByLastSeen,
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
		case canfriend.SortByID:
			self.summarySortBy = canfriend.SortByCount
		case canfriend.SortByCount:
			self.summarySortBy = canfriend.SortByLastSeen
		case canfriend.SortByLastSeen:
			self.summarySortBy = canfriend.SortByLength
		case canfriend.SortByLength:
			self.summarySortBy = canfriend.SortByID
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
		self.h(`summary`, string(canfriend.SortByID)),
		self.h(`summary`, string(canfriend.SortByCount)),
		self.h(`summary`, string(canfriend.SortByLastSeen)),
		self.h(`summary`, string(canfriend.SortByLength)),
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
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayRaw),
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayU8),
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayU16LE),
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayU16BE),
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayU32LE),
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayU32BE),
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayS32),
				canfriend.PrettifyFrameData(frameSummary, true, canfriend.DisplayASCII),
			}, "\t")+"\n")
		}

		v.Clear()
		return table.Flush()
	} else {
		return err
	}

	return nil
}
