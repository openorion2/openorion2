package game

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"

	"github.com/openorion2/openorion2/internal/save"
)

// settingsVersion is the MOX.SET version this build writes.
const settingsVersion = 0x82

// loadSettings is the settings part of main2_: Load_Game_Settings_ when
// MOX.SET exists (with Update_Game_Settings_Version_Number_ for an older
// version), Set_Default_Game_Settings_ otherwise.
func (g *Game) loadSettings() {
	defer g.setMoxLanguage()
	if g.dirMatch("MOX.SET") && g.loadGameSettings() {
		if g.settings.Version() != settingsVersion {
			g.updateGameSettingsVersionNumber()
		}
		return
	}
	g.setDefaultGameSettings()
}

// setMoxLanguage is Set_Mox_Language_: whatever language.ini says, the
// language byte (settings +0xd2) is set to 0, English.
func (g *Game) setMoxLanguage() { g.settings[0xd2] = 0 }

// loadGameSettings is Load_Game_Settings_. A short file leaves the rest of
// the settings as they were, as fread does.
func (g *Game) loadGameSettings() bool {
	data, err := os.ReadFile(g.findFile("MOX.SET"))
	if err != nil {
		return false
	}
	copy(g.settings[:], data)
	if g.e != nil {
		g.setSoundVolumeToSettings() // 0x11c79
	}
	return true
}

// setDefaultGameSettings is Set_Default_Game_Settings_.
func (g *Game) setDefaultGameSettings() {
	s := &g.settings
	*s = save.Settings{}
	for i, v := range []byte{1, 1, 1, 0, 0, 1, 1, 0, 1, 1, 0, 1, 0, 0, 0, 0} {
		s[i] = v
	}
	s[0x11], s[0x12], s[0x13], s[0x14], s[0x15] = 1, 0x32, 1, 0x32, 0xff
	s[0xd3], s[0xd4], s[0xd5] = 1, 0, 5
	s[0xd6], s[0xd7], s[0xd8], s[0xd9] = 1, 1, 1, 1
	s[0xb4], s[0xb5], s[0xb6], s[0xb7], s[0xb8] = 0, 0, 1, 0, 0
	binary.LittleEndian.PutUint16(s[0xde:], 2)
	binary.LittleEndian.PutUint16(s[0xe0:], 1)
	binary.LittleEndian.PutUint16(s[0xe2:], settingsVersion)
	for i := range g.state.Descriptions {
		g.state.Descriptions[i][0] = 0
	}
	// sprintf("MOX-%d", clock() % 10) at 0x17894d.
	n := int(int32(g.clockTicks()) % 10)
	copy(s[0x17:], cSprintf(cStringAt(g.exe, 0x17894d), n)+"\x00")
	g.setDefaultModemSettings()
	g.writeGameSettings()
}

// updateGameSettingsVersionNumber is Update_Game_Settings_Version_Number_
// (0x12227): an older MOX.SET is loaded again, stamped 0x82 and saved; if it
// cannot be opened the defaults are written instead.
func (g *Game) updateGameSettingsVersionNumber() {
	if _, err := os.Stat(g.findFile("MOX.SET")); err != nil {
		g.setDefaultGameSettings()
		return
	}
	g.loadGameSettings()
	binary.LittleEndian.PutUint16(g.settings[0xe2:], settingsVersion)
	if err := g.saveGameSettings(); err != nil {
		g.Log("Save_Game_Settings_: %v", err)
	}
}

// setDefaultModemSettings is Set_Default_Modem_Settings_ (0xf46e7): the
// serial block +0x20.. (port 0, 19200 baud, 'E', 7, 0, the init strings at
// 0x17a0bb.. ) and +0xb3 = 1. Its busy flag at +0x20 is set and cleared again.
func (g *Game) setDefaultModemSettings() {
	s := &g.settings
	le := binary.LittleEndian
	le.PutUint32(s[0x20:], 1)
	le.PutUint32(s[0x24:], 0x4b00)
	s[0x28] = 0x45
	le.PutUint32(s[0x29:], 7)
	le.PutUint32(s[0x2d:], 0)
	copy(s[0x31:0x35], g.exe.bytes(0x17a0bb, 4))
	copy(s[0x4f:0x4f+14], g.exe.bytes(0x17a0bf, 14))
	copy(s[0x6d:0x6d+11], g.exe.bytes(0x17a0cd, 11))
	copy(s[0x8b:0x8b+8], g.exe.bytes(0x17a0d8, 8))
	s[0xb3] = 1
	le.PutUint32(s[0x20:], 0)
}

// writeGameSettings is Write_Game_Settings_ (0x12937): all 0x229 bytes to
// mox.set; a failed open ends the program (Exit_With_Message_).
func (g *Game) writeGameSettings() {
	if err := os.WriteFile(g.findFile("MOX.SET"), g.settings[:], 0o644); err != nil {
		g.Log("Unable to open mox.set: %v", err)
	}
}

// findFile returns the path of a file in the game directory, matching the
// name case-insensitively as DOS does.
func (g *Game) findFile(name string) string {
	ents, err := os.ReadDir(g.dir)
	if err == nil {
		for _, e := range ents {
			if strings.EqualFold(e.Name(), name) {
				return filepath.Join(g.dir, e.Name())
			}
		}
	}
	return filepath.Join(g.dir, name)
}
