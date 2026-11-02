package game

// The autopilot: Options.Autopilot (-autopilot) plays the human of a
// single-player game by the AI.

// makePlayerIntoAI is Make_Player_Into_AI_ (0xfec20), the conversion of the
// debug cheat Make_All_AI_Game_: a random objective, the AI's designs, empty
// build queues and the colonies recomputed.
func (g *Game) makePlayerIntoAI(p int) {
	s := &g.state
	s.Players[p].Objective = uint8(s.Random.Random(5))
	g.updatePlayerShipDesigns(p)
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if int(s.Colonies[i].Owner) == p {
			g.clearColonyQueue(i)
		}
	}
	g.doPlayerColonyCalculations(p)
}
