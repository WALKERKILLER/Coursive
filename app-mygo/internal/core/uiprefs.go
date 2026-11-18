package core

import (
	"encoding/json"
	"maps"

	"github.com/walkerkiller/coursive/app-mygo/internal/design"
)

// UiPrefs is prefs.dart's UiPrefs: the appearance and behaviour settings,
// stored as JSON under "ui" in the shared preferences.
type UiPrefs struct {
	design.Prefs
	// OnChange runs after every update (the assistant hears them).
	OnChange func()
}

// UI is the process's preferences.
var UI = &UiPrefs{Prefs: design.DefaultPrefs()}

// LoadUiPrefs reads the stored preferences; corrupt ones fall back to the
// defaults.
func LoadUiPrefs() {
	UI.Prefs = design.DefaultPrefs()
	if Prefs == nil {
		return
	}
	raw, ok := Prefs.String("ui")
	if !ok {
		return
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return
	}
	p := design.DefaultPrefs()
	if p.Apply(m) {
		UI.Prefs = p
	}
}

// JSON is the preferences as stored (the assistant reads its registry keys).
func (u *UiPrefs) JSON() map[string]any { return u.Prefs.JSON() }

// ApplyJSON applies stored-form values (the bot's setting changes and their
// undo); an invalid value leaves every preference as it was.
func (u *UiPrefs) ApplyJSON(patch map[string]any) {
	m := u.Prefs.JSON()
	maps.Copy(m, patch)
	p := u.Prefs
	if !p.Apply(m) {
		return
	}
	u.Update(func(x *design.Prefs) { *x = p })
}

// Update applies a change, persists it and tells the listener.
func (u *UiPrefs) Update(change func(p *design.Prefs)) {
	change(&u.Prefs)
	if Prefs != nil {
		if b, err := json.Marshal(u.Prefs.JSON()); err == nil {
			Prefs.SetString("ui", string(b))
		}
	}
	if u.OnChange != nil {
		u.OnChange()
	}
}
