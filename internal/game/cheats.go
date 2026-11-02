package game

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"regexp"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// Function keys and the Alt-typed cheats of the galaxy screen:
// Check_Function_Keys_ (0x825a8), Check_Hot_Keys_ (0x82809) and what they
// call. Spec: docs/screens/main-screen-panel.md ("Check_Function_Keys_",
// "Check_Hot_Keys_").

// cheatWords are the Alt-typed cheats Add_Cheats_ adds, in order.
var cheatWords = [5]string{"EINSTEIN", "MOOLA", "MENLO", "ISEEALL", "SCORE"}

// Indices into mainScreenFields.cheats.
const (
	cheatAllTech = iota
	cheatMoreMoney
	cheatGetTech
	cheatOmniscience
	cheatHiScore
)

// addCheats is Add_Cheats_ (0x81d15).
func (g *Game) addCheats() error {
	f := &g.ms.fields
	for i, w := range cheatWords {
		n, err := g.e.AddMultiHotKeyField(w)
		if err != nil {
			return err
		}
		f.cheats[i] = n
	}
	f.lastField = g.e.FieldsCount() - 1
	return nil
}

// Get_Input codes of the function keys.
const (
	keyCodeF1     = -1001
	keyCodeF2     = -1002
	keyCodeF4     = -1004
	keyCodeF5     = -1005
	keyCodeF6     = -1006
	keyCodeF9     = -1009
	keyCodeF10    = -1010
	keyCodeAltF1  = -1101
	keyCodeAltF8  = -1108
	keyCodeAltF10 = -1110
)

// mouseListTo is MAINSCR.C's _mouse_list_to (0x1821b1): the relocation
// cursor over the map.
var mouseListTo = []engine.MouseRegion{
	{Image: 2, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x1df},
	{Image: 0x12, X1: 0x16, Y1: 0x16, X2: 0x20e, Y2: 0x1a4},
}

// checkFunctionKeys is Check_Function_Keys_ (0x825a8).
func (g *Game) checkFunctionKeys(r int, leave *bool) error {
	s := &g.state
	if r == keyCodeAltF10 {
		return nil
	}
	if r >= keyCodeAltF8 && r <= keyCodeAltF1 {
		if err := g.toggleGameOption(keyCodeAltF1 - r); err != nil {
			return err
		}
	}
	switch r {
	case keyCodeF10:
		msg := 0xd4
		if s.DisallowSavesCode == 0 {
			if err := g.saveGame(9); err != nil {
				return err
			}
			msg = 0xd3
		}
		return g.mainUserBox(g.hMessage(msg))
	case keyCodeF9:
		g.currentScreen, g.returnScreen = ScreenStarDistances, 0
		*leave = true
	case keyCodeF6, keyCodeF5:
		back := r == keyCodeF5
		sd := &g.sd
		sd.autoSelectStar = int16(g.cycleStarIcons(int(sd.autoSelectStar), int(s.PlayerNum), back))
		star := int(sd.autoSelectStar)
		if star < 0 || star >= int(s.NumStars) {
			return nil
		}
		col := -1
		if g.settings[0x07] != 0 {
			col = g.lastColonySelected(star, int(s.PlayerNum))
		}
		if col >= 0 {
			return g.doColonyScreen(col, leave) // 0x82720
		}
		s.MoveableBoxes[0].Content = int16(star)
		return g.doSystemPopup(star)
	case keyCodeF4:
		n := g.nStarRelocations(int(s.PlayerNum))
		switch {
		case n > 1:
			if !g.mergingRelocations {
				g.e.SetMouseList(mouseListTo)
				g.mergingRelocations = true
			} else {
				g.e.SetMouseList(engine.MouseArrow)
				g.mergingRelocations = false
			}
		case n == 1:
			return g.mainUserBox(g.hMessage(0xd1))
		default:
			return g.mainUserBox(g.hMessage(0xd2))
		}
	case keyCodeF2, keyCodeF1:
		icon := g.cycleShipIcons(&g.fb.stack, r == keyCodeF1)
		if icon > -1 && icon < int(g.stacks.iconCount) {
			return g.doFleetPopup(icon, false)
		}
	}
	return nil
}

// nStarRelocations is N_Star_Relocations_ (0x782ed): stars with a
// relocation target for player p (a char count).
func (g *Game) nStarRelocations(p int) int8 {
	s := &g.state
	var n int8
	for i := 0; i < int(s.NumStars); i++ {
		if s.Stars[i].Relocation[p] != -1 {
			n++
		}
	}
	return n
}

// playerHasColony is Player_Has_Colony_ (0x79d1c): bit p of the star's
// colony owner mask.
func (g *Game) playerHasColony(p, star int) bool {
	return g.state.Stars[star].ColonyOwners>>uint(p)&1 != 0
}

// cycleStarIcons is Cycle_Star_Icons_ (0x8302c): the next (or previous)
// star where player p has a colony, wrapping once; -1 if none.
func (g *Game) cycleStarIcons(cur, p int, back bool) int {
	n := int(g.state.NumStars)
	if !back {
		for i := cur + 1; i < n; i++ {
			if g.playerHasColony(p, i) {
				return i
			}
		}
		for i := 0; i < n; i++ {
			if g.playerHasColony(p, i) {
				return i
			}
		}
		return -1
	}
	for i := int(int16(cur - 1)); i >= 0; i-- {
		if g.playerHasColony(p, i) {
			return i
		}
	}
	for i := n - 1; i > cur-1; i-- {
		if g.playerHasColony(p, i) {
			return i
		}
	}
	return -1
}

// shipOwner is Ship_Owner_ (0x78927); the original exits with "Memory
// Corruption" for a bad index or owner.
func (g *Game) shipOwner(ship int) int {
	if ship < 0 || ship >= maxShips || g.state.Ships[ship].Owner < 0 || g.state.Ships[ship].Owner > 15 {
		panic(fmt.Sprintf("Memory Corruption: ship %d", ship))
	}
	return int(g.state.Ships[ship].Owner)
}

// cycleShipIcons is Cycle_Ship_Icons_ (0x82dff): from stack *stack, the
// next (or previous) stack whose first ship belongs to the fleet box's
// owner and is not departing, wrapping once. It returns the icon showing
// that stack and stores the stack; both are -1 when none is found, or when
// *stack is -1 to begin with.
func (g *Game) cycleShipIcons(stack *int16, back bool) int {
	s, st := &g.state, &g.stacks
	owner := int(s.PlayerNum)
	if fb := g.fb.stack; fb > 0 && fb < maxNodes && g.moveableBoxSelected(2) {
		owner = g.shipStackOwner(st.start[fb])
	}
	found := false
	at, node := -1, int16(-1)
	try := func(i int) {
		node = st.start[i]
		ship := int(st.node[node].ship)
		if g.shipOwner(ship) == owner && s.Ships[ship].Status < 3 {
			found, at = true, i
		}
	}
	icon := -1
	if st.iconCount > 0 && *stack != -1 {
		cur := int(*stack)
		if !back {
			for i := int(int16(cur + 1)); i < int(st.count) && !found; i++ {
				try(i)
			}
			for i := 0; i < int(st.count) && !found; i++ {
				try(i)
			}
		} else {
			for i := int(int16(cur - 1)); i >= 0 && !found; i-- {
				try(i)
			}
			for i := int(st.count) - 1; i > cur-1 && !found; i-- {
				try(i)
			}
		}
		if node > -1 {
			for i := 0; i < int(st.iconCount); i++ {
				if st.icon[i].node == node {
					icon = i
					break
				}
			}
		}
	}
	if icon == -1 {
		*stack = -1
		return -1
	}
	*stack = int16(at)
	return icon
}

// toggleGameOption is Toggle_Game_Option_ (0x8a216): flip a setting,
// write the session settings to MOX.SET and say so. Option 3 has no case:
// the original prints a stale buffer with the caller's DL (non-zero here).
func (g *Game) toggleGameOption(opt int) error {
	set := &g.settings
	flip := func(i int) bool {
		if set[i] == 0 {
			set[i] = 1
		} else {
			set[i] = 0
		}
		return set[i] != 0
	}
	name, on := "", true
	switch opt {
	case 0:
		on, name = flip(0x00), g.hMessage(0xa6)
	case 1:
		on, name = flip(0x01), g.hMessage(0xa7)
	case 2:
		on, name = flip(0x03), g.hMessage(0xa8)
	case 4:
		on, name = flip(0x05), g.hMessage(0xaa)
	case 5:
		on, name = flip(0x06), g.hMessage(0xab)
	case 6:
		on, name = flip(0x07), g.hMessage(0xac)
		if on {
			g.setAllColonyAutoSelectFlags()
		}
	case 7:
		on, name = flip(0x08), g.hMessage(0xad)
	}
	state := 0xe8
	if on {
		state = 0xe7
	}
	msg := name + " " + g.hMessage(state)
	if err := g.saveSessionRelatedSettings(); err != nil {
		return err
	}
	return g.mainUserBox(msg)
}

// setAllColonyAutoSelectFlags is Set_All_Colony_Auto_Select_Flags_
// (0x78507). The first test reads the stored orbit as a planet index and
// compares that planet's colony with the player number, as the original.
func (g *Game) setAllColonyAutoSelectFlags() {
	s := &g.state
	pn := int(s.PlayerNum)
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		found := false
		if o := int(st.LastOrbit[pn]); o != -1 && o >= 0 && o < maxPlanets && s.Planets[o].Colony == s.PlayerNum {
			found = true
		}
		for orbit := 0; orbit < 5 && !found; orbit++ {
			pl := int(st.Planets[orbit])
			if pl < 0 {
				continue
			}
			c := int(s.Planets[pl].Colony)
			if c < 0 || s.Colonies[c].IsOutpost != 0 {
				continue
			}
			if o := int(s.Colonies[c].Owner); o == pn {
				st.LastOrbit[o] = int8(orbit)
			} else {
				st.LastOrbit[pn] = -1
			}
		}
	}
}

// sessionSettingsSize is how much of _settings Save_Session_Related_Settings_
// (0x12030) copies over MOX.SET: up to and including the language byte.
const sessionSettingsSize = 0xd3

// saveSessionRelatedSettings is Save_Session_Related_Settings_: read
// MOX.SET, put the session settings over its start and write it back.
func (g *Game) saveSessionRelatedSettings() error {
	path := g.findFile("MOX.SET")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Settings file (MOX.SET) is missing: %w", err)
	}
	buf := make([]byte, len(g.settings))
	copy(buf, data)
	copy(buf, g.settings[:sessionSettingsSize])
	return os.WriteFile(path, buf, 0o644)
}

// saveGameSettings is Save_Game_Settings_ (0x11be4): MOX.SET from _settings.
func (g *Game) saveGameSettings() error {
	return os.WriteFile(g.findFile("MOX.SET"), g.settings[:], 0o644)
}

// saveGame is Save_Game_ (0x1160b): SAVE<slot+1>.GAM, then MOX.SET. A
// failed open shows a message box.
func (g *Game) saveGame(slot int) error {
	name := fmt.Sprintf("SAVE%d.GAM", slot+1)
	if err := os.WriteFile(g.findFile(name), g.encodeSave(slot), 0o644); err != nil {
		return g.messageBox("Error saving game: " + err.Error())
	}
	g.saveLobbyLink(slot) // port-only
	return g.saveGameSettings()
}

// encodeSave is the file Save_Game_ writes, in the block order of
// docs/formats/save.md; the description is slot min(slot, 9)'s.
func (g *Game) encodeSave(slot int) []byte {
	s := &g.state
	var b bytes.Buffer
	w := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	d := min(slot, 9)
	w(uint32(save.Version))
	w(s.Descriptions[d])
	w(s.Stardate)
	w(uint8(g.gameType)) // the save's byte is _game_type itself
	w(g.settings)
	w(s.Random.Get())
	for _, v := range []any{&s.NumColonies, &s.Colonies, &s.PlanetCount, &s.Planets,
		&s.NumStars, &s.Stars, &s.Leaders, &s.NumPlayers, &s.Players, &s.NumShips, &s.Ships,
		&s.MoveableBoxes, &s.MoveableBoxOrder, &s.PlayerNum, &s.MaxMapScale, &s.MaxZoomCount,
		&s.StarBgIndex, &s.MapMaxX, &s.MapMaxY, &s.FleetIconButtons, &s.Nebulas, &s.NumNebulas,
		&s.BillSavegame, &s.NReports, &s.Reports, &s.DonePlayerReports, &s.SomethingHappened,
		&s.DoneColonizeCheck, &s.EventData, &s.Msgs, &s.FirstMsg, &s.MaxMsgs, &s.NMsgs,
		&s.DisplayedMsgs, &s.DisplayedGNN, &s.CouncilFlag, &s.NextCouncilTurn, &s.AntaranResources,
		&s.AntaranInvasionChance, &s.DisallowSavesCode, &s.LastEventYear, &s.EventDelayCount,
		&s.Antarans, &s.CurMapScale, &s.CurMapX, &s.CurMapY} {
		w(v)
	}
	w(make([]byte, 1000))
	return b.Bytes()
}

// checkHotKeys is Check_Hot_Keys_ (0x82809): the cheats. The never-added
// _omniscient_hot_key and _all_ai_cheat stay NoField and cannot match.
func (g *Game) checkHotKeys(r int) error {
	s, f := &g.state, &g.ms.fields
	p := &s.Players[s.PlayerNum]
	if r == f.cheats[cheatOmniscience] {
		on := p.Picks[save.PickOmniscient] == 0
		p.Picks[save.PickOmniscient] = 0
		if on {
			p.Picks[save.PickOmniscient] = 1
			if err := g.mainUserBox(g.hMessage(0xd5)); err != nil {
				return err
			}
			p.Cheated, p.CheatFlag = 1, 1
		} else if err := g.mainUserBox(g.hMessage(0xd6)); err != nil {
			return err
		}
		g.ms.drawToBack = true
		for i := 0; i < int(s.NumStars); i++ {
			s.Stars[i].Owner = int8(s.starOwner(i, int(s.PlayerNum)))
		}
	}
	var flag uint8
	switch r {
	case f.cheats[cheatAllTech]:
		if err := g.giveMeAllTechAdvances(); err != nil {
			return err
		}
		flag = 2
	case f.cheats[cheatMoreMoney]:
		p.BC += 1000
		msg := fmt.Sprintf(cFormat(g.hMessage(0xd7)), save.CString(p.RaceName[:]), 1000)
		if err := g.mainUserBox(msg); err != nil {
			return err
		}
		flag = 3
	case f.cheats[cheatGetTech]:
		name, err := g.techAppName(int(p.ResearchApp))
		if err != nil {
			return err
		}
		msg := fmt.Sprintf(cFormat(g.hMessage(0xd8)), name)
		p.ResearchPoints = 9999
		if err := g.mainUserBox(msg); err != nil {
			return err
		}
		flag = 4
	case f.cheats[cheatHiScore]:
		// SCORE marks nothing as cheated.
		return g.scoreCheat()
	default:
		return nil
	}
	p.Cheated, p.CheatFlag = 1, flag
	return nil
}

// cFormat turns the printf conversions the H strings use into Go's.
// cIntVerb is %i with its flags, width and short size ("%3i", "%hi").
var cIntVerb = regexp.MustCompile(`%([-+ 0#]*[0-9]*)h?i`)

func cFormat(s string) string {
	b := bytes.ReplaceAll([]byte(s), []byte("%ld"), []byte("%d"))
	b = cIntVerb.ReplaceAll(b, []byte("%${1}d"))
	// Watcom stops at a '%' that ends the format (skill text "+%d%").
	if n := len(b) - len(bytes.TrimRight(b, "%")); n%2 == 1 {
		b = b[:len(b)-1]
	}
	return string(b)
}

// techAppName is _technology_applications[a].name as Load_Tech_Names_
// (0x5df0a) sets it: the strings of TECHNAME.LBX entry <language>, runs of
// NULs between them, 83 field names first.
func (g *Game) techAppName(a int) (string, error) {
	arc, err := g.archive("TECHNAME.LBX")
	if err != nil {
		return "", err
	}
	data, err := arc.Entry(int(g.settings[0xd2]))
	if err != nil {
		return "", err
	}
	want := 83 + a
	for i, n := 0, 0; i < len(data); n++ {
		j := bytes.IndexByte(data[i:], 0)
		if j < 0 {
			j = len(data) - i
		}
		if n == want {
			return string(data[i : i+j]), nil
		}
		i += j
		for i < len(data) && data[i] == 0 {
			i++
		}
	}
	return "", fmt.Errorf("TECHNAME.LBX has no application %d", a)
}

// warnAboutCheatingPlayers is Warn_About_Cheating_Players_ (0x8a571): for
// each other player whose cheat flag is up (it travels with his player
// record) and of whom PLAYER_NUM was not warned yet, a warning box H 0x16
// with his name and race in capitals; he is marked as having cheated and
// PLAYER_NUM as warned of him. PLAYER_NUM's own cheat flag goes down.
func (g *Game) warnAboutCheatingPlayers() error {
	s, e := &g.state, g.e
	me := &s.Players[s.PlayerNum]
	for q := 0; q < int(s.NumPlayers); q++ {
		if q == int(s.PlayerNum) {
			continue
		}
		p := &s.Players[q]
		if p.CheatFlag == 0 || me.CheatWarning>>uint(q)&1 != 0 {
			continue
		}
		name, race := cheaterCaps(save.CString(p.EmperorName[:])), cheaterCaps(save.CString(p.RaceName[:]))
		p.Cheated = 1
		msg := cSprintf(g.lbxMessage(0x16), name, race)
		if _, err := g.userBox(&g.ms.drawToBack, msg, boxWarning); err != nil {
			return err
		}
		e.V.SetPageOff()
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		e.MoxUpdate()
		if err := e.TogglePages(); err != nil {
			return err
		}
		me.CheatWarning |= 1 << uint(q)
	}
	me.CheatFlag = 0
	return nil
}

// cheaterCaps is the warning's own capitals (0x132f5c): bytes 'a'..0x7f
// less 0x20.
func cheaterCaps(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 0x7f {
			b[i] = c - 0x20
		}
	}
	return string(b)
}
