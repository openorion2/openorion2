package engine

// FadePercent is Get_Fade_Percent_ (0xc5bb0).
func (e *Engine) FadePercent() int { return e.fadePercent }

// BlindsFade is Blinds_Fade (0x13ee7e): off reaches the screen in step
// passes, pass i copying rows y1+i, y1+i+step, ... through the scan list.
func (e *Engine) BlindsFade(x1, y1, x2, y2, step int) error {
	if int16(step) < 1 {
		return nil
	}
	v := e.V
	// Save_Refresh_, Set_Refresh_Differential_No_Full_.
	oldRefresh, oldDump := v.refresh, v.dump
	v.refresh, v.dump = RefreshDifferential, 2
	for i := 0; int16(i) < int16(step); i++ {
		v.clearScan()
		for y := y1 + i; int16(y) <= int16(y2); y += step {
			v.AddSquare(x1, y, x2, y)
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	v.refresh, v.dump = oldRefresh, oldDump
	return nil
}
