package app

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/walkerkiller/coursive/app-mygo/internal/study"
)

// studyParts is the state of the canvas's parts.
type studyParts struct {
	stageParts
	evidenceParts
	shelfParts
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

	// A selection's screenshot: rendering is set while the canvas draws
	// into one, snapshotting while its videos show their prepared frames.
	rendering, snapshotting bool
	snapshotPreparing       bool
	pendingDrop             *studySelectionDrop
	selectionBusy           bool
	linkStart               []study.Anchor
	linkStartBounds         *study.Rect
}

func (s *StudyCanvas) initParts() {
	s.docks = map[string]*dockState{}
	s.penItems = map[string]ui.Rect{}
	s.pdfStates = map[string]*pdfState{}
	s.videos = map[string]*localVideo{}
	s.videoTimes = map[string]int{}
	s.answering = map[string]bool{}
	s.textSelections = map[string][2]int{}
	s.conversations = map[string]*studyConversation{}
	s.initialConversations = map[string]*conversationStart{}
	s.snapshotFrames = map[string]*ui.Bitmap{}
	s.sourceNavigations = map[string]bool{}
	s.idleView = newStageViewState()
}

func (s *StudyCanvas) disposeParts() {
	for _, cv := range s.conversations {
		cv.dispose()
	}
	s.disposeVideos()
}

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

func (s *StudyCanvas) openReview(cardID, turnID string) {}
func (s *StudyCanvas) reviewRect() *study.Rect          { return nil }
func (s *StudyCanvas) reviewNarrow() bool               { return false }
func (s *StudyCanvas) reviewPanel(c *ui.Context)        {}
func (s *StudyCanvas) activePlugins() map[string]bool   { return nil }
func (s *StudyCanvas) canvasToolContributions() []any   { return nil }
func (s *StudyCanvas) pluginsOffered() bool             { return false }
func (s *StudyCanvas) plugins()                         {}
func (s *StudyCanvas) pluginObject(c *ui.Context, o *study.PluginObject, stage study.Rect, active bool) {
}
