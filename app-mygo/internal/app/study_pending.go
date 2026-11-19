package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// studyParts is the state of the canvas's parts.
type studyParts struct {
	ctx            *ui.Context
	docks          map[string]*dockState
	penItems       map[string]ui.Rect
	controls       []study.Rect
	quickSnapshot  *studySnapshot
	pdfStates      map[string]*pdfState
	videos         map[string]*localVideo
	videoTimes     map[string]int
	answering      map[string]bool
	shelfTab       shelfTab
	textSelections map[string][2]int
}

func (s *StudyCanvas) initParts() {
	s.docks = map[string]*dockState{}
	s.penItems = map[string]ui.Rect{}
	s.pdfStates = map[string]*pdfState{}
	s.videos = map[string]*localVideo{}
	s.videoTimes = map[string]int{}
	s.answering = map[string]bool{}
	s.textSelections = map[string][2]int{}
}

func (s *StudyCanvas) disposeParts() { s.disposeVideos() }

func trimSpace(v string) string { return strings.TrimSpace(v) }

type shelfTab int

const (
	shelfNone shelfTab = iota
	shelfTranscript
	shelfFiles
	shelfPractice
	shelfNotes
	shelfThreads
)

type studySnapshot struct {
	bytes []byte
	bmp   *ui.Bitmap
}

func (s *studySnapshot) bitmap() *ui.Bitmap {
	if s.bmp == nil {
		s.bmp, _ = ui.DecodeBitmap(s.bytes)
	}
	return s.bmp
}

type conversationStart struct {
	send, task        string
	bind, allowImages bool
	pick              bool
	snapshot          *studySnapshot
	at                *study.Pt
}

func (s *StudyCanvas) adoptThreads(d *study.Document)              {}
func (s *StudyCanvas) openConversation(newConversation, bind bool) {}
func (s *StudyCanvas) startConversation(o conversationStart, done func(error)) {
	if done != nil {
		done(nil)
	}
}
func (s *StudyCanvas) chooseConversation(card *study.Card)                                      {}
func (s *StudyCanvas) courseMaterials()                                                         {}
func (s *StudyCanvas) videoModules()                                                            {}
func (s *StudyCanvas) connectionSettings()                                                      {}
func (s *StudyCanvas) editKnowledge(card *study.Card)                                           {}
func (s *StudyCanvas) editSelectionFilters()                                                    {}
func (s *StudyCanvas) selectionActions(c *ui.Context)                                           {}
func (s *StudyCanvas) selectionJob(fn func())                                                   { fn() }
func (s *StudyCanvas) linkSelection()                                                           {}
func (s *StudyCanvas) captureRegionCard()                                                       {}
func (s *StudyCanvas) snapshotDragHandle(c *ui.Context)                                         {}
func (s *StudyCanvas) snapshotVideo(c *ui.Context, card *study.Card, child func())              { child() }
func (s *StudyCanvas) prepareSelectionSnapshot(done func(*studySnapshot, error))                {}
func (s *StudyCanvas) goBack() bool                                                             { return false }
func (s *StudyCanvas) focusSource(a study.Anchor)                                               {}
func (s *StudyCanvas) paintEvidence(p *ui.Painter, r ui.Rect)                                   {}
func paintConnections(p *ui.Painter, r ui.Rect, d *study.Document, stage study.Rect, ch chrome) {}
func (s *StudyCanvas) dropOutline(c *ui.Context)                                                {}
func (s *StudyCanvas) agentStage(c *ui.Context)                                                 {}
func (s *StudyCanvas) reviewPanel(c *ui.Context)                                                {}
func (s *StudyCanvas) returnPill(c *ui.Context)                                                 {}
func (s *StudyCanvas) shelf(c *ui.Context)                                                      {}
func (s *StudyCanvas) toggleShelf(t shelfTab)                                                   {}
func (s *StudyCanvas) activePlugins() map[string]bool                                           { return nil }
func (s *StudyCanvas) canvasToolContributions() []any                                           { return nil }
func (s *StudyCanvas) pluginsOffered() bool                                                     { return false }
func (s *StudyCanvas) plugins()                                                                 {}
func (s *StudyCanvas) pluginObject(c *ui.Context, o *study.PluginObject, stage study.Rect, active bool) {
}
func readingGlow(c *ui.Context, active bool, radius float32, child func())          { child() }
func (s *StudyCanvas) cardDragSource(c *ui.Context, e ui.Element, card *study.Card) {}
