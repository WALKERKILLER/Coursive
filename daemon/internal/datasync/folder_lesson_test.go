package datasync

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLessonJournalAndMemoryTravelWithOnlyTheirCourse(t *testing.T) {
	a, root := folderFixture(t)
	b, _ := folderFixture(t)
	remote := t.TempDir()
	configureTest(t, a, remote)
	configureTest(t, b, remote)
	writeFolderTest(t, root, "courses/pilot/course.json", `{"id":"local-pilot"}`)
	event := `{"format":1,"parent":null,"fields":{"course":"local-pilot","revision":1},"sources":{},"attempts":[]}`
	name := fmt.Sprintf("%x.json", sha256.Sum256([]byte(event)))
	journal := filepath.Join("workspaces", "course-tutor", "course-"+courseHash("local-pilot"), name)
	writeFolderTest(t, root, journal, event)
	memory := "agent/bot/memory/courses/local-pilot.md"
	writeFolderTest(t, a.Roots.Data, memory, "prefer examples in this course")
	writeFolderTest(t, a.Roots.Data, memory+".edited", "1760000000000")
	writeFolderTest(t, a.Roots.Data, "agent/bot/memory/preferences.md", "private")
	writeFolderTest(t, a.Roots.Data, "agent/bot/memory/courses/other.md", "unshared")
	runFolderTest(t, a)
	runFolderTest(t, b)
	assertFolderBytes(t, b.Roots.Library, journal, event)
	assertFolderBytes(t, b.Roots.Data, memory, "prefer examples in this course")
	assertFolderBytes(t, b.Roots.Data, memory+".edited", "1760000000000")
	for _, k := range []string{"preferences.md", "courses/other.md"} {
		if _, err := os.Stat(filepath.Join(b.Roots.Data, "agent/bot/memory", k)); !os.IsNotExist(err) {
			t.Fatalf("unscoped memory %s: %v", k, err)
		}
	}
}
