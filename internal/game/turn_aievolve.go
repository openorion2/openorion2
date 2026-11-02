package game

// applyEvolutionaryUpgrades is Apply_Evolutionary_Upgrades_ (0xdcbb0): a
// chosen evolution replaces the race picks once; pick 0 (government) stays.
func (g *Game) applyEvolutionaryUpgrades() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		pl := &s.Players[p]
		if pl.Evolved == 0 || pl.EvolutionApplied != 0 {
			continue
		}
		gov := pl.Picks[0]
		pl.Picks = pl.BasePicks
		pl.EvolutionApplied = 1
		pl.Picks[0] = gov
	}
}
