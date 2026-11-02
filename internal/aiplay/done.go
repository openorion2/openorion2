package aiplay

// Done tells the network how the game ended for its empire (training's
// last reward, branch aitrain); it answers nothing.
func (p *Client) Done(r any) error {
	return p.in.Encode(map[string]any{"done": r})
}
